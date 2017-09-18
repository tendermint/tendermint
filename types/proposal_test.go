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

package types

import (
	"testing"
)

var testProposal = &Proposal{
	Height:           12345,
	Round:            23456,
	BlockPartsHeader: PartSetHeader{111, []byte("blockparts")},
	POLRound:         -1,
}

func TestProposalSignable(t *testing.T) {
	signBytes := SignBytes("test_chain_id", testProposal)
	signStr := string(signBytes)

	expected := `{"chain_id":"test_chain_id","proposal":{"block_parts_header":{"hash":"626C6F636B7061727473","total":111},"height":12345,"pol_block_id":{},"pol_round":-1,"round":23456}}`
	if signStr != expected {
		t.Errorf("Got unexpected sign string for Proposal. Expected:\n%v\nGot:\n%v", expected, signStr)
	}
}

func BenchmarkProposalWriteSignBytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		SignBytes("test_chain_id", testProposal)
	}
}

func BenchmarkProposalSign(b *testing.B) {
	privVal := GenPrivValidator()
	for i := 0; i < b.N; i++ {
		privVal.Sign(SignBytes("test_chain_id", testProposal))
	}
}

func BenchmarkProposalVerifySignature(b *testing.B) {
	signBytes := SignBytes("test_chain_id", testProposal)
	privVal := GenPrivValidator()
	signature, _ := privVal.Sign(signBytes)
	pubKey := privVal.PubKey

	for i := 0; i < b.N; i++ {
		pubKey.VerifyBytes(SignBytes("test_chain_id", testProposal), signature)
	}
}
