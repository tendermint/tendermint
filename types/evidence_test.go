package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	cmn "github.com/tendermint/tmlibs/common"
)

type voteData struct {
	vote1 *Vote
	vote2 *Vote
	valid bool
}

func makeVote(val *PrivValidatorFS, chainID string, valIndex int, height int64, round, step int, blockID BlockID) *Vote {
	v := &Vote{
		ValidatorAddress: val.PubKey.Address(),
		ValidatorIndex:   valIndex,
		Height:           height,
		Round:            round,
		Type:             byte(step),
		BlockID:          blockID,
	}
	sig := val.PrivKey.Sign(SignBytes(chainID, v))
	v.Signature = sig
	return v

}

func TestEvidence(t *testing.T) {
	_, tmpFilePath := cmn.Tempfile("priv_validator_")
	val := GenPrivValidatorFS(tmpFilePath)
	val2 := GenPrivValidatorFS(tmpFilePath)
	blockID := makeBlockID("blockhash", 1000, "partshash")
	blockID2 := makeBlockID("blockhash2", 1000, "partshash")
	blockID3 := makeBlockID("blockhash", 10000, "partshash")
	blockID4 := makeBlockID("blockhash", 10000, "partshash2")

	chainID := "mychain"

	vote1 := makeVote(val, chainID, 0, 10, 2, 1, blockID)
	badVote := makeVote(val, chainID, 0, 10, 2, 1, blockID)
	badVote.Signature = val2.PrivKey.Sign(SignBytes(chainID, badVote))

	cases := []voteData{
		{vote1, makeVote(val, chainID, 0, 10, 2, 1, blockID2), true}, // different block ids
		{vote1, makeVote(val, chainID, 0, 10, 2, 1, blockID3), true},
		{vote1, makeVote(val, chainID, 0, 10, 2, 1, blockID4), true},
		{vote1, makeVote(val, chainID, 0, 10, 2, 1, blockID), false},     // wrong block id
		{vote1, makeVote(val, "mychain2", 0, 10, 2, 1, blockID2), false}, // wrong chain id
		{vote1, makeVote(val, chainID, 1, 10, 2, 1, blockID2), false},    // wrong val index
		{vote1, makeVote(val, chainID, 0, 11, 2, 1, blockID2), false},    // wrong height
		{vote1, makeVote(val, chainID, 0, 10, 3, 1, blockID2), false},    // wrong round
		{vote1, makeVote(val, chainID, 0, 10, 2, 2, blockID2), false},    // wrong step
		{vote1, makeVote(val2, chainID, 0, 10, 2, 1, blockID), false},    // wrong validator
		{vote1, badVote, false},                                          // signed by wrong key
	}

	for _, c := range cases {
		ev := &DuplicateVoteEvidence{
			PubKey: val.PubKey,
			VoteA:  c.vote1,
			VoteB:  c.vote2,
		}
		if c.valid {
			assert.Nil(t, ev.Verify(chainID), "evidence should be valid")
		} else {
			assert.NotNil(t, ev.Verify(chainID), "evidence should be invalid")
		}
	}

}
