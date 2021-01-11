package p2p_test

import (
	"errors"
	"testing"

	gogotypes "github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
)

type TestMessage = gogotypes.StringValue

func echoReactor(channel *p2p.Channel) {
	for {
		select {
		case envelope := <-channel.In():
			channel.Out() <- p2p.Envelope{
				To:      envelope.From,
				Message: &TestMessage{Value: envelope.Message.(*TestMessage).Value},
			}
		case <-channel.Done():
			return
		}
	}
}

func TestRouter(t *testing.T) {
	logger := log.TestingLogger()
	network := p2p.NewMemoryNetwork(logger)
	transport := network.GenerateTransport()
	chID := p2p.ChannelID(1)

	// Start some other in-memory network nodes to communicate with, running
	// a simple echo reactor that returns received messages.
	peers := []p2p.PeerAddress{}
	for i := 0; i < 3; i++ {
		peerTransport := network.GenerateTransport()
		peerRouter := p2p.NewRouter(logger.With("peerID", i), map[p2p.Protocol]p2p.Transport{
			p2p.MemoryProtocol: peerTransport,
		}, nil)
		peers = append(peers, peerTransport.Endpoints()[0].PeerAddress())

		channel, err := peerRouter.OpenChannel(chID, &TestMessage{})
		require.NoError(t, err)
		defer channel.Close()
		go echoReactor(channel)

		err = peerRouter.Start()
		require.NoError(t, err)
		defer func() { require.NoError(t, peerRouter.Stop()) }()
	}

	// Start the main router and connect it to the peers above.
	router := p2p.NewRouter(logger, map[p2p.Protocol]p2p.Transport{
		p2p.MemoryProtocol: transport,
	}, peers)
	channel, err := router.OpenChannel(chID, &TestMessage{})
	require.NoError(t, err)
	peerUpdates, err := router.SubscribePeerUpdates()
	require.NoError(t, err)

	err = router.Start()
	require.NoError(t, err)
	defer func() {
		channel.Close()
		peerUpdates.Close()
		require.NoError(t, router.Stop())
	}()

	// Wait for peers to come online, and ping them as they do.
	for i := 0; i < len(peers); i++ {
		peerUpdate := <-peerUpdates.Updates()
		peerID := peerUpdate.PeerID
		require.Equal(t, p2p.PeerUpdate{
			PeerID: peerID,
			Status: p2p.PeerStatusUp,
		}, peerUpdate)

		channel.Out() <- p2p.Envelope{To: peerID, Message: &TestMessage{Value: "hi!"}}
		assert.Equal(t, p2p.Envelope{
			From:    peerID,
			Message: &TestMessage{Value: "hi!"},
		}, (<-channel.In()).Strip())
	}

	// We then submit an error for a peer, and watch it get disconnected.
	channel.Error() <- p2p.PeerError{
		PeerID:   peers[0].NodeID(),
		Err:      errors.New("test error"),
		Severity: p2p.PeerErrorSeverityCritical,
	}
	peerUpdate := <-peerUpdates.Updates()
	require.Equal(t, p2p.PeerUpdate{
		PeerID: peers[0].NodeID(),
		Status: p2p.PeerStatusDown,
	}, peerUpdate)

	// We now broadcast a message, which we should receive back from only two peers.
	channel.Out() <- p2p.Envelope{
		Broadcast: true,
		Message:   &TestMessage{Value: "broadcast"},
	}
	for i := 0; i < len(peers)-1; i++ {
		envelope := <-channel.In()
		require.NotEqual(t, peers[0].NodeID(), envelope.From)
		require.Equal(t, &TestMessage{Value: "broadcast"}, envelope.Message)
	}
	select {
	case envelope := <-channel.In():
		t.Errorf("unexpected message: %v", envelope)
	default:
	}
}
