package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type voteData struct {
	vote1 *Vote
	vote2 *Vote
	valid bool
}

func makeVote(val TestSigner, chainID string, valIndex int, height int64, round, step int, blockID BlockID) *Vote {
	v := &Vote{
		ValidatorAddress: val.Address(),
		ValidatorIndex:   valIndex,
		Height:           height,
		Round:            round,
		Type:             byte(step),
		BlockID:          blockID,
	}
	sig, _ := val.Sign(SignBytes(chainID, v))
	v.Signature = sig
	return v

}

func TestEvidence(t *testing.T) {
	val := GenSigner()
	val2 := GenSigner()
	blockID := makeBlockID("blockhash", 1000, "partshash")
	blockID2 := makeBlockID("blockhash2", 1000, "partshash")
	blockID3 := makeBlockID("blockhash", 10000, "partshash")
	blockID4 := makeBlockID("blockhash", 10000, "partshash2")

	chainID := "mychain"

	vote1 := makeVote(val, chainID, 0, 10, 2, 1, blockID)
	badVote := makeVote(val, chainID, 0, 10, 2, 1, blockID)
	badVote.Signature, _ = val2.Sign(SignBytes(chainID, badVote))

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
			PubKey: val.PubKey(),
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
