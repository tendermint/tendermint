package p2p_test

import (
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/mocks"
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

func TestRouter_Network(t *testing.T) {
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

func TestRouter_Channel(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	// Set up a router with no transports (so no peers).
	peerManager, err := p2p.NewPeerManager(selfID, dbm.NewMemDB(), p2p.PeerManagerOptions{})
	require.NoError(t, err)
	router, err := p2p.NewRouter(log.TestingLogger(), selfInfo, selfKey, peerManager, nil, p2p.RouterOptions{})
	require.NoError(t, err)

	require.NoError(t, router.Start())
	t.Cleanup(func() {
		require.NoError(t, router.Stop())
	})

	// Opening a channel should work.
	channel, err := router.OpenChannel(chID, &p2ptest.Message{})
	require.NoError(t, err)

	// Opening the same channel again should fail.
	_, err = router.OpenChannel(chID, &p2ptest.Message{})
	require.Error(t, err)

	// Opening a different channel should work.
	_, err = router.OpenChannel(2, &p2ptest.Message{})
	require.NoError(t, err)

	// Closing the channel, then opening it again should be fine.
	channel.Close()
	time.Sleep(100 * time.Millisecond) // yes yes, but Close() is async...

	channel, err = router.OpenChannel(chID, &p2ptest.Message{})
	require.NoError(t, err)

	// We should be able to send on the channel, even though there are no peers.
	p2ptest.RequireSend(t, channel, p2p.Envelope{
		To:      p2p.NodeID(strings.Repeat("a", 40)),
		Message: &p2ptest.Message{Value: "foo"},
	})

	// A message to ourselves should be dropped.
	p2ptest.RequireSend(t, channel, p2p.Envelope{
		To:      selfID,
		Message: &p2ptest.Message{Value: "self"},
	})
	p2ptest.RequireEmpty(t, channel)
}

// Channel tests are hairy to mock, so we use an in-memory network instead.
func TestRouter_Channel_SendReceive(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	// Create a test network and open a channel on all nodes.
	network := p2ptest.MakeNetwork(t, 3)
	ids := network.NodeIDs()
	aID, bID, cID := ids[0], ids[1], ids[2]
	channels := network.MakeChannels(t, chID, &p2ptest.Message{})
	a, b, c := channels[aID], channels[bID], channels[cID]
	otherChannels := network.MakeChannels(t, 9, &p2ptest.Message{})

	// Sending a message a->b should work, and not send anything
	// further to a, b, or c.
	p2ptest.RequireSend(t, a, p2p.Envelope{To: bID, Message: &p2ptest.Message{Value: "foo"}})
	p2ptest.RequireReceive(t, b, p2p.Envelope{From: aID, Message: &p2ptest.Message{Value: "foo"}})
	p2ptest.RequireEmpty(t, a, b, c)

	// Sending a different message type should be dropped.
	p2ptest.RequireSend(t, a, p2p.Envelope{To: bID, Message: &gogotypes.BoolValue{Value: true}})
	p2ptest.RequireEmpty(t, a, b, c)

	// Sending to an unknown peer should be dropped.
	p2ptest.RequireSend(t, a, p2p.Envelope{
		To:      p2p.NodeID(strings.Repeat("a", 40)),
		Message: &p2ptest.Message{Value: "a"},
	})
	p2ptest.RequireEmpty(t, a, b, c)

	// Sending without a recipient should be dropped.
	p2ptest.RequireSend(t, a, p2p.Envelope{Message: &p2ptest.Message{Value: "noto"}})
	p2ptest.RequireEmpty(t, a, b, c)

	// Sending to self should be dropped.
	p2ptest.RequireSend(t, a, p2p.Envelope{To: aID, Message: &p2ptest.Message{Value: "self"}})
	p2ptest.RequireEmpty(t, a, b, c)

	// Removing b and sending to it should be dropped.
	network.Remove(t, bID)
	p2ptest.RequireSend(t, a, p2p.Envelope{To: bID, Message: &p2ptest.Message{Value: "nob"}})
	p2ptest.RequireEmpty(t, a, b, c)

	// After all this, sending a message c->a should work.
	p2ptest.RequireSend(t, c, p2p.Envelope{To: aID, Message: &p2ptest.Message{Value: "bar"}})
	p2ptest.RequireReceive(t, a, p2p.Envelope{From: cID, Message: &p2ptest.Message{Value: "bar"}})
	p2ptest.RequireEmpty(t, a, b, c)

	// None of these messages should have made it onto the other channels.
	for _, other := range otherChannels {
		p2ptest.RequireEmpty(t, other)
	}
}

func TestRouter_Channel_Broadcast(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	// Create a test network and open a channel on all nodes.
	network := p2ptest.MakeNetwork(t, 4)
	ids := network.NodeIDs()
	aID, bID, cID, dID := ids[0], ids[1], ids[2], ids[3]
	channels := network.MakeChannels(t, 1, &p2ptest.Message{})
	a, b, c, d := channels[aID], channels[bID], channels[cID], channels[dID]

	// Sending a broadcast from b should work.
	p2ptest.RequireSend(t, b, p2p.Envelope{Broadcast: true, Message: &p2ptest.Message{Value: "foo"}})
	p2ptest.RequireReceive(t, a, p2p.Envelope{From: bID, Message: &p2ptest.Message{Value: "foo"}})
	p2ptest.RequireReceive(t, c, p2p.Envelope{From: bID, Message: &p2ptest.Message{Value: "foo"}})
	p2ptest.RequireReceive(t, d, p2p.Envelope{From: bID, Message: &p2ptest.Message{Value: "foo"}})
	p2ptest.RequireEmpty(t, a, b, c, d)

	// Removing one node from the network shouldn't prevent broadcasts from working.
	network.Remove(t, dID)
	p2ptest.RequireSend(t, a, p2p.Envelope{Broadcast: true, Message: &p2ptest.Message{Value: "bar"}})
	p2ptest.RequireReceive(t, b, p2p.Envelope{From: aID, Message: &p2ptest.Message{Value: "bar"}})
	p2ptest.RequireReceive(t, c, p2p.Envelope{From: aID, Message: &p2ptest.Message{Value: "bar"}})
	p2ptest.RequireEmpty(t, a, b, c, d)

}

// FIXME: Remove this.
func TestRouter_Channel_SendMock(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	// Set up a mock transport with a single mock peer.
	closeCh := make(chan time.Time)
	closeOnce := sync.Once{}
	mockCh := make(chan []byte, 1)

	mockConnection := &mocks.Connection{}
	mockConnection.On("String").Return("mock")
	mockConnection.On("Handshake", mock.Anything, selfInfo, selfKey).
		Return(peerInfo, peerKey.PubKey(), nil)
	mockConnection.On("ReceiveMessage").WaitUntil(closeCh).Return(chID, nil, io.EOF)
	mockConnection.On("SendMessage", chID, mock.Anything).Run(func(args mock.Arguments) {
		mockCh <- args.Get(1).([]byte)
	}).Return(true, nil)
	mockConnection.On("Close").Run(func(_ mock.Arguments) {
		closeOnce.Do(func() { close(closeCh) })
	}).Return(nil)

	mockTransport := &mocks.Transport{}
	mockTransport.On("String").Return("mock")
	mockTransport.On("Protocols").Return([]p2p.Protocol{"mock"})
	mockTransport.On("Accept").Once().Return(mockConnection, nil)
	mockTransport.On("Accept").WaitUntil(closeCh).Return(nil, io.EOF)
	mockTransport.On("Close").Return(nil)

	// Set up and start the router.
	peerManager, err := p2p.NewPeerManager(selfID, dbm.NewMemDB(), p2p.PeerManagerOptions{})
	require.NoError(t, err)
	router, err := p2p.NewRouter(log.TestingLogger(), selfInfo, selfKey, peerManager,
		[]p2p.Transport{mockTransport}, p2p.RouterOptions{})
	require.NoError(t, err)

	require.NoError(t, router.Start())
	t.Cleanup(func() {
		require.NoError(t, router.Stop())
	})

	// Wait for the peer to come online.
	sub := peerManager.Subscribe()
	defer sub.Close()
	p2ptest.RequireUpdate(t, sub, p2p.PeerUpdate{
		NodeID: peerID,
		Status: p2p.PeerStatusUp,
	})
	sub.Close()

	// Sending a message to the mock peer should work.
	channel, err := router.OpenChannel(chID, &p2ptest.Message{})
	require.NoError(t, err)

	p2ptest.RequireSend(t, channel, p2p.Envelope{
		To:      peerID,
		Message: &p2ptest.Message{Value: "foo"},
	})
	//require.Equal(t, []byte{}, <-mockCh)
}
