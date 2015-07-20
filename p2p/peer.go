package p2p

import (
	"fmt"
	"io"
	"net"

	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/types"
)

type Peer struct {
	BaseService

	outbound bool
	mconn    *MConnection

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
			log.Notice("Peer handshake", "peerNodeInfo", peerNodeInfo)
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
		p.Stop()
		onPeerError(p, r)
	}
	mconn := NewMConnection(conn, chDescs, onReceive, onError)
	p = &Peer{
		outbound: outbound,
		mconn:    mconn,
		NodeInfo: peerNodeInfo,
		Key:      peerNodeInfo.PubKey.KeyString(),
		Data:     NewCMap(),
	}
	p.BaseService = *NewBaseService(log, "Peer", p)
	return p
}

func (p *Peer) AfterStart() {
	p.mconn.Start()
}

func (p *Peer) AfterStop() {
	p.mconn.Stop()
}

func (p *Peer) Connection() *MConnection {
	return p.mconn
}

func (p *Peer) IsOutbound() bool {
	return p.outbound
}

func (p *Peer) Send(chId byte, msg interface{}) bool {
	if !p.IsRunning() {
		return false
	}
	return p.mconn.Send(chId, msg)
}

func (p *Peer) TrySend(chId byte, msg interface{}) bool {
	if !p.IsRunning() {
		return false
	}
	return p.mconn.TrySend(chId, msg)
}

func (p *Peer) CanSend(chId byte) bool {
	if !p.IsRunning() {
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
