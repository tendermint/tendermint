package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var testProposal *Proposal

func init() {
	var stamp, err = time.Parse(TimeFormat, "2018-02-11T07:09:22.765Z")
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
	chainID := "test_chain_id"
	signBytes := testProposal.SignBytes(chainID)

	expected, err := cdc.MarshalBinaryLengthPrefixed(CanonicalizeProposal(chainID, testProposal))
	require.NoError(t, err)
	require.Equal(t, expected, signBytes, "Got unexpected sign bytes for Proposal")
}

func TestProposalString(t *testing.T) {
	str := testProposal.String()
	expected := `Proposal{0(Proposal) 12345/23456 111:626C6F636B70 (-1,:0:000000000000) 000000000000 @ 2018-02-11T07:09:22.765Z}`
	if str != expected {
		t.Errorf("Got unexpected string for Proposal. Expected:\n%v\nGot:\n%v", expected, str)
	}
}

func TestProposalVerifySignature(t *testing.T) {
	privVal := NewMockPV()
	pubKey := privVal.GetPubKey()

	prop := NewProposal(4, 2, PartSetHeader{777, []byte("proper")}, 2, BlockID{})
	signBytes := prop.SignBytes("test_chain_id")

	// sign it
	err := privVal.SignProposal("test_chain_id", prop)
	require.NoError(t, err)

	// verify the same proposal
	valid := pubKey.VerifyBytes(signBytes, prop.Signature)
	require.True(t, valid)

	// serialize, deserialize and verify again....
	newProp := new(Proposal)
	bs, err := cdc.MarshalBinaryLengthPrefixed(prop)
	require.NoError(t, err)
	err = cdc.UnmarshalBinaryLengthPrefixed(bs, &newProp)
	require.NoError(t, err)

	// verify the transmitted proposal
	newSignBytes := newProp.SignBytes("test_chain_id")
	require.Equal(t, string(signBytes), string(newSignBytes))
	valid = pubKey.VerifyBytes(newSignBytes, newProp.Signature)
	require.True(t, valid)
}

func BenchmarkProposalWriteSignBytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testProposal.SignBytes("test_chain_id")
	}
}

func BenchmarkProposalSign(b *testing.B) {
	privVal := NewMockPV()
	for i := 0; i < b.N; i++ {
		err := privVal.SignProposal("test_chain_id", testProposal)
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkProposalVerifySignature(b *testing.B) {
	privVal := NewMockPV()
	err := privVal.SignProposal("test_chain_id", testProposal)
	require.Nil(b, err)
	pubKey := privVal.GetPubKey()

	for i := 0; i < b.N; i++ {
		pubKey.VerifyBytes(testProposal.SignBytes("test_chain_id"), testProposal.Signature)
	}
}
