package p2p_test

import (
	"errors"
	"testing"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/p2ptest"
)

func echoReactor(channel *p2p.Channel) {
	for {
		select {
		case envelope := <-channel.In:
			value := envelope.Message.(*p2ptest.StringMessage).Value
			channel.Out <- p2p.Envelope{
				To:      envelope.From,
				Message: &p2ptest.StringMessage{Value: value},
			}
		case <-channel.Done():
			return
		}
	}
}

func TestNetwork(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	// Create a test network and open a channel where all peers run echoReactor.
	network := p2ptest.MakeNetwork(t, 4)
	local := network.RandomNode()
	peers := network.Peers(local.NodeID)
	channel := network.MakeChannelAtNode(t, local.NodeID, 1, &p2ptest.StringMessage{},
		func(t *testing.T, peerID p2p.NodeID, peerChannel *p2p.Channel) {
			go echoReactor(peerChannel)
		},
	)

	// Sending a message to each peer should return a message.
	for _, peer := range peers {
		channel.Out <- p2p.Envelope{
			To:      peer.NodeID,
			Message: &p2ptest.StringMessage{Value: "foo"},
		}
		e := <-channel.In
		require.Equal(t, p2p.Envelope{
			From:    peer.NodeID,
			Message: &p2ptest.StringMessage{Value: "foo"},
		}, e)
	}

	// Sending a broadcast should return back a message from all peers.
	channel.Out <- p2p.Envelope{
		Broadcast: true,
		Message:   &p2ptest.StringMessage{Value: "foo"},
	}
	seen := map[p2p.NodeID]bool{}
	for i := 0; i < len(network.Nodes)-1; i++ {
		e := <-channel.In
		require.NotEmpty(t, e.From, "received message without sender")
		require.NotEqual(t, local.NodeID, e.From)
		require.NotContains(t, seen, e.From, "received duplicate message from %v", e.From)
		require.Equal(t, &p2ptest.StringMessage{Value: "foo"}, e.Message)
		seen[e.From] = true
	}

	// We then submit an error for a peer, and watch it get disconnected.
	sub := local.PeerManager.Subscribe()
	defer sub.Close()

	channel.Error <- p2p.PeerError{
		NodeID: peers[0].NodeID,
		Err:    errors.New("test error"),
	}
	peerUpdate := <-sub.Updates()
	require.Equal(t, p2p.PeerUpdate{
		NodeID: peers[0].NodeID,
		Status: p2p.PeerStatusDown,
	}, peerUpdate)

	// The peer will automatically get reconnected by the router and peer
	// manager, so we wait for that to happen.
	peerUpdate = <-sub.Updates()
	require.Equal(t, p2p.PeerUpdate{
		NodeID: peers[0].NodeID,
		Status: p2p.PeerStatusUp,
	}, peerUpdate)
}
