package p2p

import (
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/p2p/conn"
	"net"
	"testing"
)

type FakePeer struct {
	*cmn.BaseService
	addr *NetAddress
}

func NewFakePeer(ip net.IP) *FakePeer {
	return &FakePeer{
		addr: NewNetAddressIPPort(ip, 26656),
	}
}

func (fp *FakePeer) ID() ID { return "" }
func (fp *FakePeer) RemoteIP() net.IP { return fp.addr.IP }
func (fp *FakePeer) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: fp.addr.IP, Port: 8800}
}

func (fp *FakePeer) IsOutbound() bool   { return true }
func (fp *FakePeer) IsPersistent() bool { return true }
func (fp *FakePeer) IsRunning() bool    { return true }
func (fp *FakePeer) CloseConn() error   { return nil }
func (fp *FakePeer) NodeInfo() NodeInfo {
	return DefaultNodeInfo{
		ID_:        fp.addr.ID,
		ListenAddr: fp.addr.DialString(),
	}
}
func (fp *FakePeer) Status() conn.ConnectionStatus           { return conn.ConnectionStatus{} }
func (fp *FakePeer) SocketAddr() *NetAddress                 { return fp.addr }
func (fp *FakePeer) FlushStop()                              {}
func (fp *FakePeer) TrySend(chID byte, msgBytes []byte) bool { return true }
func (fp *FakePeer) Send(chID byte, msgBytes []byte) bool    { return true }

func (fp *FakePeer) Get(key string) interface{}    { return nil }
func (fp *FakePeer) Set(key string, v interface{}) {}

func TestStorePeerBehaviour(t *testing.T) {
	peer := NewFakePeer(net.IP{127, 0, 0, 1})
	pb := NewStorePeerBehaviour()
	pb.Errored(peer, ErrPeerUnknown)

	peerErrors := pb.GetPeerErrors()
	if peerErrors[peer][0] != ErrPeerUnknown {
		t.Errorf("Expected to have 1 PeerError")
	}

	pb.MarkPeerAsGood(peer)
	goodPeers := pb.GetGoodPeers()
	if !goodPeers[peer] {
		t.Errorf("Expected to find the peer marked as good")
	}
}
