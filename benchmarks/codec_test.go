package benchmarks

import (
	"testing"

	"github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/go-wire"
	proto "github.com/tendermint/tendermint/benchmarks/proto"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

func BenchmarkEncodeStatusWire(b *testing.B) {
	b.StopTimer()
	pubKey := crypto.GenPrivKeyEd25519().PubKey()
	status := &ctypes.ResultStatus{
		NodeInfo: &p2p.NodeInfo{
			PubKey:     pubKey.Unwrap().(crypto.PubKeyEd25519),
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

func BenchmarkEncodeNodeInfoWire(b *testing.B) {
	b.StopTimer()
	pubKey := crypto.GenPrivKeyEd25519().PubKey().Unwrap().(crypto.PubKeyEd25519)
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

func BenchmarkEncodeNodeInfoBinary(b *testing.B) {
	b.StopTimer()
	pubKey := crypto.GenPrivKeyEd25519().PubKey().Unwrap().(crypto.PubKeyEd25519)
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
		jsonBytes := wire.BinaryBytes(nodeInfo)
		counter += len(jsonBytes)
	}

}

func BenchmarkEncodeNodeInfoProto(b *testing.B) {
	b.StopTimer()
	pubKey := crypto.GenPrivKeyEd25519().PubKey().Unwrap().(crypto.PubKeyEd25519)
	pubKey2 := &proto.PubKey{Ed25519: &proto.PubKeyEd25519{Bytes: pubKey[:]}}
	nodeInfo := &proto.NodeInfo{
		PubKey:     pubKey2,
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
		bytes, err := nodeInfo.Marshal()
		if err != nil {
			b.Fatal(err)
			return
		}
		//jsonBytes := wire.JSONBytes(nodeInfo)
		counter += len(bytes)
	}

}
