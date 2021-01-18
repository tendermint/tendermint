package pex

import (
	"net"

	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/pex"
	"github.com/tendermint/tendermint/version"
)

var addrB = pex.NewAddrBook("./testdata/addrbook1", false)

func Fuzz(data []byte) int {
	pexR := pex.NewReactor(addrB, &pex.ReactorConfig{SeedMode: false})
	if pexR == nil {
		panic("nil Reactor")
	}

	peer := newFuzzPeer()
	pexR.AddPeer(peer)
	pexR.Receive(pex.PexChannel, peer, data)
	return 1
}

type fuzzPeer struct {
	*service.BaseService
	m map[string]interface{}
}

var _ p2p.Peer = (*fuzzPeer)(nil)

func newFuzzPeer() *fuzzPeer {
	fp := &fuzzPeer{m: make(map[string]interface{})}
	fp.BaseService = service.NewBaseService(nil, "fuzzPeer", fp)
	return fp
}

var privKey = ed25519.GenPrivKey()
var nodeID = p2p.NodeIDFromPubKey(privKey.PubKey())
var defaultNodeInfo = p2p.NodeInfo{
	ProtocolVersion: p2p.NewProtocolVersion(
		version.P2PProtocol,
		version.BlockProtocol,
		0,
	),
	NodeID:     nodeID,
	ListenAddr: "0.0.0.0:98992",
	Moniker:    "foo1",
}

func (fp *fuzzPeer) FlushStop()       {}
func (fp *fuzzPeer) ID() p2p.NodeID   { return nodeID }
func (fp *fuzzPeer) RemoteIP() net.IP { return net.IPv4(0, 0, 0, 0) }
func (fp *fuzzPeer) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: fp.RemoteIP(), Port: 98991, Zone: ""}
}
func (fp *fuzzPeer) IsOutbound() bool                  { return false }
func (fp *fuzzPeer) IsPersistent() bool                { return false }
func (fp *fuzzPeer) CloseConn() error                  { return nil }
func (fp *fuzzPeer) NodeInfo() p2p.NodeInfo            { return defaultNodeInfo }
func (fp *fuzzPeer) Status() p2p.ConnectionStatus      { var cs p2p.ConnectionStatus; return cs }
func (fp *fuzzPeer) SocketAddr() *p2p.NetAddress       { return p2p.NewNetAddress(fp.ID(), fp.RemoteAddr()) }
func (fp *fuzzPeer) Send(byte, []byte) bool            { return true }
func (fp *fuzzPeer) TrySend(byte, []byte) bool         { return true }
func (fp *fuzzPeer) Set(key string, value interface{}) { fp.m[key] = value }
func (fp *fuzzPeer) Get(key string) interface{}        { return fp.m[key] }
