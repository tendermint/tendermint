package p2p_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/gogo/protobuf/proto"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/crypto"
	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/p2p/mocks"
	"github.com/tendermint/tendermint/internal/p2p/p2ptest"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
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
	network := p2ptest.MakeNetwork(t, p2ptest.NetworkOptions{NumNodes: 8})
	local := network.RandomNode()
	peers := network.Peers(local.NodeID)
	channels := network.MakeChannels(t, chDesc, &p2ptest.Message{}, 0)

	network.Start(t)

	channel := channels[local.NodeID]
	for _, peer := range peers {
		go echoReactor(channels[peer.NodeID])
	}

	// Sending a message to each peer should work.
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
	peerUpdates := local.MakePeerUpdatesNoRequireEmpty(t)
	channel.Error <- p2p.PeerError{
		NodeID: peers[0].NodeID,
		Err:    errors.New("boom"),
	}
	p2ptest.RequireUpdates(t, peerUpdates, []p2p.PeerUpdate{
		{NodeID: peers[0].NodeID, Status: p2p.PeerStatusDown},
		{NodeID: peers[0].NodeID, Status: p2p.PeerStatusUp},
	})
}

func TestRouter_Channel_Basic(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	// Set up a router with no transports (so no peers).
	peerManager, err := p2p.NewPeerManager(selfID, dbm.NewMemDB(), p2p.PeerManagerOptions{})
	require.NoError(t, err)
	defer peerManager.Close()

	router, err := p2p.NewRouter(
		log.TestingLogger(),
		p2p.NopMetrics(),
		selfInfo,
		selfKey,
		peerManager,
		nil,
		p2p.RouterOptions{},
	)
	require.NoError(t, err)

	require.NoError(t, router.Start())
	t.Cleanup(func() {
		require.NoError(t, router.Stop())
	})

	// Opening a channel should work.
	channel, err := router.OpenChannel(chDesc, &p2ptest.Message{}, 0)
	require.NoError(t, err)
	require.Contains(t, router.NodeInfo().Channels, chDesc.ID)

	// Opening the same channel again should fail.
	_, err = router.OpenChannel(chDesc, &p2ptest.Message{}, 0)
	require.Error(t, err)

	// Opening a different channel should work.
	chDesc2 := p2p.ChannelDescriptor{ID: byte(2)}
	_, err = router.OpenChannel(chDesc2, &p2ptest.Message{}, 0)
	require.NoError(t, err)
	require.Contains(t, router.NodeInfo().Channels, chDesc2.ID)

	// Closing the channel, then opening it again should be fine.
	channel.Close()
	time.Sleep(100 * time.Millisecond) // yes yes, but Close() is async...

	channel, err = router.OpenChannel(chDesc, &p2ptest.Message{}, 0)
	require.NoError(t, err)

	// We should be able to send on the channel, even though there are no peers.
	p2ptest.RequireSend(t, channel, p2p.Envelope{
		To:      types.NodeID(strings.Repeat("a", 40)),
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
	network := p2ptest.MakeNetwork(t, p2ptest.NetworkOptions{NumNodes: 3})

	ids := network.NodeIDs()
	aID, bID, cID := ids[0], ids[1], ids[2]
	channels := network.MakeChannels(t, chDesc, &p2ptest.Message{}, 0)
	a, b, c := channels[aID], channels[bID], channels[cID]
	otherChannels := network.MakeChannels(t, p2ptest.MakeChannelDesc(9), &p2ptest.Message{}, 0)

	network.Start(t)

	// Sending a message a->b should work, and not send anything
	// further to a, b, or c.
	p2ptest.RequireSend(t, a, p2p.Envelope{To: bID, Message: &p2ptest.Message{Value: "foo"}})
	p2ptest.RequireReceive(t, b, p2p.Envelope{From: aID, Message: &p2ptest.Message{Value: "foo"}})
	p2ptest.RequireEmpty(t, a, b, c)

	// Sending a nil message a->b should be dropped.
	p2ptest.RequireSend(t, a, p2p.Envelope{To: bID, Message: nil})
	p2ptest.RequireEmpty(t, a, b, c)

	// Sending a different message type should be dropped.
	p2ptest.RequireSend(t, a, p2p.Envelope{To: bID, Message: &gogotypes.BoolValue{Value: true}})
	p2ptest.RequireEmpty(t, a, b, c)

	// Sending to an unknown peer should be dropped.
	p2ptest.RequireSend(t, a, p2p.Envelope{
		To:      types.NodeID(strings.Repeat("a", 40)),
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
	network := p2ptest.MakeNetwork(t, p2ptest.NetworkOptions{NumNodes: 4})

	ids := network.NodeIDs()
	aID, bID, cID, dID := ids[0], ids[1], ids[2], ids[3]
	channels := network.MakeChannels(t, chDesc, &p2ptest.Message{}, 0)
	a, b, c, d := channels[aID], channels[bID], channels[cID], channels[dID]

	network.Start(t)

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

func TestRouter_Channel_Wrapper(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	// Create a test network and open a channel on all nodes.
	network := p2ptest.MakeNetwork(t, p2ptest.NetworkOptions{NumNodes: 2})

	ids := network.NodeIDs()
	aID, bID := ids[0], ids[1]
	channels := network.MakeChannels(t, chDesc, &wrapperMessage{}, 0)
	a, b := channels[aID], channels[bID]

	network.Start(t)

	// Since wrapperMessage implements p2p.Wrapper and handles Message, it
	// should automatically wrap and unwrap sent messages -- we prepend the
	// wrapper actions to the message value to signal this.
	p2ptest.RequireSend(t, a, p2p.Envelope{To: bID, Message: &p2ptest.Message{Value: "foo"}})
	p2ptest.RequireReceive(t, b, p2p.Envelope{From: aID, Message: &p2ptest.Message{Value: "unwrap:wrap:foo"}})

	// If we send a different message that can't be wrapped, it should be dropped.
	p2ptest.RequireSend(t, a, p2p.Envelope{To: bID, Message: &gogotypes.BoolValue{Value: true}})
	p2ptest.RequireEmpty(t, b)

	// If we send the wrapper message itself, it should also be passed through
	// since WrapperMessage supports it, and should only be unwrapped at the receiver.
	p2ptest.RequireSend(t, a, p2p.Envelope{
		To:      bID,
		Message: &wrapperMessage{Message: p2ptest.Message{Value: "foo"}},
	})
	p2ptest.RequireReceive(t, b, p2p.Envelope{
		From:    aID,
		Message: &p2ptest.Message{Value: "unwrap:foo"},
	})

}

// WrapperMessage prepends the value with "wrap:" and "unwrap:" to test it.
type wrapperMessage struct {
	p2ptest.Message
}

var _ p2p.Wrapper = (*wrapperMessage)(nil)

func (w *wrapperMessage) Wrap(inner proto.Message) error {
	switch inner := inner.(type) {
	case *p2ptest.Message:
		w.Message.Value = fmt.Sprintf("wrap:%v", inner.Value)
	case *wrapperMessage:
		*w = *inner
	default:
		return fmt.Errorf("invalid message type %T", inner)
	}
	return nil
}

func (w *wrapperMessage) Unwrap() (proto.Message, error) {
	return &p2ptest.Message{Value: fmt.Sprintf("unwrap:%v", w.Message.Value)}, nil
}

func TestRouter_Channel_Error(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	// Create a test network and open a channel on all nodes.
	network := p2ptest.MakeNetwork(t, p2ptest.NetworkOptions{NumNodes: 3})
	network.Start(t)

	ids := network.NodeIDs()
	aID, bID := ids[0], ids[1]
	channels := network.MakeChannels(t, chDesc, &p2ptest.Message{}, 0)
	a := channels[aID]

	// Erroring b should cause it to be disconnected. It will reconnect shortly after.
	sub := network.Nodes[aID].MakePeerUpdates(t)
	p2ptest.RequireError(t, a, p2p.PeerError{NodeID: bID, Err: errors.New("boom")})
	p2ptest.RequireUpdates(t, sub, []p2p.PeerUpdate{
		{NodeID: bID, Status: p2p.PeerStatusDown},
		{NodeID: bID, Status: p2p.PeerStatusUp},
	})
}

func TestRouter_AcceptPeers(t *testing.T) {
	testcases := map[string]struct {
		peerInfo types.NodeInfo
		peerKey  crypto.PubKey
		ok       bool
	}{
		"valid handshake": {peerInfo, peerKey.PubKey(), true},
		"empty handshake": {types.NodeInfo{}, nil, false},
		"invalid key":     {peerInfo, selfKey.PubKey(), false},
		"self handshake":  {selfInfo, selfKey.PubKey(), false},
		"incompatible peer": {
			types.NodeInfo{
				NodeID:     peerID,
				ListenAddr: "0.0.0.0:0",
				Network:    "other-network",
				Moniker:    string(peerID),
			},
			peerKey.PubKey(),
			false,
		},
	}
	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Cleanup(leaktest.Check(t))

			// Set up a mock transport that handshakes.
			closer := tmsync.NewCloser()
			mockConnection := &mocks.Connection{}
			mockConnection.On("String").Maybe().Return("mock")
			mockConnection.On("Handshake", mock.Anything, selfInfo, selfKey).
				Return(tc.peerInfo, tc.peerKey, nil)
			mockConnection.On("Close").Run(func(_ mock.Arguments) { closer.Close() }).Return(nil)
			mockConnection.On("RemoteEndpoint").Return(p2p.Endpoint{})
			if tc.ok {
				// without the sleep after RequireUpdate this method isn't
				// always called. Consider making this call optional.
				mockConnection.On("ReceiveMessage").Return(chID, nil, io.EOF)
			}

			mockTransport := &mocks.Transport{}
			mockTransport.On("String").Maybe().Return("mock")
			mockTransport.On("Protocols").Return([]p2p.Protocol{"mock"})
			mockTransport.On("Close").Return(nil)
			mockTransport.On("Accept").Once().Return(mockConnection, nil)
			mockTransport.On("Accept").Once().Return(nil, io.EOF)

			// Set up and start the router.
			peerManager, err := p2p.NewPeerManager(selfID, dbm.NewMemDB(), p2p.PeerManagerOptions{})
			require.NoError(t, err)
			defer peerManager.Close()

			sub := peerManager.Subscribe()
			defer sub.Close()

			router, err := p2p.NewRouter(
				log.TestingLogger(),
				p2p.NopMetrics(),
				selfInfo,
				selfKey,
				peerManager,
				[]p2p.Transport{mockTransport},
				p2p.RouterOptions{},
			)
			require.NoError(t, err)
			require.NoError(t, router.Start())

			if tc.ok {
				p2ptest.RequireUpdate(t, sub, p2p.PeerUpdate{
					NodeID: tc.peerInfo.NodeID,
					Status: p2p.PeerStatusUp,
				})
				// force a context switch so that the
				// connection is handled.
				time.Sleep(time.Millisecond)
				sub.Close()
			} else {
				select {
				case <-closer.Done():
				case <-time.After(100 * time.Millisecond):
					require.Fail(t, "connection not closed")
				}
			}

			require.NoError(t, router.Stop())
			mockTransport.AssertExpectations(t)
			mockConnection.AssertExpectations(t)
		})
	}
}

func TestRouter_AcceptPeers_Errors(t *testing.T) {
	for _, err := range []error{io.EOF} {
		t.Run(err.Error(), func(t *testing.T) {
			t.Cleanup(leaktest.Check(t))

			// Set up a mock transport that returns io.EOF once, which should prevent
			// the router from calling Accept again.
			mockTransport := &mocks.Transport{}
			mockTransport.On("String").Maybe().Return("mock")
			mockTransport.On("Accept", mock.Anything).Once().Return(nil, err)
			mockTransport.On("Listen", mock.Anything).Return(nil).Maybe()
			mockTransport.On("Close").Return(nil)
			mockTransport.On("Protocols").Return([]p2p.Protocol{"mock"})
			// Set up and start the router.
			peerManager, err := p2p.NewPeerManager(selfID, dbm.NewMemDB(), p2p.PeerManagerOptions{})
			require.NoError(t, err)

			router, err := p2p.NewRouter(
				log.TestingLogger(),
				p2p.NopMetrics(),
				selfInfo,
				selfKey,
				peerManager,
				[]p2p.Transport{mockTransport},
				p2p.RouterOptions{},
			)
			require.NoError(t, err)

			require.NoError(t, router.Start())
			time.Sleep(time.Second)
			require.NoError(t, router.Stop())

			mockTransport.AssertExpectations(t)

		})
	}
}

func TestRouter_AcceptPeers_HeadOfLineBlocking(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	// Set up a mock transport that returns a connection that blocks during the
	// handshake. It should be able to accept several of these in parallel, i.e.
	// a single connection can't halt other connections being accepted.
	acceptCh := make(chan bool, 3)
	closeCh := make(chan time.Time)

	mockConnection := &mocks.Connection{}
	mockConnection.On("String").Maybe().Return("mock")
	mockConnection.On("Handshake", mock.Anything, selfInfo, selfKey).
		WaitUntil(closeCh).Return(types.NodeInfo{}, nil, io.EOF)
	mockConnection.On("Close").Return(nil)
	mockConnection.On("RemoteEndpoint").Return(p2p.Endpoint{})

	mockTransport := &mocks.Transport{}
	mockTransport.On("String").Maybe().Return("mock")
	mockTransport.On("Protocols").Return([]p2p.Protocol{"mock"})
	mockTransport.On("Close").Return(nil)
	mockTransport.On("Accept").Times(3).Run(func(_ mock.Arguments) {
		acceptCh <- true
	}).Return(mockConnection, nil)
	mockTransport.On("Accept").Once().Return(nil, io.EOF)

	// Set up and start the router.
	peerManager, err := p2p.NewPeerManager(selfID, dbm.NewMemDB(), p2p.PeerManagerOptions{})
	require.NoError(t, err)
	defer peerManager.Close()

	router, err := p2p.NewRouter(
		log.TestingLogger(),
		p2p.NopMetrics(),
		selfInfo,
		selfKey,
		peerManager,
		[]p2p.Transport{mockTransport},
		p2p.RouterOptions{},
	)
	require.NoError(t, err)
	require.NoError(t, router.Start())

	require.Eventually(t, func() bool {
		return len(acceptCh) == 3
	}, time.Second, 10*time.Millisecond)
	close(closeCh)
	time.Sleep(100 * time.Millisecond)

	require.NoError(t, router.Stop())
	mockTransport.AssertExpectations(t)
	mockConnection.AssertExpectations(t)
}

func TestRouter_DialPeers(t *testing.T) {
	testcases := map[string]struct {
		dialID   types.NodeID
		peerInfo types.NodeInfo
		peerKey  crypto.PubKey
		dialErr  error
		ok       bool
	}{
		"valid dial":         {peerInfo.NodeID, peerInfo, peerKey.PubKey(), nil, true},
		"empty handshake":    {peerInfo.NodeID, types.NodeInfo{}, nil, nil, false},
		"invalid key":        {peerInfo.NodeID, peerInfo, selfKey.PubKey(), nil, false},
		"unexpected node ID": {peerInfo.NodeID, selfInfo, selfKey.PubKey(), nil, false},
		"dial error":         {peerInfo.NodeID, peerInfo, peerKey.PubKey(), errors.New("boom"), false},
		"incompatible peer": {
			peerInfo.NodeID,
			types.NodeInfo{
				NodeID:     peerID,
				ListenAddr: "0.0.0.0:0",
				Network:    "other-network",
				Moniker:    string(peerID),
			},
			peerKey.PubKey(),
			nil,
			false,
		},
	}
	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Cleanup(leaktest.Check(t))

			address := p2p.NodeAddress{Protocol: "mock", NodeID: tc.dialID}
			endpoint := p2p.Endpoint{Protocol: "mock", Path: string(tc.dialID)}

			// Set up a mock transport that handshakes.
			closer := tmsync.NewCloser()
			mockConnection := &mocks.Connection{}
			mockConnection.On("String").Maybe().Return("mock")
			if tc.dialErr == nil {
				mockConnection.On("Handshake", mock.Anything, selfInfo, selfKey).
					Return(tc.peerInfo, tc.peerKey, nil)
				mockConnection.On("Close").Run(func(_ mock.Arguments) { closer.Close() }).Return(nil)
			}
			if tc.ok {
				// without the sleep after RequireUpdate this method isn't
				// always called. Consider making this call optional.
				mockConnection.On("ReceiveMessage").Return(chID, nil, io.EOF)
			}

			mockTransport := &mocks.Transport{}
			mockTransport.On("String").Maybe().Return("mock")
			mockTransport.On("Protocols").Return([]p2p.Protocol{"mock"})
			mockTransport.On("Close").Return(nil)
			mockTransport.On("Accept").Maybe().Return(nil, io.EOF)
			if tc.dialErr == nil {
				mockTransport.On("Dial", mock.Anything, endpoint).Once().Return(mockConnection, nil)
				// This handles the retry when a dialed connection gets closed after ReceiveMessage
				// returns io.EOF above.
				mockTransport.On("Dial", mock.Anything, endpoint).Maybe().Return(nil, io.EOF)
			} else {
				mockTransport.On("Dial", mock.Anything, endpoint).Once().
					Run(func(_ mock.Arguments) { closer.Close() }).
					Return(nil, tc.dialErr)
			}

			// Set up and start the router.
			peerManager, err := p2p.NewPeerManager(selfID, dbm.NewMemDB(), p2p.PeerManagerOptions{})
			require.NoError(t, err)
			defer peerManager.Close()

			added, err := peerManager.Add(address)
			require.NoError(t, err)
			require.True(t, added)
			sub := peerManager.Subscribe()
			defer sub.Close()

			router, err := p2p.NewRouter(
				log.TestingLogger(),
				p2p.NopMetrics(),
				selfInfo,
				selfKey,
				peerManager,
				[]p2p.Transport{mockTransport},
				p2p.RouterOptions{},
			)
			require.NoError(t, err)
			require.NoError(t, router.Start())

			if tc.ok {
				p2ptest.RequireUpdate(t, sub, p2p.PeerUpdate{
					NodeID: tc.peerInfo.NodeID,
					Status: p2p.PeerStatusUp,
				})
				// force a context switch so that the
				// connection is handled.
				time.Sleep(time.Millisecond)
				sub.Close()
			} else {
				select {
				case <-closer.Done():
				case <-time.After(100 * time.Millisecond):
					require.Fail(t, "connection not closed")
				}
			}

			require.NoError(t, router.Stop())
			mockTransport.AssertExpectations(t)
			mockConnection.AssertExpectations(t)
		})
	}
}

func TestRouter_DialPeers_Parallel(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	a := p2p.NodeAddress{Protocol: "mock", NodeID: types.NodeID(strings.Repeat("a", 40))}
	b := p2p.NodeAddress{Protocol: "mock", NodeID: types.NodeID(strings.Repeat("b", 40))}
	c := p2p.NodeAddress{Protocol: "mock", NodeID: types.NodeID(strings.Repeat("c", 40))}

	// Set up a mock transport that returns a connection that blocks during the
	// handshake. It should dial all peers in parallel.
	dialCh := make(chan bool, 3)
	closeCh := make(chan time.Time)

	mockConnection := &mocks.Connection{}
	mockConnection.On("String").Maybe().Return("mock")
	mockConnection.On("Handshake", mock.Anything, selfInfo, selfKey).
		WaitUntil(closeCh).Return(types.NodeInfo{}, nil, io.EOF)
	mockConnection.On("Close").Return(nil)

	mockTransport := &mocks.Transport{}
	mockTransport.On("String").Maybe().Return("mock")
	mockTransport.On("Protocols").Return([]p2p.Protocol{"mock"})
	mockTransport.On("Close").Return(nil)
	mockTransport.On("Accept").Once().Return(nil, io.EOF)
	for _, address := range []p2p.NodeAddress{a, b, c} {
		endpoint := p2p.Endpoint{Protocol: address.Protocol, Path: string(address.NodeID)}
		mockTransport.On("Dial", mock.Anything, endpoint).Run(func(_ mock.Arguments) {
			dialCh <- true
		}).Return(mockConnection, nil)
	}

	// Set up and start the router.
	peerManager, err := p2p.NewPeerManager(selfID, dbm.NewMemDB(), p2p.PeerManagerOptions{})
	require.NoError(t, err)
	defer peerManager.Close()

	added, err := peerManager.Add(a)
	require.NoError(t, err)
	require.True(t, added)

	added, err = peerManager.Add(b)
	require.NoError(t, err)
	require.True(t, added)

	added, err = peerManager.Add(c)
	require.NoError(t, err)
	require.True(t, added)

	router, err := p2p.NewRouter(
		log.TestingLogger(),
		p2p.NopMetrics(),
		selfInfo,
		selfKey,
		peerManager,
		[]p2p.Transport{mockTransport},
		p2p.RouterOptions{
			DialSleep: func(_ context.Context) {},
			NumConcurrentDials: func() int {
				ncpu := runtime.NumCPU()
				if ncpu <= 3 {
					return 3
				}
				return ncpu
			},
		},
	)

	require.NoError(t, err)
	require.NoError(t, router.Start())

	require.Eventually(t,
		func() bool {
			return len(dialCh) == 3
		},
		5*time.Second,
		100*time.Millisecond,
		"reached %d rather than 3", len(dialCh))

	close(closeCh)
	time.Sleep(500 * time.Millisecond)

	require.NoError(t, router.Stop())
	mockTransport.AssertExpectations(t)
	mockConnection.AssertExpectations(t)
}

func TestRouter_EvictPeers(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	// Set up a mock transport that we can evict.
	closeCh := make(chan time.Time)
	closeOnce := sync.Once{}

	mockConnection := &mocks.Connection{}
	mockConnection.On("String").Maybe().Return("mock")
	mockConnection.On("Handshake", mock.Anything, selfInfo, selfKey).
		Return(peerInfo, peerKey.PubKey(), nil)
	mockConnection.On("ReceiveMessage").WaitUntil(closeCh).Return(chID, nil, io.EOF)
	mockConnection.On("RemoteEndpoint").Return(p2p.Endpoint{})
	mockConnection.On("Close").Run(func(_ mock.Arguments) {
		closeOnce.Do(func() {
			close(closeCh)
		})
	}).Return(nil)

	mockTransport := &mocks.Transport{}
	mockTransport.On("String").Maybe().Return("mock")
	mockTransport.On("Protocols").Return([]p2p.Protocol{"mock"})
	mockTransport.On("Close").Return(nil)
	mockTransport.On("Accept").Once().Return(mockConnection, nil)
	mockTransport.On("Accept").Once().Return(nil, io.EOF)

	// Set up and start the router.
	peerManager, err := p2p.NewPeerManager(selfID, dbm.NewMemDB(), p2p.PeerManagerOptions{})
	require.NoError(t, err)
	defer peerManager.Close()

	sub := peerManager.Subscribe()
	defer sub.Close()

	router, err := p2p.NewRouter(
		log.TestingLogger(),
		p2p.NopMetrics(),
		selfInfo,
		selfKey,
		peerManager,
		[]p2p.Transport{mockTransport},
		p2p.RouterOptions{},
	)
	require.NoError(t, err)
	require.NoError(t, router.Start())

	// Wait for the mock peer to connect, then evict it by reporting an error.
	p2ptest.RequireUpdate(t, sub, p2p.PeerUpdate{
		NodeID: peerInfo.NodeID,
		Status: p2p.PeerStatusUp,
	})

	peerManager.Errored(peerInfo.NodeID, errors.New("boom"))

	p2ptest.RequireUpdate(t, sub, p2p.PeerUpdate{
		NodeID: peerInfo.NodeID,
		Status: p2p.PeerStatusDown,
	})
	sub.Close()

	require.NoError(t, router.Stop())
	mockTransport.AssertExpectations(t)
	mockConnection.AssertExpectations(t)
}

func TestRouter_ChannelCompatability(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	incompatiblePeer := types.NodeInfo{
		NodeID:     peerID,
		ListenAddr: "0.0.0.0:0",
		Network:    "test",
		Moniker:    string(peerID),
		Channels:   []byte{0x03},
	}

	mockConnection := &mocks.Connection{}
	mockConnection.On("String").Maybe().Return("mock")
	mockConnection.On("Handshake", mock.Anything, selfInfo, selfKey).
		Return(incompatiblePeer, peerKey.PubKey(), nil)
	mockConnection.On("RemoteEndpoint").Return(p2p.Endpoint{})
	mockConnection.On("Close").Return(nil)

	mockTransport := &mocks.Transport{}
	mockTransport.On("String").Maybe().Return("mock")
	mockTransport.On("Protocols").Return([]p2p.Protocol{"mock"})
	mockTransport.On("Close").Return(nil)
	mockTransport.On("Accept").Once().Return(mockConnection, nil)
	mockTransport.On("Accept").Once().Return(nil, io.EOF)

	// Set up and start the router.
	peerManager, err := p2p.NewPeerManager(selfID, dbm.NewMemDB(), p2p.PeerManagerOptions{})
	require.NoError(t, err)
	defer peerManager.Close()

	router, err := p2p.NewRouter(
		log.TestingLogger(),
		p2p.NopMetrics(),
		selfInfo,
		selfKey,
		peerManager,
		[]p2p.Transport{mockTransport},
		p2p.RouterOptions{},
	)
	require.NoError(t, err)
	require.NoError(t, router.Start())
	time.Sleep(1 * time.Second)
	require.NoError(t, router.Stop())
	require.Empty(t, peerManager.Peers())

	mockConnection.AssertExpectations(t)
	mockTransport.AssertExpectations(t)
}

func TestRouter_DontSendOnInvalidChannel(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	peer := types.NodeInfo{
		NodeID:     peerID,
		ListenAddr: "0.0.0.0:0",
		Network:    "test",
		Moniker:    string(peerID),
		Channels:   []byte{0x02},
	}

	mockConnection := &mocks.Connection{}
	mockConnection.On("String").Maybe().Return("mock")
	mockConnection.On("Handshake", mock.Anything, selfInfo, selfKey).
		Return(peer, peerKey.PubKey(), nil)
	mockConnection.On("RemoteEndpoint").Return(p2p.Endpoint{})
	mockConnection.On("Close").Return(nil)
	mockConnection.On("ReceiveMessage").Return(chID, nil, io.EOF)

	mockTransport := &mocks.Transport{}
	mockTransport.On("String").Maybe().Return("mock")
	mockTransport.On("Protocols").Return([]p2p.Protocol{"mock"})
	mockTransport.On("Close").Return(nil)
	mockTransport.On("Accept").Once().Return(mockConnection, nil)
	mockTransport.On("Accept").Once().Return(nil, io.EOF)

	// Set up and start the router.
	peerManager, err := p2p.NewPeerManager(selfID, dbm.NewMemDB(), p2p.PeerManagerOptions{})
	require.NoError(t, err)
	defer peerManager.Close()

	sub := peerManager.Subscribe()
	defer sub.Close()

	router, err := p2p.NewRouter(
		log.TestingLogger(),
		p2p.NopMetrics(),
		selfInfo,
		selfKey,
		peerManager,
		[]p2p.Transport{mockTransport},
		p2p.RouterOptions{},
	)
	require.NoError(t, err)
	require.NoError(t, router.Start())

	p2ptest.RequireUpdate(t, sub, p2p.PeerUpdate{
		NodeID: peerInfo.NodeID,
		Status: p2p.PeerStatusUp,
	})

	channel, err := router.OpenChannel(chDesc, &p2ptest.Message{}, 0)
	require.NoError(t, err)

	channel.Out <- p2p.Envelope{
		To:      peer.NodeID,
		Message: &p2ptest.Message{Value: "Hi"},
	}

	require.NoError(t, router.Stop())
	mockTransport.AssertExpectations(t)
}
