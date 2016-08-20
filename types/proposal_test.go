package types

import (
	"testing"
)

func TestProposalSignable(t *testing.T) {
	proposal := &Proposal{
		Height:           12345,
		Round:            23456,
		BlockPartsHeader: PartSetHeader{111, []byte("blockparts")},
		POLRound:         -1,
	}
	signBytes := SignBytes("test_chain_id", proposal)
	signStr := string(signBytes)

	expected := `{"chain_id":"test_chain_id","proposal":{"block_parts_header":{"hash":"626C6F636B7061727473","total":111},"height":12345,"pol_block_id":null,"pol_round":-1,"round":23456}}`
	if signStr != expected {
		t.Errorf("Got unexpected sign string for Proposal. Expected:\n%v\nGot:\n%v", expected, signStr)
	}
}
