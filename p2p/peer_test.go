package p2p

import (
	golog "log"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	crypto "github.com/tendermint/go-crypto"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"

	"github.com/tendermint/tendermint/config"
	tmconn "github.com/tendermint/tendermint/p2p/conn"
)

const testCh = 0x01

func TestPeerBasic(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// simulate remote peer
	rp := &remotePeer{PrivKey: crypto.GenPrivKeyEd25519(), Config: cfg}
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
	rp := &remotePeer{PrivKey: crypto.GenPrivKeyEd25519(), Config: config}
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
	var (
		chDescs = []*tmconn.ChannelDescriptor{
			{ID: testCh, Priority: 1},
		}
		conf = peerConfig{
			chDescs: chDescs,
			mConfig: tmconn.DefaultMConnConfig(),
			nodeInfo: NodeInfo{
				ID:       addr.ID,
				Moniker:  "host_peer",
				Network:  "testing",
				Version:  "123.123.123",
				Channels: []byte{testCh},
			},
			nodeKey:      NodeKey{PrivKey: crypto.GenPrivKeyEd25519()},
			onPeerError:  func(p Peer, r interface{}) {},
			p2pConfig:    *cfg,
			reactorsByCh: map[byte]Reactor{testCh: NewTestReactor(chDescs, true)},
		}
		timeout = 100 * time.Millisecond
	)

	c, err := addr.DialTimeout(timeout)
	if err != nil {
		return nil, err
	}

	p, err := upgrade(c, timeout, conf)
	if err != nil {
		return nil, err
	}
	p.SetLogger(log.TestingLogger().With("peer", addr))

	return p, nil
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
		c, err := l.Accept()
		if err != nil {
			golog.Fatalf("Failed to accept conn: %+v", err)
		}

		_, err = upgrade(c, 100*time.Millisecond, peerConfig{
			mConfig: tmconn.DefaultMConnConfig(),
			nodeInfo: NodeInfo{
				ID:         rp.Addr().ID,
				Moniker:    "remote_peer",
				Network:    "testing",
				Version:    "123.123.123",
				ListenAddr: l.Addr().String(),
				Channels:   rp.channels,
			},
			nodeKey: NodeKey{
				PrivKey: rp.PrivKey,
			},
		})

		conns = append(conns, c)

		select {
		case <-rp.quit:
			for _, c := range conns {
				if err := c.Close(); err != nil {
					golog.Fatal(err)
				}
			}
			return
		default:
		}
	}
}
