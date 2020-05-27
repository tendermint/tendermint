package types

import (
	"math"
	"testing"
	"time"

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

func makeVote(
	t *testing.T, val PrivValidator, chainID string, valIndex int, height int64, round, step int, blockID BlockID,
) *Vote {
	pubKey, err := val.GetPubKey()
	require.NoError(t, err)
	v := &Vote{
		ValidatorAddress: pubKey.Address(),
		ValidatorIndex:   valIndex,
		Height:           height,
		Round:            round,
		Type:             SignedMsgType(step),
		BlockID:          blockID,
	}
	err = val.SignVote(chainID, v)
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

	vote1 := makeVote(t, val, chainID, 0, 10, 2, 1, blockID)
	badVote := makeVote(t, val, chainID, 0, 10, 2, 1, blockID)
	err := val2.SignVote(chainID, badVote)
	assert.NoError(t, err)

	cases := []voteData{
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID2), true}, // different block ids
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID3), true},
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID4), true},
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID), false},     // wrong block id
		{vote1, makeVote(t, val, "mychain2", 0, 10, 2, 1, blockID2), false}, // wrong chain id
		{vote1, makeVote(t, val, chainID, 1, 10, 2, 1, blockID2), false},    // wrong val index
		{vote1, makeVote(t, val, chainID, 0, 11, 2, 1, blockID2), false},    // wrong height
		{vote1, makeVote(t, val, chainID, 0, 10, 3, 1, blockID2), false},    // wrong round
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 2, blockID2), false},    // wrong step
		{vote1, makeVote(t, val2, chainID, 0, 10, 2, 1, blockID), false},    // wrong validator
		{vote1, badVote, false}, // signed by wrong key
	}

	pubKey, err := val.GetPubKey()
	require.NoError(t, err)
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
	ev := randomDuplicatedVoteEvidence(t)

	assert.True(t, ev.Equal(ev))
	assert.False(t, ev.Equal(&DuplicateVoteEvidence{}))
}

func TestEvidenceList(t *testing.T) {
	ev := randomDuplicatedVoteEvidence(t)
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
		VoteA:  makeVote(t, val, chainID, math.MaxInt64, math.MaxInt64, math.MaxInt64, math.MaxInt64, blockID),
		VoteB:  makeVote(t, val, chainID, math.MaxInt64, math.MaxInt64, math.MaxInt64, math.MaxInt64, blockID2),
	}

	bz, err := cdc.MarshalBinaryLengthPrefixed(ev)
	require.NoError(t, err)

	assert.EqualValues(t, MaxEvidenceBytes, len(bz))
}

func randomDuplicatedVoteEvidence(t *testing.T) *DuplicateVoteEvidence {
	val := NewMockPV()
	blockID := makeBlockID([]byte("blockhash"), 1000, []byte("partshash"))
	blockID2 := makeBlockID([]byte("blockhash2"), 1000, []byte("partshash"))
	const chainID = "mychain"
	return &DuplicateVoteEvidence{
		VoteA: makeVote(t, val, chainID, 0, 10, 2, 1, blockID),
		VoteB: makeVote(t, val, chainID, 0, 10, 2, 1, blockID2),
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
			ev.VoteA = makeVote(t, val, chainID, math.MaxInt64, math.MaxInt64, math.MaxInt64, 0, blockID2)
		}, true},
		{"Invalid vote order", func(ev *DuplicateVoteEvidence) {
			swap := ev.VoteA.Copy()
			ev.VoteA = ev.VoteB.Copy()
			ev.VoteB = swap
		}, true},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			pk := secp256k1.GenPrivKey().PubKey()
			vote1 := makeVote(t, val, chainID, math.MaxInt64, math.MaxInt64, math.MaxInt64, 0x02, blockID)
			vote2 := makeVote(t, val, chainID, math.MaxInt64, math.MaxInt64, math.MaxInt64, 0x02, blockID2)
			ev := NewDuplicateVoteEvidence(pk, vote1, vote2)
			tc.malleateEvidence(ev)
			assert.Equal(t, tc.expectErr, ev.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestMockGoodEvidenceValidateBasic(t *testing.T) {
	goodEvidence := NewMockEvidence(int64(1), time.Now(), 1, []byte{1})
	assert.Nil(t, goodEvidence.ValidateBasic())
}

func TestMockBadEvidenceValidateBasic(t *testing.T) {
	badEvidence := NewMockEvidence(int64(1), time.Now(), 1, []byte{1})
	assert.Nil(t, badEvidence.ValidateBasic())
}

func TestEvidenceProto(t *testing.T) {
	// -------- Votes --------
	val := NewMockPV()
	blockID := makeBlockID(tmhash.Sum([]byte("blockhash")), math.MaxInt64, tmhash.Sum([]byte("partshash")))
	blockID2 := makeBlockID(tmhash.Sum([]byte("blockhash2")), math.MaxInt64, tmhash.Sum([]byte("partshash")))
	const chainID = "mychain"
	v := makeVote(t, val, chainID, math.MaxInt64, math.MaxInt64, 1, 0x01, blockID)
	v2 := makeVote(t, val, chainID, math.MaxInt64, math.MaxInt64, 2, 0x01, blockID2)

	tests := []struct {
		testName string
		evidence Evidence
		wantErr  bool
		wantErr2 bool
	}{
		{"&DuplicateVoteEvidence empty fail", &DuplicateVoteEvidence{}, true, true},
		{"&DuplicateVoteEvidence nil voteB", &DuplicateVoteEvidence{VoteA: v, VoteB: nil}, true, true},
		{"&DuplicateVoteEvidence nil voteA", &DuplicateVoteEvidence{VoteA: nil, VoteB: v}, true, true},
		{"&DuplicateVoteEvidence success", &DuplicateVoteEvidence{VoteA: v2, VoteB: v,
			PubKey: val.PrivKey.PubKey()}, false, false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			pb, err := EvidenceToProto(tt.evidence)
			if tt.wantErr {
				assert.Error(t, err, tt.testName)
				return
			}
			assert.NoError(t, err, tt.testName)

			evi, err := EvidenceFromProto(pb)
			if tt.wantErr2 {
				assert.Error(t, err, tt.testName)
				return
			}
			require.Equal(t, tt.evidence, evi, tt.testName)
		})
	}
}
