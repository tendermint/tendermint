package pex

import (
	"net"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/p2p/pex"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

var (
	pexR   *pex.Reactor
	peer   p2p.Peer
	logger = log.MustNewDefaultLogger(log.LogFormatPlain, log.LogLevelInfo, false)
)

func init() {
	addrB := pex.NewAddrBook("./testdata/addrbook1", false)
	pexR = pex.NewReactor(addrB, &pex.ReactorConfig{SeedMode: false})
	pexR.SetLogger(logger)
	peer = newFuzzPeer()
	pexR.AddPeer(peer)

	cfg := config.DefaultP2PConfig()
	cfg.PexReactor = true
	sw := p2p.MakeSwitch(cfg, 0, "127.0.0.1", "123.123.123", func(i int, sw *p2p.Switch) *p2p.Switch {
		return sw
	}, logger)
	pexR.SetSwitch(sw)
}

func Fuzz(data []byte) int {
	if len(data) == 0 {
		return -1
	}

	pexR.Receive(pex.PexChannel, peer, data)

	if !peer.IsRunning() {
		// do not increase priority for msgs which lead to peer being stopped
		return 0
	}

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
var nodeID = types.NodeIDFromPubKey(privKey.PubKey())
var defaultNodeInfo = types.NodeInfo{
	ProtocolVersion: types.ProtocolVersion{
		P2P:   version.P2PProtocol,
		Block: version.BlockProtocol,
		App:   0,
	},
	NodeID:     nodeID,
	ListenAddr: "127.0.0.1:0",
	Moniker:    "foo1",
}

func (fp *fuzzPeer) FlushStop()       {}
func (fp *fuzzPeer) ID() types.NodeID { return nodeID }
func (fp *fuzzPeer) RemoteIP() net.IP { return net.IPv4(198, 163, 190, 214) }
func (fp *fuzzPeer) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: fp.RemoteIP(), Port: 26656, Zone: ""}
}
func (fp *fuzzPeer) IsOutbound() bool             { return false }
func (fp *fuzzPeer) IsPersistent() bool           { return false }
func (fp *fuzzPeer) CloseConn() error             { return nil }
func (fp *fuzzPeer) NodeInfo() types.NodeInfo     { return defaultNodeInfo }
func (fp *fuzzPeer) Status() p2p.ConnectionStatus { var cs p2p.ConnectionStatus; return cs }
func (fp *fuzzPeer) SocketAddr() *p2p.NetAddress {
	return types.NewNetAddress(fp.ID(), fp.RemoteAddr())
}
func (fp *fuzzPeer) Send(byte, []byte) bool            { return true }
func (fp *fuzzPeer) TrySend(byte, []byte) bool         { return true }
func (fp *fuzzPeer) Set(key string, value interface{}) { fp.m[key] = value }
func (fp *fuzzPeer) Get(key string) interface{}        { return fp.m[key] }
