package p2p

import (
	"fmt"
	golog "log"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/tendermint/tendermint/config"
	tmconn "github.com/tendermint/tendermint/p2p/conn"
)

func TestPeerBasic(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// simulate remote peer
	rp := &remotePeer{PrivKey: ed25519.GenPrivKey(), Config: cfg}
	rp.Start()
	defer rp.Stop()

	p, err := createOutboundPeerAndPerformHandshake(rp.Addr(), cfg, tmconn.DefaultMConnConfig())
	require.Nil(err)

	err = p.Start()
	require.Nil(err)
	defer p.Stop()

	assert.True(p.IsRunning())
	assert.True(p.IsOutbound())
	assert.False(p.IsPersistent())
	p.persistent = true
	assert.True(p.IsPersistent())
	assert.Equal(rp.Addr().DialString(), p.Addr().String())
	assert.Equal(rp.ID(), p.ID())
}

func TestPeerSend(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	config := cfg

	// simulate remote peer
	rp := &remotePeer{PrivKey: ed25519.GenPrivKey(), Config: config}
	rp.Start()
	defer rp.Stop()

	p, err := createOutboundPeerAndPerformHandshake(rp.Addr(), config, tmconn.DefaultMConnConfig())
	require.Nil(err)

	err = p.Start()
	require.Nil(err)

	defer p.Stop()

	assert.True(p.CanSend(testCh))
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
	reactorsByCh := map[byte]Reactor{testCh: NewTestReactor(chDescs, true)}
	pk := ed25519.GenPrivKey()
	pc, err := testOutboundPeerConn(addr, config, false, pk)
	if err != nil {
		return nil, err
	}
	timeout := 1 * time.Second
	ourNodeInfo := testNodeInfo(addr.ID, "host_peer")
	peerNodeInfo, err := handshake(pc.conn, timeout, ourNodeInfo)
	if err != nil {
		return nil, err
	}

	p := newPeer(pc, mConfig, peerNodeInfo, reactorsByCh, chDescs, func(p Peer, r interface{}) {})
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
	addr *NetAddress,
	config *config.P2PConfig,
	persistent bool,
	ourNodePrivKey crypto.PrivKey,
) (peerConn, error) {
	conn, err := testDial(addr, config)
	if err != nil {
		return peerConn{}, cmn.ErrorWrap(err, "Error creating peer")
	}

	pc, err := testPeerConn(conn, config, true, persistent, ourNodePrivKey, addr)
	if err != nil {
		if cerr := conn.Close(); cerr != nil {
			return peerConn{}, cmn.ErrorWrap(err, cerr.Error())
		}
		return peerConn{}, err
	}

	// ensure dialed ID matches connection ID
	if addr.ID != pc.ID() {
		if cerr := conn.Close(); cerr != nil {
			return peerConn{}, cmn.ErrorWrap(err, cerr.Error())
		}
		return peerConn{}, ErrSwitchAuthenticationFailure{addr, pc.ID()}
	}

	return pc, nil
}

type remotePeer struct {
	PrivKey    crypto.PrivKey
	Config     *config.P2PConfig
	addr       *NetAddress
	quit       chan struct{}
	channels   cmn.HexBytes
	listenAddr string
}

func (rp *remotePeer) Addr() *NetAddress {
	return rp.addr
}

func (rp *remotePeer) ID() ID {
	return PubKeyToID(rp.PrivKey.PubKey())
}

func (rp *remotePeer) Start() {
	if rp.listenAddr == "" {
		rp.listenAddr = "127.0.0.1:0"
	}

	l, e := net.Listen("tcp", rp.listenAddr) // any available address
	if e != nil {
		golog.Fatalf("net.Listen tcp :0: %+v", e)
	}
	rp.addr = NewNetAddress(PubKeyToID(rp.PrivKey.PubKey()), l.Addr())
	rp.quit = make(chan struct{})
	if rp.channels == nil {
		rp.channels = []byte{testCh}
	}
	go rp.accept(l)
}

func (rp *remotePeer) Stop() {
	close(rp.quit)
}

func (rp *remotePeer) accept(l net.Listener) {
	conns := []net.Conn{}

	for {
		conn, err := l.Accept()
		if err != nil {
			golog.Fatalf("Failed to accept conn: %+v", err)
		}

		pc, err := testInboundPeerConn(conn, rp.Config, rp.PrivKey)
		if err != nil {
			golog.Fatalf("Failed to create a peer: %+v", err)
		}

		_, err = handshake(pc.conn, time.Second, rp.nodeInfo(l))
		if err != nil {
			golog.Fatalf("Failed to perform handshake: %+v", err)
		}

		conns = append(conns, conn)

		select {
		case <-rp.quit:
			for _, conn := range conns {
				if err := conn.Close(); err != nil {
					golog.Fatal(err)
				}
			}
			return
		default:
		}
	}
}

func (rp *remotePeer) nodeInfo(l net.Listener) NodeInfo {
	return DefaultNodeInfo{
		ProtocolVersion: defaultProtocolVersion,
		ID_:             rp.Addr().ID,
		ListenAddr:      l.Addr().String(),
		Network:         "testing",
		Version:         "1.2.3-rc0-deadbeef",
		Channels:        rp.channels,
		Moniker:         "remote_peer",
	}
}
