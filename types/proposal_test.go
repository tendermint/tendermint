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
	signature := privVal.Sign(signBytes)
	pubKey := privVal.PubKey

	for i := 0; i < b.N; i++ {
		pubKey.VerifyBytes(SignBytes("test_chain_id", testProposal), signature)
	}
}
