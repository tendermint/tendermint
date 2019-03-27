package mock

import (
	"net"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/conn"
)

type Peer struct {
	*cmn.BaseService
	ip                   net.IP
	id                   p2p.ID
	addr                 *p2p.NetAddress
	pubKey               crypto.PubKey
	outbound, persistent bool
}

// NewPeer creates and starts a new mock peer. If the ip
// is nil, random routable address is used.
func NewPeer(ip net.IP) *Peer {
	var netAddr *p2p.NetAddress
	if ip == nil {
		_, netAddr = p2p.CreateRoutableAddr()
	} else {
		netAddr = p2p.NewNetAddressIPPort(ip, 26656)
	}
	nodeKey := p2p.NodeKey{PrivKey: ed25519.GenPrivKey()}
	netAddr.ID = nodeKey.ID()
	mp := &Peer{
		ip:   ip,
		id:   nodeKey.ID(),
		addr: netAddr,
	}
	mp.BaseService = cmn.NewBaseService(nil, "MockPeer", mp)
	mp.Start()
	return mp
}

func (mp *Peer) FlushStop()                              { mp.Stop() }
func (mp *Peer) TrySend(chID byte, msgBytes []byte) bool { return true }
func (mp *Peer) Send(chID byte, msgBytes []byte) bool    { return true }
func (mp *Peer) NodeInfo() p2p.NodeInfo {
	return p2p.DefaultNodeInfo{
		ID_:        mp.addr.ID,
		ListenAddr: mp.addr.DialString(),
	}
}
func (mp *Peer) Status() conn.ConnectionStatus { return conn.ConnectionStatus{} }
func (mp *Peer) ID() p2p.ID                    { return mp.id }
func (mp *Peer) IsOutbound() bool              { return false }
func (mp *Peer) IsPersistent() bool            { return true }
func (mp *Peer) Get(s string) interface{}      { return nil }
func (mp *Peer) Set(string, interface{})       {}
func (mp *Peer) RemoteIP() net.IP              { return mp.ip }
func (mp *Peer) OriginalAddr() *p2p.NetAddress { return nil }
func (mp *Peer) RemoteAddr() net.Addr          { return &net.TCPAddr{IP: mp.ip, Port: 8800} }
func (mp *Peer) CloseConn() error              { return nil }
