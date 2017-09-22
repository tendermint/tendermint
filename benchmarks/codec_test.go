// Copyright 2015 Tendermint. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
