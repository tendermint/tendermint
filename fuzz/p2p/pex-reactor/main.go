package pex

import (
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tmlibs/log"
)

func Fuzz(data []byte) int {
	addrB := p2p.NewAddrBook("./testdata/addrbook1", false)
	prec := p2p.NewPEXReactor(addrB)
	if prec == nil {
		panic("nil Reactor")
	}
	peer1 := newFuzzPeer()
	prec.AddPeer(peer1)
	prec.Receive(0x01, peer1, data)
	return 1
}

type fuzzPeer struct {
	m map[string]interface{}
}

var _ p2p.Peer = (*fuzzPeer)(nil)

func newFuzzPeer() *fuzzPeer {
	return &fuzzPeer{m: make(map[string]interface{})}
}

func (fp *fuzzPeer) Key() string {
	return "fuzz-peer"
}

func (fp *fuzzPeer) Send(ch byte, data interface{}) bool {
	return true
}

func (fp *fuzzPeer) TrySend(ch byte, data interface{}) bool {
	return true
}

func (fp *fuzzPeer) Set(key string, value interface{}) {
	fp.m[key] = value
}

func (fp *fuzzPeer) Get(key string) interface{} {
	return fp.m[key]
}

var ed25519Key = crypto.GenPrivKeyEd25519()
var defaultNodeInfo = &p2p.NodeInfo{
	PubKey:     ed25519Key.PubKey().Unwrap().(crypto.PubKeyEd25519),
	Moniker:    "foo1",
	Version:    "tender-fuzz",
	RemoteAddr: "0.0.0.0:98991",
	ListenAddr: "0.0.0.0:98992",
}

func (fp *fuzzPeer) IsOutbound() bool             { return false }
func (fp *fuzzPeer) IsPersistent() bool           { return false }
func (fp *fuzzPeer) IsRunning() bool              { return false }
func (fp *fuzzPeer) NodeInfo() *p2p.NodeInfo      { return defaultNodeInfo }
func (fp *fuzzPeer) OnReset() error               { return nil }
func (fp *fuzzPeer) OnStart() error               { return nil }
func (fp *fuzzPeer) OnStop()                      {}
func (fp *fuzzPeer) Reset() (bool, error)         { return true, nil }
func (fp *fuzzPeer) Start() (bool, error)         { return true, nil }
func (fp *fuzzPeer) Stop() bool                   { return true }
func (fp *fuzzPeer) String() string               { return fp.Key() }
func (fp *fuzzPeer) Status() p2p.ConnectionStatus { var cs p2p.ConnectionStatus; return cs }
func (fp *fuzzPeer) SetLogger(l log.Logger)       {}
