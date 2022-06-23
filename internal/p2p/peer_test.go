package p2p

import (
	"context"
	"fmt"
	golog "log"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"

	"github.com/tendermint/tendermint/config"
	tmconn "github.com/tendermint/tendermint/internal/p2p/conn"
)

func TestPeerBasic(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// simulate remote peer
	rp := &remotePeer{PrivKey: ed25519.GenPrivKey(), Config: cfg}
	rp.Start()
	t.Cleanup(rp.Stop)

	p, err := createOutboundPeerAndPerformHandshake(rp.Addr(), cfg, tmconn.DefaultMConnConfig())
	require.Nil(err)

	err = p.Start()
	require.Nil(err)
	t.Cleanup(func() {
		if err := p.Stop(); err != nil {
			t.Error(err)
		}
	})

	assert.True(p.IsRunning())
	assert.True(p.IsOutbound())
	assert.False(p.IsPersistent())
	p.persistent = true
	assert.True(p.IsPersistent())
	assert.Equal(rp.Addr().DialString(), p.RemoteAddr().String())
	assert.Equal(rp.ID(), p.ID())
}

func TestPeerSend(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	config := cfg

	// simulate remote peer
	rp := &remotePeer{PrivKey: ed25519.GenPrivKey(), Config: config}
	rp.Start()
	t.Cleanup(rp.Stop)

	p, err := createOutboundPeerAndPerformHandshake(rp.Addr(), config, tmconn.DefaultMConnConfig())
	require.Nil(err)

	err = p.Start()
	require.Nil(err)

	t.Cleanup(func() {
		if err := p.Stop(); err != nil {
			t.Error(err)
		}
	})

	assert.True(p.Send(testCh, []byte("Asylum")))
}

func createOutboundPeerAndPerformHandshake(
	addr *NetAddress,
	config *config.P2PConfig,
	mConfig tmconn.MConnConfig,
) (*peer, error) {
	chDescs := []*tmconn.ChannelDescriptor{
		{ID: testCh, Priority: 1},
	}
	pk := ed25519.GenPrivKey()
	ourNodeInfo := testNodeInfo(types.NodeIDFromPubKey(pk.PubKey()), "host_peer")
	transport := NewMConnTransport(log.TestingLogger(), mConfig, chDescs, MConnTransportOptions{})
	reactorsByCh := map[byte]Reactor{testCh: NewTestReactor(chDescs, true)}
	pc, err := testOutboundPeerConn(transport, addr, config, false, pk)
	if err != nil {
		return nil, err
	}
	peerInfo, _, err := pc.conn.Handshake(context.Background(), 0, ourNodeInfo, pk)
	if err != nil {
		return nil, err
	}

	p := newPeer(peerInfo, pc, reactorsByCh, func(p Peer, r interface{}) {})
	p.SetLogger(log.TestingLogger().With("peer", addr))
	return p, nil
}

func testDial(addr *NetAddress, cfg *config.P2PConfig) (net.Conn, error) {
	if cfg.TestDialFail {
		return nil, fmt.Errorf("dial err (peerConfig.DialFail == true)")
	}

	conn, err := addr.DialTimeout(cfg.DialTimeout)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func testOutboundPeerConn(
	transport *MConnTransport,
	addr *NetAddress,
	config *config.P2PConfig,
	persistent bool,
	ourNodePrivKey crypto.PrivKey,
) (peerConn, error) {

	var pc peerConn
	conn, err := testDial(addr, config)
	if err != nil {
		return pc, fmt.Errorf("error creating peer: %w", err)
	}

	pc, err = testPeerConn(transport, conn, true, persistent)
	if err != nil {
		if cerr := conn.Close(); cerr != nil {
			return pc, fmt.Errorf("%v: %w", cerr.Error(), err)
		}
		return pc, err
	}

	return pc, nil
}

type remotePeer struct {
	PrivKey    crypto.PrivKey
	Config     *config.P2PConfig
	Network    string
	addr       *NetAddress
	channels   bytes.HexBytes
	listenAddr string
	listener   net.Listener
}

func (rp *remotePeer) Addr() *NetAddress {
	return rp.addr
}

func (rp *remotePeer) ID() types.NodeID {
	return types.NodeIDFromPubKey(rp.PrivKey.PubKey())
}

func (rp *remotePeer) Start() {
	if rp.listenAddr == "" {
		rp.listenAddr = "127.0.0.1:0"
	}

	l, e := net.Listen("tcp", rp.listenAddr) // any available address
	if e != nil {
		golog.Fatalf("net.Listen tcp :0: %+v", e)
	}
	rp.listener = l
	rp.addr = types.NewNetAddress(types.NodeIDFromPubKey(rp.PrivKey.PubKey()), l.Addr())
	if rp.channels == nil {
		rp.channels = []byte{testCh}
	}
	go rp.accept()
}

func (rp *remotePeer) Stop() {
	rp.listener.Close()
}

func (rp *remotePeer) Dial(addr *NetAddress) (net.Conn, error) {
	transport := NewMConnTransport(log.TestingLogger(), MConnConfig(rp.Config),
		[]*ChannelDescriptor{}, MConnTransportOptions{})
	conn, err := addr.DialTimeout(1 * time.Second)
	if err != nil {
		return nil, err
	}
	pc, err := testInboundPeerConn(transport, conn)
	if err != nil {
		return nil, err
	}
	_, _, err = pc.conn.Handshake(context.Background(), 0, rp.nodeInfo(), rp.PrivKey)
	if err != nil {
		return nil, err
	}
	return conn, err
}

func (rp *remotePeer) accept() {
	transport := NewMConnTransport(log.TestingLogger(), MConnConfig(rp.Config),
		[]*ChannelDescriptor{}, MConnTransportOptions{})
	conns := []net.Conn{}

	for {
		conn, err := rp.listener.Accept()
		if err != nil {
			golog.Printf("Failed to accept conn: %+v", err)
			for _, conn := range conns {
				_ = conn.Close()
			}
			return
		}

		pc, err := testInboundPeerConn(transport, conn)
		if err != nil {
			golog.Printf("Failed to create a peer: %+v", err)
		}
		_, _, err = pc.conn.Handshake(context.Background(), 0, rp.nodeInfo(), rp.PrivKey)
		if err != nil {
			golog.Printf("Failed to handshake a peer: %+v", err)
		}

		conns = append(conns, conn)
	}
}

func (rp *remotePeer) nodeInfo() types.NodeInfo {
	ni := types.NodeInfo{
		ProtocolVersion: defaultProtocolVersion,
		NodeID:          rp.Addr().ID,
		ListenAddr:      rp.listener.Addr().String(),
		Network:         "testing",
		Version:         "1.2.3-rc0-deadbeef",
		Channels:        rp.channels,
		Moniker:         "remote_peer",
	}
	if rp.Network != "" {
		ni.Network = rp.Network
	}
	return ni
}
