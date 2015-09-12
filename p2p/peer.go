package p2p

import (
	"fmt"
	"io"
	"net"

	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/wire"
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
			wire.WriteBinary(ourNodeInfo, conn, &n, &err1)
		},
		func() {
			var n int64
			wire.ReadBinary(peerNodeInfo, conn, &n, &err2)
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
	onReceive := func(chID byte, msgBytes []byte) {
		reactor := reactorsByCh[chID]
		if reactor == nil {
			PanicSanity(Fmt("Unknown channel %X", chID))
		}
		reactor.Receive(chID, p, msgBytes)
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

func (p *Peer) OnStart() error {
	p.BaseService.OnStart()
	_, err := p.mconn.Start()
	return err
}

func (p *Peer) OnStop() {
	p.BaseService.OnStop()
	p.mconn.Stop()
}

func (p *Peer) Connection() *MConnection {
	return p.mconn
}

func (p *Peer) IsOutbound() bool {
	return p.outbound
}

func (p *Peer) Send(chID byte, msg interface{}) bool {
	if !p.IsRunning() {
		return false
	}
	return p.mconn.Send(chID, msg)
}

func (p *Peer) TrySend(chID byte, msg interface{}) bool {
	if !p.IsRunning() {
		return false
	}
	return p.mconn.TrySend(chID, msg)
}

func (p *Peer) CanSend(chID byte) bool {
	if !p.IsRunning() {
		return false
	}
	return p.mconn.CanSend(chID)
}

func (p *Peer) WriteTo(w io.Writer) (n int64, err error) {
	wire.WriteString(p.Key, w, &n, &err)
	return
}

func (p *Peer) String() string {
	if p.outbound {
		return fmt.Sprintf("Peer{%v %v out}", p.mconn, p.Key[:12])
	} else {
		return fmt.Sprintf("Peer{%v %v in}", p.mconn, p.Key[:12])
	}
}

func (p *Peer) Equals(other *Peer) bool {
	return p.Key == other.Key
}
