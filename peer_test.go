package p2p

import (
	golog "log"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	crypto "github.com/tendermint/go-crypto"
)

func TestPeerStartStop(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// simulate remote peer
	rp := &remotePeer{PrivKey: crypto.GenPrivKeyEd25519()}
	rp.Start()
	defer rp.Stop()

	p, err := createPeerAndPerformHandshake(rp.RemoteAddr())
	require.Nil(err)

	p.Start()
	defer p.Stop()

	assert.True(p.IsRunning())
}

func createPeerAndPerformHandshake(addr *NetAddress) (*Peer, error) {
	chDescs := []*ChannelDescriptor{
		&ChannelDescriptor{ID: 0x01, Priority: 1},
	}
	reactorsByCh := map[byte]Reactor{0x01: NewTestReactor(chDescs, true)}
	pk := crypto.GenPrivKeyEd25519()
	p, err := newPeer(addr, reactorsByCh, chDescs, func(p *Peer, r interface{}) {}, pk)
	if err != nil {
		return nil, err
	}
	err = p.HandshakeTimeout(&NodeInfo{
		PubKey:  pk.PubKey().(crypto.PubKeyEd25519),
		Moniker: "host_peer",
		Network: "testing",
		Version: "123.123.123",
	}, 1*time.Second)
	if err != nil {
		return nil, err
	}
	return p, nil
}

type remotePeer struct {
	PrivKey crypto.PrivKeyEd25519
	addr    *NetAddress
	quit    chan struct{}
}

func (p *remotePeer) RemoteAddr() *NetAddress {
	return p.addr
}

func (p *remotePeer) Start() {
	l, e := net.Listen("tcp", "127.0.0.1:0") // any available address
	if e != nil {
		golog.Fatalf("net.Listen tcp :0: %+v", e)
	}
	p.addr = NewNetAddress(l.Addr())
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
		peer, err := newPeerFromExistingConn(conn, false, make(map[byte]Reactor), make([]*ChannelDescriptor, 0), func(p *Peer, r interface{}) {}, p.PrivKey)
		if err != nil {
			golog.Fatalf("Failed to create a peer: %+v", err)
		}
		err = peer.HandshakeTimeout(&NodeInfo{
			PubKey:  p.PrivKey.PubKey().(crypto.PubKeyEd25519),
			Moniker: "remote_peer",
			Network: "testing",
			Version: "123.123.123",
		}, 1*time.Second)
		if err != nil {
			golog.Fatalf("Failed to perform handshake: %+v", err)
		}
		select {
		case <-p.quit:
			conn.Close()
			return
		default:
		}
	}
}
