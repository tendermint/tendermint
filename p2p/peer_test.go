package p2p

import (
	golog "log"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	crypto "github.com/tendermint/go-crypto"
	tmconn "github.com/tendermint/tendermint/p2p/conn"
)

const testCh = 0x01

func TestPeerBasic(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// simulate remote peer
	rp := &remotePeer{PrivKey: crypto.GenPrivKeyEd25519(), Config: DefaultPeerConfig()}
	rp.Start()
	defer rp.Stop()

	p, err := createOutboundPeerAndPerformHandshake(rp.Addr(), DefaultPeerConfig())
	require.Nil(err)

	err = p.Start()
	require.Nil(err)
	defer p.Stop()

	assert.True(p.IsRunning())
	assert.True(p.IsOutbound())
	assert.False(p.IsPersistent())
	p.persistent = true
	assert.True(p.IsPersistent())
	assert.Equal(rp.Addr().String(), p.Addr().String())
	assert.Equal(rp.PubKey(), p.PubKey())
}

func TestPeerWithoutAuthEnc(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	config := DefaultPeerConfig()
	config.AuthEnc = false

	// simulate remote peer
	rp := &remotePeer{PrivKey: crypto.GenPrivKeyEd25519(), Config: config}
	rp.Start()
	defer rp.Stop()

	p, err := createOutboundPeerAndPerformHandshake(rp.Addr(), config)
	require.Nil(err)

	err = p.Start()
	require.Nil(err)
	defer p.Stop()

	assert.True(p.IsRunning())
}

func TestPeerSend(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	config := DefaultPeerConfig()
	config.AuthEnc = false

	// simulate remote peer
	rp := &remotePeer{PrivKey: crypto.GenPrivKeyEd25519(), Config: config}
	rp.Start()
	defer rp.Stop()

	p, err := createOutboundPeerAndPerformHandshake(rp.Addr(), config)
	require.Nil(err)

	err = p.Start()
	require.Nil(err)

	defer p.Stop()

	assert.True(p.CanSend(testCh))
	assert.True(p.Send(testCh, "Asylum"))
}

func createOutboundPeerAndPerformHandshake(addr *NetAddress, config *PeerConfig) (*peer, error) {
	chDescs := []*tmconn.ChannelDescriptor{
		{ID: testCh, Priority: 1},
	}
	reactorsByCh := map[byte]Reactor{testCh: NewTestReactor(chDescs, true)}
	pk := crypto.GenPrivKeyEd25519()
	p, err := newOutboundPeer(addr, reactorsByCh, chDescs, func(p Peer, r interface{}) {}, pk, config, false)
	if err != nil {
		return nil, err
	}
	err = p.HandshakeTimeout(NodeInfo{
		PubKey:   pk.PubKey(),
		Moniker:  "host_peer",
		Network:  "testing",
		Version:  "123.123.123",
		Channels: []byte{testCh},
	}, 1*time.Second)
	if err != nil {
		return nil, err
	}
	return p, nil
}

type remotePeer struct {
	PrivKey crypto.PrivKeyEd25519
	Config  *PeerConfig
	addr    *NetAddress
	quit    chan struct{}
}

func (p *remotePeer) Addr() *NetAddress {
	return p.addr
}

func (p *remotePeer) PubKey() crypto.PubKey {
	return p.PrivKey.PubKey()
}

func (p *remotePeer) Start() {
	l, e := net.Listen("tcp", "127.0.0.1:0") // any available address
	if e != nil {
		golog.Fatalf("net.Listen tcp :0: %+v", e)
	}
	p.addr = NewNetAddress("", l.Addr())
	p.quit = make(chan struct{})
	go p.accept(l)
}

func (p *remotePeer) Stop() {
	close(p.quit)
}

func (p *remotePeer) accept(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			golog.Fatalf("Failed to accept conn: %+v", err)
		}
		peer, err := newInboundPeer(conn, make(map[byte]Reactor), make([]*tmconn.ChannelDescriptor, 0), func(p Peer, r interface{}) {}, p.PrivKey, p.Config)
		if err != nil {
			golog.Fatalf("Failed to create a peer: %+v", err)
		}
		err = peer.HandshakeTimeout(NodeInfo{
			PubKey:     p.PrivKey.PubKey(),
			Moniker:    "remote_peer",
			Network:    "testing",
			Version:    "123.123.123",
			ListenAddr: l.Addr().String(),
			Channels:   []byte{testCh},
		}, 1*time.Second)
		if err != nil {
			golog.Fatalf("Failed to perform handshake: %+v", err)
		}
		select {
		case <-p.quit:
			if err := conn.Close(); err != nil {
				golog.Fatal(err)
			}
			return
		default:
		}
	}
}
