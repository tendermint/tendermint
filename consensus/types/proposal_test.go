package consensus

import (
	"testing"

	"github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	_ "github.com/tendermint/tendermint/config/tendermint_test"
	"github.com/tendermint/tendermint/types"
)

func TestProposalSignable(t *testing.T) {
	proposal := &Proposal{
		Height:     12345,
		Round:      23456,
		BlockParts: types.PartSetHeader{111, []byte("blockparts")},
		POLParts:   types.PartSetHeader{222, []byte("polparts")},
		Signature:  nil,
	}
	signBytes := account.SignBytes(proposal)
	signStr := string(signBytes)
	expected := Fmt(`{"chain_id":"%X","proposal":{"block_parts":{"hash":"626C6F636B7061727473","total":111},"height":12345,"pol_parts":{"hash":"706F6C7061727473","total":222},"round":23456}}`,
		config.GetString("chain_id"))
	if signStr != expected {
		t.Errorf("Got unexpected sign string for SendTx. Expected:\n%v\nGot:\n%v", expected, signStr)
	}
}
