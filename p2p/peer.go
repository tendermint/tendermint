package p2p

import (
	"fmt"
	"io"
	"net"
	"sync/atomic"

	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

type Peer struct {
	outbound bool
	mconn    *MConnection
	running  uint32

	Key  string
	Data *CMap // User data.
}

func newPeer(conn net.Conn, outbound bool, reactorsByCh map[byte]Reactor, chDescs []*ChannelDescriptor, onPeerError func(*Peer, interface{})) *Peer {
	var p *Peer
	onReceive := func(chId byte, msgBytes []byte) {
		reactor := reactorsByCh[chId]
		if reactor == nil {
			panic(Fmt("Unknown channel %X", chId))
		}
		reactor.Receive(chId, p, msgBytes)
	}
	onError := func(r interface{}) {
		p.stop()
		onPeerError(p, r)
	}
	mconn := NewMConnection(conn, chDescs, onReceive, onError)
	p = &Peer{
		outbound: outbound,
		mconn:    mconn,
		running:  0,
		Key:      mconn.RemoteAddress.String(),
		Data:     NewCMap(),
	}
	return p
}

func (p *Peer) start() {
	if atomic.CompareAndSwapUint32(&p.running, 0, 1) {
		log.Debug("Starting Peer", "peer", p)
		p.mconn.Start()
	}
}

func (p *Peer) stop() {
	if atomic.CompareAndSwapUint32(&p.running, 1, 0) {
		log.Debug("Stopping Peer", "peer", p)
		p.mconn.Stop()
	}
}

func (p *Peer) IsRunning() bool {
	return atomic.LoadUint32(&p.running) == 1
}

func (p *Peer) Connection() *MConnection {
	return p.mconn
}

func (p *Peer) IsOutbound() bool {
	return p.outbound
}

func (p *Peer) Send(chId byte, msg interface{}) bool {
	if atomic.LoadUint32(&p.running) == 0 {
		return false
	}
	return p.mconn.Send(chId, msg)
}

func (p *Peer) TrySend(chId byte, msg interface{}) bool {
	if atomic.LoadUint32(&p.running) == 0 {
		return false
	}
	return p.mconn.TrySend(chId, msg)
}

func (p *Peer) CanSend(chId byte) bool {
	if atomic.LoadUint32(&p.running) == 0 {
		return false
	}
	return p.mconn.CanSend(chId)
}

func (p *Peer) WriteTo(w io.Writer) (n int64, err error) {
	binary.WriteString(p.Key, w, &n, &err)
	return
}

func (p *Peer) String() string {
	if p.outbound {
		return fmt.Sprintf("Peer{->%v}", p.mconn)
	} else {
		return fmt.Sprintf("Peer{%v->}", p.mconn)
	}
}

func (p *Peer) Equals(other *Peer) bool {
	return p.Key == other.Key
}
