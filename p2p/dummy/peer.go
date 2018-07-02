package dummy

import (
	"net"

	p2p "github.com/tendermint/tendermint/p2p"
	tmconn "github.com/tendermint/tendermint/p2p/conn"
	cmn "github.com/tendermint/tendermint/libs/common"
)

type peer struct {
	cmn.BaseService
	kv map[string]interface{}
}

var _ p2p.Peer = (*peer)(nil)

// NewPeer creates new dummy peer.
func NewPeer() *peer {
	p := &peer{
		kv: make(map[string]interface{}),
	}
	p.BaseService = *cmn.NewBaseService(nil, "peer", p)

	return p
}

// ID always returns dummy.
func (p *peer) ID() p2p.ID {
	return p2p.ID("dummy")
}

// IsOutbound always returns false.
func (p *peer) IsOutbound() bool {
	return false
}

// IsPersistent always returns false.
func (p *peer) IsPersistent() bool {
	return false
}

// NodeInfo always returns empty node info.
func (p *peer) NodeInfo() p2p.NodeInfo {
	return p2p.NodeInfo{}
}

// RemoteIP always returns localhost.
func (p *peer) RemoteIP() net.IP {
	return net.ParseIP("127.0.0.1")
}

// Status always returns empry connection status.
func (p *peer) Status() tmconn.ConnectionStatus {
	return tmconn.ConnectionStatus{}
}

// Send does not do anything and just returns true.
func (p *peer) Send(byte, []byte) bool {
	return true
}

// TrySend does not do anything and just returns true.
func (p *peer) TrySend(byte, []byte) bool {
	return true
}

// Set records value under key specified in the map.
func (p *peer) Set(key string, value interface{}) {
	p.kv[key] = value
}

// Get returns a value associated with the key. Nil is returned if no value
// found.
func (p *peer) Get(key string) interface{} {
	if value, ok := p.kv[key]; ok {
		return value
	}
	return nil
}
