package p2p

import (
	"fmt"
	"io"
	"net"
	"sync/atomic"

	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/types"
)

type Peer struct {
	outbound bool
	mconn    *MConnection
	running  uint32

	*types.NodeInfo
	Key  string
	Data *CMap // User data.
}

// NOTE: blocking
// Before creating a peer with newPeer(), perform a handshake on connection.
func peerHandshake(conn net.Conn, ourNodeInfo *types.NodeInfo) (*types.NodeInfo, error) {
	var peerNodeInfo = new(types.NodeInfo)
	var err1 error
	var err2 error
	Parallel(
		func() {
			var n int64
			binary.WriteBinary(ourNodeInfo, conn, &n, &err1)
		},
		func() {
			var n int64
			binary.ReadBinary(peerNodeInfo, conn, &n, &err2)
			log.Info("Peer handshake", "peerNodeInfo", peerNodeInfo)
		})
	if err1 != nil {
		return nil, err1
	}
	if err2 != nil {
		return nil, err2
	}
	return peerNodeInfo, nil
}

// NOTE: call peerHandshake on conn before calling newPeer().
func newPeer(conn net.Conn, peerNodeInfo *types.NodeInfo, outbound bool, reactorsByCh map[byte]Reactor, chDescs []*ChannelDescriptor, onPeerError func(*Peer, interface{})) *Peer {
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
		NodeInfo: peerNodeInfo,
		Key:      peerNodeInfo.PubKey.KeyString(),
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
