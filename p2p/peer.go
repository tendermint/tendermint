package p2p

import (
	"fmt"
	"io"
	"net"
	"sync/atomic"

	. "github.com/tendermint/tendermint/binary"
)

/* Peer */

type Peer struct {
	outbound bool
	mconn    *MConnection
	started  uint32
	stopped  uint32

	Key string
}

func newPeer(conn net.Conn, outbound bool, chDescs []*ChannelDescriptor, onPeerError func(*Peer, interface{})) *Peer {
	var p *Peer
	onError := func(r interface{}) {
		p.stop()
		onPeerError(p, r)
	}
	mconn := NewMConnection(conn, chDescs, onError)
	p = &Peer{
		outbound: outbound,
		mconn:    mconn,
		stopped:  0,
		Key:      mconn.RemoteAddress.String(),
	}
	mconn.Peer = p // hacky optimization
	return p
}

func (p *Peer) start() {
	if atomic.CompareAndSwapUint32(&p.started, 0, 1) {
		log.Debug("Starting %v", p)
		p.mconn.Start()
	}
}

func (p *Peer) stop() {
	if atomic.CompareAndSwapUint32(&p.stopped, 0, 1) {
		log.Debug("Stopping %v", p)
		p.mconn.Stop()
	}
}

func (p *Peer) IsOutbound() bool {
	return p.outbound
}

func (p *Peer) Send(chId byte, msg Binary) bool {
	if atomic.LoadUint32(&p.stopped) == 1 {
		return false
	}
	return p.mconn.Send(chId, msg)
}

func (p *Peer) TrySend(chId byte, msg Binary) bool {
	if atomic.LoadUint32(&p.stopped) == 1 {
		return false
	}
	return p.mconn.TrySend(chId, msg)
}

func (p *Peer) CanSend(chId byte) bool {
	if atomic.LoadUint32(&p.stopped) == 1 {
		return false
	}
	return p.mconn.CanSend(chId)
}

func (p *Peer) WriteTo(w io.Writer) (n int64, err error) {
	return String(p.Key).WriteTo(w)
}

func (p *Peer) String() string {
	if p.outbound {
		return fmt.Sprintf("P(->%v)", p.mconn)
	} else {
		return fmt.Sprintf("P(%v->)", p.mconn)
	}
}

func (p *Peer) Equals(other *Peer) bool {
	return p.Key == other.Key
}
