package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	wire "github.com/tendermint/go-wire"
)

var testProposal *Proposal

func init() {
	var stamp, err = time.Parse(timeFormat, "2018-02-11T07:09:22.765Z")
	if err != nil {
		panic(err)
	}
	testProposal = &Proposal{
		Height:           12345,
		Round:            23456,
		BlockPartsHeader: PartSetHeader{111, []byte("blockparts")},
		POLRound:         -1,
		Timestamp:        stamp,
	}
}

func TestProposalSignable(t *testing.T) {
	signBytes := SignBytes("test_chain_id", testProposal)
	signStr := string(signBytes)

	expected := `{"chain_id":"test_chain_id","proposal":{"block_parts_header":{"hash":"626C6F636B7061727473","total":111},"height":12345,"pol_block_id":{},"pol_round":-1,"round":23456,"timestamp":"2018-02-11T07:09:22.765Z"}}`
	if signStr != expected {
		t.Errorf("Got unexpected sign string for Proposal. Expected:\n%v\nGot:\n%v", expected, signStr)
	}
}

func TestProposalString(t *testing.T) {
	str := testProposal.String()
	expected := `Proposal{12345/23456 111:626C6F636B70 (-1,:0:000000000000) {<nil>} @ 2018-02-11T07:09:22.765Z}`
	if str != expected {
		t.Errorf("Got unexpected string for Proposal. Expected:\n%v\nGot:\n%v", expected, str)
	}
}

func TestProposalVerifySignature(t *testing.T) {
	privVal := GenPrivValidatorFS("")
	pubKey := privVal.GetPubKey()

	prop := NewProposal(4, 2, PartSetHeader{777, []byte("proper")}, 2, BlockID{})
	signBytes := SignBytes("test_chain_id", prop)

	// sign it
	signature, err := privVal.Signer.Sign(signBytes)
	require.NoError(t, err)

	// verify the same proposal
	valid := pubKey.VerifyBytes(SignBytes("test_chain_id", prop), signature)
	require.True(t, valid)

	// serialize, deserialize and verify again....
	newProp := new(Proposal)
	bs := wire.BinaryBytes(prop)
	err = wire.ReadBinaryBytes(bs, &newProp)
	require.NoError(t, err)

	// verify the transmitted proposal
	newSignBytes := SignBytes("test_chain_id", newProp)
	require.Equal(t, string(signBytes), string(newSignBytes))
	valid = pubKey.VerifyBytes(newSignBytes, signature)
	require.True(t, valid)
}

func BenchmarkProposalWriteSignBytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		SignBytes("test_chain_id", testProposal)
	}
}

func BenchmarkProposalSign(b *testing.B) {
	privVal := GenPrivValidatorFS("")
	for i := 0; i < b.N; i++ {
		_, err := privVal.Signer.Sign(SignBytes("test_chain_id", testProposal))
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkProposalVerifySignature(b *testing.B) {
	signBytes := SignBytes("test_chain_id", testProposal)
	privVal := GenPrivValidatorFS("")
	signature, _ := privVal.Signer.Sign(signBytes)
	pubKey := privVal.GetPubKey()

	for i := 0; i < b.N; i++ {
		pubKey.VerifyBytes(SignBytes("test_chain_id", testProposal), signature)
	}
}
