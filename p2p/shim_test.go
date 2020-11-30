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

func setup(
	t *testing.T,
	peers []p2p.Peer,
) (*reactorShimTestSuite, func()) {

	rts := &reactorShimTestSuite{
		shim: p2p.NewShim("TestShim", testChannelShims),
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

	teardown := func() {
		require.NoError(t, rts.shim.Stop())

		for _, chs := range rts.shim.Channels {
			require.NoError(t, chs.Channel.Close())
		}
	}

	return rts, teardown
}

func TestReactorShim_GetChannel(t *testing.T) {
	rts, teardown := setup(t, nil)

	t.Cleanup(func() {
		teardown()
	})

	p2pCh := rts.shim.GetChannel(p2p.ChannelID(channelID1))
	require.NotNil(t, p2pCh)
	require.Equal(t, p2pCh.ID, p2p.ChannelID(channelID1))

	p2pCh = rts.shim.GetChannel(p2p.ChannelID(byte(0x03)))
	require.Nil(t, p2pCh)
}

func TestReactorShim_GetChannels(t *testing.T) {
	rts, teardown := setup(t, nil)

	t.Cleanup(func() {
		teardown()
	})

	p2pChs := rts.shim.GetChannels()
	require.Len(t, p2pChs, 2)
	require.Equal(t, p2p.ChannelID(p2pChs[0].ID), p2p.ChannelID(channelID1))
	require.Equal(t, p2p.ChannelID(p2pChs[1].ID), p2p.ChannelID(channelID2))
}

func TestReactorShim_AddPeer(t *testing.T) {
	peerA, peerIDA := simplePeer(t, "aa")

	rts, teardown := setup(t, []p2p.Peer{peerA})

	t.Cleanup(func() {
		teardown()
	})

	rts.shim.AddPeer(peerA)

	peerUpdate := <-rts.shim.PeerUpdateCh
	require.Equal(t, peerIDA, peerUpdate.PeerID)
	require.Equal(t, p2p.PeerStatusUp, peerUpdate.Status)
}

func TestReactorShim_RemovePeer(t *testing.T) {
	peerA, peerIDA := simplePeer(t, "aa")

	rts, teardown := setup(t, []p2p.Peer{peerA})

	t.Cleanup(func() {
		teardown()
	})

	rts.shim.RemovePeer(peerA, "test reason")

	peerUpdate := <-rts.shim.PeerUpdateCh
	require.Equal(t, peerIDA, peerUpdate.PeerID)
	require.Equal(t, p2p.PeerStatusDown, peerUpdate.Status)
}

func simplePeer(t *testing.T, id string) (*p2pmocks.Peer, p2p.PeerID) {
	t.Helper()

	peer := &p2pmocks.Peer{}
	peer.On("ID").Return(p2p.ID(id))

	pID, err := p2p.PeerIDFromString(string(peer.ID()))
	require.NoError(t, err)

	return peer, pID
}

func TestReactorShim_Receive(t *testing.T) {
	peerA, peerIDA := simplePeer(t, "aa")

	rts, teardown := setup(t, []p2p.Peer{peerA})

	t.Cleanup(func() {
		teardown()
	})

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

	wg.Add(1)
	rts.shim.Receive(channelID1, peerA, bz)

	// Simulate receiving the envelope in some real reactor and replying back with
	// the same envelope.
	p2pCh := rts.shim.Channels[p2p.ChannelID(channelID1)]
	e := <-p2pCh.InCh
	require.Equal(t, peerIDA, e.From)
	require.NotNil(t, e.Message)
	p2pCh.OutCh <- p2p.Envelope{To: e.From, Message: e.Message}

	// wait until the mock peer called Send and we (fake) proxied the envelope
	wg.Wait()
	require.NotNil(t, response)

	m, err := response.Unwrap()
	require.NoError(t, err)
	require.Equal(t, msg.GetChunkRequest(), m)

	peerA.AssertExpectations(t)
}
