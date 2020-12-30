package mock

import (
	"net"

	"github.com/tendermint/tendermint/crypto/ed25519"

	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/conn"
)

type Peer struct {
	*service.BaseService
	ip                   net.IP
	id                   p2p.ID
	addr                 *p2p.NetAddress
	kv                   map[string]interface{}
	Outbound, Persistent bool
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
		kv:   make(map[string]interface{}),
	}
	mp.BaseService = service.NewBaseService(nil, "MockPeer", mp)
	if err := mp.Start(); err != nil {
		panic(err)
	}
	return mp
}

func (mp *Peer) FlushStop()                              { mp.Stop() } //nolint:errcheck //ignore error
func (mp *Peer) TrySend(chID byte, msgBytes []byte) bool { return true }
func (mp *Peer) Send(chID byte, msgBytes []byte) bool    { return true }
func (mp *Peer) NodeInfo() p2p.NodeInfo {
	return p2p.DefaultNodeInfo{
		DefaultNodeID: mp.addr.ID,
		ListenAddr:    mp.addr.DialString(),
	}
}
func (mp *Peer) Status() conn.ConnectionStatus { return conn.ConnectionStatus{} }
func (mp *Peer) ID() p2p.ID                    { return mp.id }
func (mp *Peer) IsOutbound() bool              { return mp.Outbound }
func (mp *Peer) IsPersistent() bool            { return mp.Persistent }
func (mp *Peer) Get(key string) interface{} {
	if value, ok := mp.kv[key]; ok {
		return value
	}
	return nil
}
func (mp *Peer) Set(key string, value interface{}) {
	mp.kv[key] = value
}
func (mp *Peer) RemoteIP() net.IP            { return mp.ip }
func (mp *Peer) SocketAddr() *p2p.NetAddress { return mp.addr }
func (mp *Peer) RemoteAddr() net.Addr        { return &net.TCPAddr{IP: mp.ip, Port: 8800} }
func (mp *Peer) CloseConn() error            { return nil }
