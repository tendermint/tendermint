package p2p_test

import (
	"sync"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/p2p"
	p2pmocks "github.com/tendermint/tendermint/p2p/mocks"
	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
)

var (
	channelID1 = byte(0x01)
	channelID2 = byte(0x02)

	p2pCfg = config.DefaultP2PConfig()

	testChannelShims = map[p2p.ChannelID]*p2p.ChannelDescriptorShim{
		p2p.ChannelID(channelID1): {
			MsgType: new(ssproto.Message),
			Descriptor: &p2p.ChannelDescriptor{
				ID:                  channelID1,
				Priority:            3,
				SendQueueCapacity:   10,
				RecvMessageCapacity: int(4e6),
			},
		},
		p2p.ChannelID(channelID2): {
			MsgType: new(ssproto.Message),
			Descriptor: &p2p.ChannelDescriptor{
				ID:                  channelID2,
				Priority:            1,
				SendQueueCapacity:   4,
				RecvMessageCapacity: int(16e6),
			},
		},
	}
)

type reactorShimTestSuite struct {
	shim *p2p.ReactorShim
	sw   *p2p.Switch
}

func setup(t *testing.T, peers []p2p.Peer) *reactorShimTestSuite {
	t.Helper()

	rts := &reactorShimTestSuite{
		shim: p2p.NewReactorShim("TestShim", testChannelShims),
	}

	rts.sw = p2p.MakeSwitch(p2pCfg, 1, "testing", "123.123.123", func(_ int, sw *p2p.Switch) *p2p.Switch {
		for _, peer := range peers {
			p2p.AddPeerToSwitchPeerSet(sw, peer)
		}

		sw.AddReactor(rts.shim.Name, rts.shim)
		return sw
	})

	// start the reactor shim
	require.NoError(t, rts.shim.Start())

	t.Cleanup(func() {
		require.NoError(t, rts.shim.Stop())

		for _, chs := range rts.shim.Channels {
			chs.Channel.Close()
		}
	})

	return rts
}

func simplePeer(t *testing.T, id string) (*p2pmocks.Peer, p2p.NodeID) {
	t.Helper()

	peerID := p2p.NodeID(id)
	peer := &p2pmocks.Peer{}
	peer.On("ID").Return(peerID)

	return peer, peerID
}

func TestReactorShim_GetChannel(t *testing.T) {
	rts := setup(t, nil)

	p2pCh := rts.shim.GetChannel(p2p.ChannelID(channelID1))
	require.NotNil(t, p2pCh)
	require.Equal(t, p2pCh.ID(), p2p.ChannelID(channelID1))

	p2pCh = rts.shim.GetChannel(p2p.ChannelID(byte(0x03)))
	require.Nil(t, p2pCh)
}

func TestReactorShim_GetChannels(t *testing.T) {
	rts := setup(t, nil)

	p2pChs := rts.shim.GetChannels()
	require.Len(t, p2pChs, 2)
	require.Equal(t, p2p.ChannelID(p2pChs[0].ID), p2p.ChannelID(channelID1))
	require.Equal(t, p2p.ChannelID(p2pChs[1].ID), p2p.ChannelID(channelID2))
}

func TestReactorShim_AddPeer(t *testing.T) {
	peerA, peerIDA := simplePeer(t, "aa")
	rts := setup(t, []p2p.Peer{peerA})

	var wg sync.WaitGroup
	wg.Add(1)

	var peerUpdate p2p.PeerUpdate
	go func() {
		peerUpdate = <-rts.shim.PeerUpdates.Updates()
		wg.Done()
	}()

	rts.shim.AddPeer(peerA)
	wg.Wait()

	require.Equal(t, peerIDA, peerUpdate.PeerID)
	require.Equal(t, p2p.PeerStatusUp, peerUpdate.Status)
}

func TestReactorShim_RemovePeer(t *testing.T) {
	peerA, peerIDA := simplePeer(t, "aa")
	rts := setup(t, []p2p.Peer{peerA})

	var wg sync.WaitGroup
	wg.Add(1)

	var peerUpdate p2p.PeerUpdate
	go func() {
		peerUpdate = <-rts.shim.PeerUpdates.Updates()
		wg.Done()
	}()

	rts.shim.RemovePeer(peerA, "test reason")
	wg.Wait()

	require.Equal(t, peerIDA, peerUpdate.PeerID)
	require.Equal(t, p2p.PeerStatusDown, peerUpdate.Status)
}

func TestReactorShim_Receive(t *testing.T) {
	peerA, peerIDA := simplePeer(t, "aa")
	rts := setup(t, []p2p.Peer{peerA})

	msg := &ssproto.Message{
		Sum: &ssproto.Message_ChunkRequest{
			ChunkRequest: &ssproto.ChunkRequest{Height: 1, Format: 1, Index: 1},
		},
	}

	bz, err := proto.Marshal(msg)
	require.NoError(t, err)

	var wg sync.WaitGroup

	var response *ssproto.Message
	peerA.On("Send", channelID1, mock.Anything).Run(func(args mock.Arguments) {
		m := &ssproto.Message{}
		require.NoError(t, proto.Unmarshal(args[1].([]byte), m))

		response = m
		wg.Done()
	}).Return(true)

	p2pCh := rts.shim.Channels[p2p.ChannelID(channelID1)]

	wg.Add(2)

	// Simulate receiving the envelope in some real reactor and replying back with
	// the same envelope and then closing the Channel.
	go func() {
		e := <-p2pCh.Channel.In()
		require.Equal(t, peerIDA, e.From)
		require.NotNil(t, e.Message)

		p2pCh.Channel.Out() <- p2p.Envelope{To: e.From, Message: e.Message}
		p2pCh.Channel.Close()
		wg.Done()
	}()

	rts.shim.Receive(channelID1, peerA, bz)

	// wait until the mock peer called Send and we (fake) proxied the envelope
	wg.Wait()
	require.NotNil(t, response)

	m, err := response.Unwrap()
	require.NoError(t, err)
	require.Equal(t, msg.GetChunkRequest(), m)

	// Since p2pCh was closed in the simulated reactor above, calling Receive
	// should not block.
	rts.shim.Receive(channelID1, peerA, bz)
	require.Empty(t, p2pCh.Channel.In())

	peerA.AssertExpectations(t)
}
