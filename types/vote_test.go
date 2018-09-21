package types

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmtime "github.com/tendermint/tendermint/types/time"
)

func examplePrevote() *UnsignedVote {
	return exampleVote(VoteTypePrevote)
}

func examplePrecommit() *UnsignedVote {
	return exampleVote(VoteTypePrecommit)
}

func exampleVote(t byte) *UnsignedVote {
	var stamp, err = time.Parse(TimeFormat, "2017-12-25T03:00:01.234Z")
	if err != nil {
		panic(err)
	}

	return &UnsignedVote{
		ValidatorAddress: tmhash.Sum([]byte("validator_address")),
		ValidatorIndex:   56789,
		Height:           12345,
		Round:            2,
		Timestamp:        stamp,
		Type:             t,
		BlockID: BlockID{
			Hash: tmhash.Sum([]byte("blockID_hash")),
			PartsHeader: PartSetHeader{
				Total: 1000000,
				Hash:  tmhash.Sum([]byte("blockID_part_set_header_hash")),
			},
		},
		ChainID: "test_chain_id",
	}
}

func TestVoteSignable(t *testing.T) {
	t.Skip("TODO(ismail): switch to amino")
	vote := examplePrecommit()
	signBytes := vote.SignBytes()
	signStr := string(signBytes)

	expected := `{"@chain_id":"test_chain_id","@type":"vote","block_id":{"hash":"8B01023386C371778ECB6368573E539AFC3CC860","parts":{"hash":"72DB3D959635DFF1BB567BEDAA70573392C51596","total":"1000000"}},"height":"12345","round":"2","timestamp":"2017-12-25T03:00:01.234Z","type":2}`
	if signStr != expected {
		// NOTE: when this fails, you probably want to fix up consensus/replay_test too
		t.Errorf("Got unexpected sign string for UnsignedVote. Expected:\n%v\nGot:\n%v", expected, signStr)
	}
}

func TestVoteVerifySignature(t *testing.T) {
	privVal := NewMockPV()
	pubkey := privVal.GetPubKey()

	vote := examplePrecommit()
	signBytes := vote.SignBytes()

	// sign it
	signedVote, err := privVal.SignVote(vote)
	require.NoError(t, err)

	// verify the same vote
	valid := pubkey.VerifyBytes(vote.SignBytes(), signedVote.Signature)
	require.True(t, valid)

	// serialize, deserialize and verify again....
	precommit := new(SignedVote)
	bs, err := cdc.MarshalBinary(signedVote)
	require.NoError(t, err)
	err = cdc.UnmarshalBinary(bs, &precommit)
	require.NoError(t, err)

	// verify the transmitted vote
	newSignBytes := precommit.Vote.SignBytes()
	require.Equal(t, string(signBytes), string(newSignBytes))
	valid = pubkey.VerifyBytes(newSignBytes, precommit.Signature)
	require.True(t, valid)
}

func TestIsVoteTypeValid(t *testing.T) {
	tc := []struct {
		name string
		in   byte
		out  bool
	}{
		{"Prevote", VoteTypePrevote, true},
		{"Precommit", VoteTypePrecommit, true},
		{"InvalidType", byte(3), false},
	}

	for _, tt := range tc {
		tt := tt
		t.Run(tt.name, func(st *testing.T) {
			if rs := IsVoteTypeValid(tt.in); rs != tt.out {
				t.Errorf("Got unexpected UnsignedVote type. Expected:\n%v\nGot:\n%v", rs, tt.out)
			}
		})
	}
}

func TestVoteVerify(t *testing.T) {
	privVal := NewMockPV()
	pubkey := privVal.GetPubKey()

	vote := examplePrevote()
	vote.ValidatorAddress = pubkey.Address()
	sv := &SignedVote{Vote: vote}

	err := sv.Verify(ed25519.GenPrivKey().PubKey())
	if assert.Error(t, err) {
		assert.Equal(t, ErrVoteInvalidValidatorAddress, err)
	}

	err = sv.Verify(pubkey)
	if assert.Error(t, err) {
		assert.Equal(t, ErrVoteInvalidSignature, err)
	}
}

func TestMaxVoteBytes(t *testing.T) {
	vote := &UnsignedVote{
		ValidatorAddress: tmhash.Sum([]byte("validator_address")),
		ValidatorIndex:   math.MaxInt64,
		Height:           math.MaxInt64,
		Round:            math.MaxInt64,
		Timestamp:        tmtime.Now(),
		Type:             VoteTypePrevote,
		BlockID: BlockID{
			Hash: tmhash.Sum([]byte("blockID_hash")),
			PartsHeader: PartSetHeader{
				Total: math.MaxInt64,
				Hash:  tmhash.Sum([]byte("blockID_part_set_header_hash")),
			},
		},
	}

	privVal := NewMockPV()
	sv, err := privVal.SignVote(vote)
	require.NoError(t, err)

	bz, err := cdc.MarshalBinary(sv)
	require.NoError(t, err)
	// TODO(ismail): if we include the chainId in the vote this varies ...
	// we need a max size for chainId too
	assert.Equal(t, MaxVoteBytes, len(bz))
}
