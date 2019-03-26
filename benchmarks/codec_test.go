package benchmarks

import (
	"testing"
	"time"

	amino "github.com/tendermint/go-amino"

	proto "github.com/tendermint/tendermint/benchmarks/proto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/p2p"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

func testNodeInfo(id p2p.ID) p2p.DefaultNodeInfo {
	return p2p.DefaultNodeInfo{
		ProtocolVersion: p2p.ProtocolVersion{1, 2, 3},
		ID_:             id,
		Moniker:         "SOMENAME",
		Network:         "SOMENAME",
		ListenAddr:      "SOMEADDR",
		Version:         "SOMEVER",
		Other: p2p.DefaultNodeInfoOther{
			TxIndex:    "on",
			RPCAddress: "0.0.0.0:26657",
		},
	}
}

func BenchmarkEncodeStatusWire(b *testing.B) {
	b.StopTimer()
	cdc := amino.NewCodec()
	ctypes.RegisterAmino(cdc)
	nodeKey := p2p.NodeKey{PrivKey: ed25519.GenPrivKey()}
	status := &ctypes.ResultStatus{
		NodeInfo: testNodeInfo(nodeKey.ID()),
		SyncInfo: ctypes.SyncInfo{
			LatestBlockHash:   []byte("SOMEBYTES"),
			LatestBlockHeight: 123,
			LatestBlockTime:   time.Unix(0, 1234),
		},
		ValidatorInfo: ctypes.ValidatorInfo{
			PubKey: nodeKey.PubKey(),
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
	nodeKey := p2p.NodeKey{PrivKey: ed25519.GenPrivKey()}
	nodeInfo := testNodeInfo(nodeKey.ID())
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
	nodeKey := p2p.NodeKey{PrivKey: ed25519.GenPrivKey()}
	nodeInfo := testNodeInfo(nodeKey.ID())
	b.StartTimer()

	counter := 0
	for i := 0; i < b.N; i++ {
		jsonBytes := cdc.MustMarshalBinaryBare(nodeInfo)
		counter += len(jsonBytes)
	}

}

func BenchmarkEncodeNodeInfoProto(b *testing.B) {
	b.StopTimer()
	nodeKey := p2p.NodeKey{PrivKey: ed25519.GenPrivKey()}
	nodeID := string(nodeKey.ID())
	someName := "SOMENAME"
	someAddr := "SOMEADDR"
	someVer := "SOMEVER"
	someString := "SOMESTRING"
	otherString := "OTHERSTRING"
	nodeInfo := proto.NodeInfo{
		Id:         &proto.ID{Id: &nodeID},
		Moniker:    &someName,
		Network:    &someName,
		ListenAddr: &someAddr,
		Version:    &someVer,
		Other:      []string{someString, otherString},
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
