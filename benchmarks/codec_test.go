package benchmarks

import (
	"testing"
	"time"

	"github.com/tendermint/go-amino"
	"github.com/tendermint/go-crypto"

	proto "github.com/tendermint/tendermint/benchmarks/proto"
	"github.com/tendermint/tendermint/p2p"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

func BenchmarkEncodeStatusWire(b *testing.B) {
	b.StopTimer()
	cdc := amino.NewCodec()
	ctypes.RegisterAmino(cdc)
	pubKey := crypto.GenPrivKeyEd25519().PubKey()
	status := &ctypes.ResultStatus{
		NodeInfo: p2p.NodeInfo{
			PubKey:     pubKey,
			Moniker:    "SOMENAME",
			Network:    "SOMENAME",
			ListenAddr: "SOMEADDR",
			Version:    "SOMEVER",
			Other:      []string{"SOMESTRING", "OTHERSTRING"},
		},
		SyncInfo: ctypes.SyncInfo{
			LatestBlockHash:   []byte("SOMEBYTES"),
			LatestBlockHeight: 123,
			LatestBlockTime:   time.Unix(0, 1234),
		},
		ValidatorInfo: ctypes.ValidatorInfo{
			PubKey:            pubKey,
		},
	}
	b.StartTimer()

	counter := 0
	for i := 0; i < b.N; i++ {
		jsonBytes, err := cdc.MarshalJSON(status)
		if err != nil {
			panic(err)
		}
		counter += len(jsonBytes)
	}

}

func BenchmarkEncodeNodeInfoWire(b *testing.B) {
	b.StopTimer()
	cdc := amino.NewCodec()
	ctypes.RegisterAmino(cdc)
	pubKey := crypto.GenPrivKeyEd25519().PubKey()
	nodeInfo := p2p.NodeInfo{
		PubKey:     pubKey,
		Moniker:    "SOMENAME",
		Network:    "SOMENAME",
		ListenAddr: "SOMEADDR",
		Version:    "SOMEVER",
		Other:      []string{"SOMESTRING", "OTHERSTRING"},
	}
	b.StartTimer()

	counter := 0
	for i := 0; i < b.N; i++ {
		jsonBytes, err := cdc.MarshalJSON(nodeInfo)
		if err != nil {
			panic(err)
		}
		counter += len(jsonBytes)
	}
}

func BenchmarkEncodeNodeInfoBinary(b *testing.B) {
	b.StopTimer()
	cdc := amino.NewCodec()
	ctypes.RegisterAmino(cdc)
	pubKey := crypto.GenPrivKeyEd25519().PubKey()
	nodeInfo := p2p.NodeInfo{
		PubKey:     pubKey,
		Moniker:    "SOMENAME",
		Network:    "SOMENAME",
		ListenAddr: "SOMEADDR",
		Version:    "SOMEVER",
		Other:      []string{"SOMESTRING", "OTHERSTRING"},
	}
	b.StartTimer()

	counter := 0
	for i := 0; i < b.N; i++ {
		jsonBytes := cdc.MustMarshalBinaryBare(nodeInfo)
		counter += len(jsonBytes)
	}

}

func BenchmarkEncodeNodeInfoProto(b *testing.B) {
	b.StopTimer()
	pubKey := crypto.GenPrivKeyEd25519().PubKey().(crypto.PubKeyEd25519)
	pubKey2 := &proto.PubKey{Ed25519: &proto.PubKeyEd25519{Bytes: pubKey[:]}}
	nodeInfo := proto.NodeInfo{
		PubKey:     pubKey2,
		Moniker:    "SOMENAME",
		Network:    "SOMENAME",
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
