package benchmarks

import (
	"testing"

	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-p2p"
	"github.com/tendermint/go-wire"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

func BenchmarkEncodeStatus(b *testing.B) {
	b.StopTimer()
	pubKey := crypto.GenPrivKeyEd25519().PubKey().(crypto.PubKeyEd25519)
	status := &ctypes.ResultStatus{
		NodeInfo: &p2p.NodeInfo{
			PubKey:     pubKey,
			Moniker:    "SOMENAME",
			Network:    "SOMENAME",
			RemoteAddr: "SOMEADDR",
			ListenAddr: "SOMEADDR",
			Version:    "SOMEVER",
			Other:      []string{"SOMESTRING", "OTHERSTRING"},
		},
		PubKey:            pubKey,
		LatestBlockHash:   []byte("SOMEBYTES"),
		LatestBlockHeight: 123,
		LatestBlockTime:   1234,
	}
	b.StartTimer()

	counter := 0
	for i := 0; i < b.N; i++ {
		jsonBytes := wire.JSONBytes(status)
		counter += len(jsonBytes)
	}

}

func BenchmarkEncodeNodeInfo(b *testing.B) {
	b.StopTimer()
	pubKey := crypto.GenPrivKeyEd25519().PubKey().(crypto.PubKeyEd25519)
	nodeInfo := &p2p.NodeInfo{
		PubKey:     pubKey,
		Moniker:    "SOMENAME",
		Network:    "SOMENAME",
		RemoteAddr: "SOMEADDR",
		ListenAddr: "SOMEADDR",
		Version:    "SOMEVER",
		Other:      []string{"SOMESTRING", "OTHERSTRING"},
	}
	b.StartTimer()

	counter := 0
	for i := 0; i < b.N; i++ {
		jsonBytes := wire.JSONBytes(nodeInfo)
		counter += len(jsonBytes)
	}

}
