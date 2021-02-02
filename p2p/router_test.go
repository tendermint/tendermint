package p2p_test

import (
	"errors"
	"testing"

	"github.com/fortytw2/leaktest"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
)

type TestMessage = gogotypes.StringValue

func generateNode() (p2p.NodeInfo, crypto.PrivKey) {
	privKey := ed25519.GenPrivKey()
	nodeID := p2p.NodeIDFromPubKey(privKey.PubKey())
	nodeInfo := p2p.NodeInfo{
		NodeID: nodeID,
		// FIXME: We have to fake a ListenAddr for now.
		ListenAddr: "127.0.0.1:1234",
		Moniker:    "foo",
	}
	return nodeInfo, privKey
}

func echoReactor(channel *p2p.Channel) {
	for {
		select {
		case envelope := <-channel.In:
			channel.Out <- p2p.Envelope{
				To:      envelope.From,
				Message: &TestMessage{Value: envelope.Message.(*TestMessage).Value},
			}
		case <-channel.Done():
			return
		}
	}
}

func TestRouter(t *testing.T) {
	defer leaktest.Check(t)()

	logger := log.TestingLogger()
	network := p2p.NewMemoryNetwork(logger)
	nodeInfo, privKey := generateNode()
	transport := network.CreateTransport(nodeInfo.NodeID)
	defer transport.Close()
	chID := p2p.ChannelID(1)

	// Start some other in-memory network nodes to communicate with, running
	// a simple echo reactor that returns received messages.
	peers := []p2p.NodeAddress{}
	for i := 0; i < 3; i++ {
		peerInfo, peerKey := generateNode()
		peerManager, err := p2p.NewPeerManager(peerInfo.NodeID, dbm.NewMemDB(), p2p.PeerManagerOptions{})
		require.NoError(t, err)
		peerTransport := network.CreateTransport(peerInfo.NodeID)
		defer peerTransport.Close()
		peerRouter, err := p2p.NewRouter(
			logger.With("peerID", i),
			peerInfo,
			peerKey,
			peerManager,
			[]p2p.Transport{peerTransport},
			p2p.RouterOptions{},
		)
		require.NoError(t, err)
		peers = append(peers, peerTransport.Endpoints()[0].NodeAddress(peerInfo.NodeID))

		channel, err := peerRouter.OpenChannel(chID, &TestMessage{})
		require.NoError(t, err)
		defer channel.Close()
		go echoReactor(channel)

		err = peerRouter.Start()
		require.NoError(t, err)
		defer func() { require.NoError(t, peerRouter.Stop()) }()
	}

	// Start the main router and connect it to the peers above.
	peerManager, err := p2p.NewPeerManager(nodeInfo.NodeID, dbm.NewMemDB(), p2p.PeerManagerOptions{})
	require.NoError(t, err)
	defer peerManager.Close()
	for _, address := range peers {
		err := peerManager.Add(address)
		require.NoError(t, err)
	}
	peerUpdates := peerManager.Subscribe()
	defer peerUpdates.Close()

	router, err := p2p.NewRouter(logger, nodeInfo, privKey, peerManager, []p2p.Transport{transport}, p2p.RouterOptions{})
	require.NoError(t, err)
	channel, err := router.OpenChannel(chID, &TestMessage{})
	require.NoError(t, err)
	defer channel.Close()

	err = router.Start()
	require.NoError(t, err)
	defer func() {
		// Since earlier defers are closed after this, and we have to make sure
		// we close channels and subscriptions before the router, we explicitly
		// close them here to.
		peerUpdates.Close()
		channel.Close()
		require.NoError(t, router.Stop())
	}()

	// Wait for peers to come online, and ping them as they do.
	for i := 0; i < len(peers); i++ {
		peerUpdate := <-peerUpdates.Updates()
		peerID := peerUpdate.NodeID
		require.Equal(t, p2p.PeerUpdate{
			NodeID: peerID,
			Status: p2p.PeerStatusUp,
		}, peerUpdate)

		channel.Out <- p2p.Envelope{To: peerID, Message: &TestMessage{Value: "hi!"}}
		assert.Equal(t, p2p.Envelope{
			From:    peerID,
			Message: &TestMessage{Value: "hi!"},
		}, <-channel.In)
	}

	// We now send a broadcast, which we should return back from all peers.
	channel.Out <- p2p.Envelope{
		Broadcast: true,
		Message:   &TestMessage{Value: "broadcast"},
	}
	for i := 0; i < len(peers); i++ {
		envelope := <-channel.In
		require.Equal(t, &TestMessage{Value: "broadcast"}, envelope.Message)
	}

	// We then submit an error for a peer, and watch it get disconnected.
	channel.Error <- p2p.PeerError{
		NodeID: peers[0].NodeID,
		Err:    errors.New("test error"),
	}
	peerUpdate := <-peerUpdates.Updates()
	require.Equal(t, p2p.PeerUpdate{
		NodeID: peers[0].NodeID,
		Status: p2p.PeerStatusDown,
	}, peerUpdate)

	// The peer manager will automatically reconnect the peer, so we wait
	// for that to happen.
	peerUpdate = <-peerUpdates.Updates()
	require.Equal(t, p2p.PeerUpdate{
		NodeID: peers[0].NodeID,
		Status: p2p.PeerStatusUp,
	}, peerUpdate)
}
