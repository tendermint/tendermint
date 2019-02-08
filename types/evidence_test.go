package types

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	"github.com/tendermint/tendermint/crypto/tmhash"
)

type voteData struct {
	vote1 *Vote
	vote2 *Vote
	valid bool
}

func makeVote(val PrivValidator, chainID string, valIndex int, height int64, round, step int, blockID BlockID) *Vote {
	addr := val.GetPubKey().Address()
	v := &Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   valIndex,
		Height:           height,
		Round:            round,
		Type:             SignedMsgType(step),
		BlockID:          blockID,
	}
	err := val.SignVote(chainID, v)
	if err != nil {
		panic(err)
	}
	return v
}

func TestEvidence(t *testing.T) {
	val := NewMockPV()
	val2 := NewMockPV()

	blockID := makeBlockID([]byte("blockhash"), 1000, []byte("partshash"))
	blockID2 := makeBlockID([]byte("blockhash2"), 1000, []byte("partshash"))
	blockID3 := makeBlockID([]byte("blockhash"), 10000, []byte("partshash"))
	blockID4 := makeBlockID([]byte("blockhash"), 10000, []byte("partshash2"))

	const chainID = "mychain"

	vote1 := makeVote(val, chainID, 0, 10, 2, 1, blockID)
	badVote := makeVote(val, chainID, 0, 10, 2, 1, blockID)
	err := val2.SignVote(chainID, badVote)
	if err != nil {
		panic(err)
	}

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
		{vote1, badVote, false}, // signed by wrong key
	}

	pubKey := val.GetPubKey()
	for _, c := range cases {
		ev := &DuplicateVoteEvidence{
			VoteA: c.vote1,
			VoteB: c.vote2,
		}
		if c.valid {
			assert.Nil(t, ev.Verify(chainID, pubKey), "evidence should be valid")
		} else {
			assert.NotNil(t, ev.Verify(chainID, pubKey), "evidence should be invalid")
		}
	}
}

func TestDuplicatedVoteEvidence(t *testing.T) {
	ev := randomDuplicatedVoteEvidence()

	assert.True(t, ev.Equal(ev))
	assert.False(t, ev.Equal(&DuplicateVoteEvidence{}))
}

func TestEvidenceList(t *testing.T) {
	ev := randomDuplicatedVoteEvidence()
	evl := EvidenceList([]Evidence{ev})

	assert.NotNil(t, evl.Hash())
	assert.True(t, evl.Has(ev))
	assert.False(t, evl.Has(&DuplicateVoteEvidence{}))
}

func TestMaxEvidenceBytes(t *testing.T) {
	val := NewMockPV()
	blockID := makeBlockID(tmhash.Sum([]byte("blockhash")), math.MaxInt64, tmhash.Sum([]byte("partshash")))
	blockID2 := makeBlockID(tmhash.Sum([]byte("blockhash2")), math.MaxInt64, tmhash.Sum([]byte("partshash")))
	const chainID = "mychain"
	ev := &DuplicateVoteEvidence{
		PubKey: secp256k1.GenPrivKey().PubKey(), // use secp because it's pubkey is longer
		VoteA:  makeVote(val, chainID, math.MaxInt64, math.MaxInt64, math.MaxInt64, math.MaxInt64, blockID),
		VoteB:  makeVote(val, chainID, math.MaxInt64, math.MaxInt64, math.MaxInt64, math.MaxInt64, blockID2),
	}

	bz, err := cdc.MarshalBinaryLengthPrefixed(ev)
	require.NoError(t, err)

	assert.EqualValues(t, MaxEvidenceBytes, len(bz))
}

func randomDuplicatedVoteEvidence() *DuplicateVoteEvidence {
	val := NewMockPV()
	blockID := makeBlockID([]byte("blockhash"), 1000, []byte("partshash"))
	blockID2 := makeBlockID([]byte("blockhash2"), 1000, []byte("partshash"))
	const chainID = "mychain"
	return &DuplicateVoteEvidence{
		VoteA: makeVote(val, chainID, 0, 10, 2, 1, blockID),
		VoteB: makeVote(val, chainID, 0, 10, 2, 1, blockID2),
	}
}

func TestDuplicateVoteEvidenceValidation(t *testing.T) {
	val := NewMockPV()
	blockID := makeBlockID(tmhash.Sum([]byte("blockhash")), math.MaxInt64, tmhash.Sum([]byte("partshash")))
	blockID2 := makeBlockID(tmhash.Sum([]byte("blockhash2")), math.MaxInt64, tmhash.Sum([]byte("partshash")))
	const chainID = "mychain"

	testCases := []struct {
		testName         string
		malleateEvidence func(*DuplicateVoteEvidence)
		expectErr        bool
	}{
		{"Good DuplicateVoteEvidence", func(ev *DuplicateVoteEvidence) {}, false},
		{"Nil vote A", func(ev *DuplicateVoteEvidence) { ev.VoteA = nil }, true},
		{"Nil vote B", func(ev *DuplicateVoteEvidence) { ev.VoteB = nil }, true},
		{"Nil votes", func(ev *DuplicateVoteEvidence) {
			ev.VoteA = nil
			ev.VoteB = nil
		}, true},
		{"Invalid vote type", func(ev *DuplicateVoteEvidence) {
			ev.VoteA = makeVote(val, chainID, math.MaxInt64, math.MaxInt64, math.MaxInt64, 0, blockID2)
		}, true},
	}
	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			ev := &DuplicateVoteEvidence{
				PubKey: secp256k1.GenPrivKey().PubKey(),
				VoteA:  makeVote(val, chainID, math.MaxInt64, math.MaxInt64, math.MaxInt64, 0x02, blockID),
				VoteB:  makeVote(val, chainID, math.MaxInt64, math.MaxInt64, math.MaxInt64, 0x02, blockID2),
			}
			tc.malleateEvidence(ev)
			assert.Equal(t, tc.expectErr, ev.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}
