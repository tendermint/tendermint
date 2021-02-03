package p2p_test

import (
	"errors"
	"testing"

	"github.com/fortytw2/leaktest"

	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/p2ptest"
)

func echoReactor(channel *p2p.Channel) {
	for {
		select {
		case envelope := <-channel.In:
			value := envelope.Message.(*p2ptest.Message).Value
			channel.Out <- p2p.Envelope{
				To:      envelope.From,
				Message: &p2ptest.Message{Value: value},
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
	channels := network.MakeChannels(t, 1, &p2ptest.Message{})

	channel := channels[local.NodeID]
	for _, peer := range peers {
		go echoReactor(channels[peer.NodeID])
	}

	// Sending a message to each peer should return the same message.
	for _, peer := range peers {
		p2ptest.RequireSendReceive(t, channel, peer.NodeID,
			&p2ptest.Message{Value: "foo"},
			&p2ptest.Message{Value: "foo"},
		)
	}

	// Sending a broadcast should return back a message from all peers.
	p2ptest.RequireSend(t, channel, p2p.Envelope{
		Broadcast: true,
		Message:   &p2ptest.Message{Value: "bar"},
	})
	expect := []p2p.Envelope{}
	for _, peer := range peers {
		expect = append(expect, p2p.Envelope{
			From:    peer.NodeID,
			Message: &p2ptest.Message{Value: "bar"},
		})
	}
	p2ptest.RequireReceiveUnordered(t, channel, expect)

	// We then submit an error for a peer, and watch it get disconnected and
	// then reconnected as the router retries it.
	peerUpdates := local.MakePeerUpdates(t)
	channel.Error <- p2p.PeerError{
		NodeID: peers[0].NodeID,
		Err:    errors.New("boom"),
	}
	p2ptest.RequireUpdates(t, peerUpdates, []p2p.PeerUpdate{
		{NodeID: peers[0].NodeID, Status: p2p.PeerStatusDown},
		{NodeID: peers[0].NodeID, Status: p2p.PeerStatusUp},
	})
}
