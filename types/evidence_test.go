package types

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmrand "github.com/tendermint/tendermint/libs/rand"
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
		VoteA: makeVote(t, val, chainID, math.MaxInt64, math.MaxInt64, math.MaxInt64, math.MaxInt64, blockID),
		VoteB: makeVote(t, val, chainID, math.MaxInt64, math.MaxInt64, math.MaxInt64, math.MaxInt64, blockID2),
	}

	//TODO: Add other types of evidence to test and set MaxEvidenceBytes accordingly

	// evl := &LunaticValidatorEvidence{
	// Header: makeHeaderRandom(),
	// Vote:   makeVote(t, val, chainID, math.MaxInt64, math.MaxInt64, math.MaxInt64, math.MaxInt64, blockID2),

	// 	InvalidHeaderField: "",
	// }

	// evp := &PhantomValidatorEvidence{
	// 	Header: makeHeaderRandom(),
	// 	Vote:   makeVote(t, val, chainID, math.MaxInt64, math.MaxInt64, math.MaxInt64, math.MaxInt64, blockID2),

	// 	LastHeightValidatorWasInSet: math.MaxInt64,
	// }

	// signedHeader := SignedHeader{Header: makeHeaderRandom(), Commit: randCommit(time.Now())}
	// evc := &ConflictingHeadersEvidence{
	// 	H1: &signedHeader,
	// 	H2: &signedHeader,
	// }

	testCases := []struct {
		testName string
		evidence Evidence
	}{
		{"DuplicateVote", ev},
		// {"LunaticValidatorEvidence", evl},
		// {"PhantomValidatorEvidence", evp},
		// {"ConflictingHeadersEvidence", evc},
	}

	for _, tt := range testCases {
		bz, err := cdc.MarshalBinaryLengthPrefixed(tt.evidence)
		require.NoError(t, err, tt.testName)

		assert.LessOrEqual(t, MaxEvidenceBytes, int64(len(bz)), tt.testName)
	}

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
			vote1 := makeVote(t, val, chainID, math.MaxInt64, math.MaxInt64, math.MaxInt64, 0x02, blockID)
			vote2 := makeVote(t, val, chainID, math.MaxInt64, math.MaxInt64, math.MaxInt64, 0x02, blockID2)
			ev := NewDuplicateVoteEvidence(vote1, vote2)
			tc.malleateEvidence(ev)
			assert.Equal(t, tc.expectErr, ev.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestMockGoodEvidenceValidateBasic(t *testing.T) {
	goodEvidence := NewMockEvidence(int64(1), time.Now(), []byte{1})
	assert.Nil(t, goodEvidence.ValidateBasic())
}

func TestMockBadEvidenceValidateBasic(t *testing.T) {
	badEvidence := NewMockEvidence(int64(1), time.Now(), []byte{1})
	assert.Nil(t, badEvidence.ValidateBasic())
}

func TestLunaticValidatorEvidence(t *testing.T) {
	var (
		blockID  = makeBlockIDRandom()
		header   = makeHeaderRandom()
		bTime, _ = time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
		val      = NewMockPV()
		vote     = makeVote(t, val, header.ChainID, 0, header.Height, 0, 2, blockID)
	)

	header.Time = bTime

	ev := &LunaticValidatorEvidence{
		Header:             header,
		Vote:               vote,
		InvalidHeaderField: "AppHash",
	}

	assert.Equal(t, header.Height, ev.Height())
	assert.Equal(t, bTime, ev.Time())
	assert.EqualValues(t, vote.ValidatorAddress, ev.Address())
	assert.NotEmpty(t, ev.Hash())
	assert.NotEmpty(t, ev.Bytes())
	pubKey, err := val.GetPubKey()
	require.NoError(t, err)
	assert.NoError(t, ev.Verify(header.ChainID, pubKey))
	assert.Error(t, ev.Verify("other", pubKey))
	privKey2 := ed25519.GenPrivKey()
	pubKey2 := privKey2.PubKey()
	assert.Error(t, ev.Verify("other", pubKey2))
	assert.True(t, ev.Equal(ev))
	assert.NoError(t, ev.ValidateBasic())
	assert.NotEmpty(t, ev.String())
}

func TestPhantomValidatorEvidence(t *testing.T) {
	var (
		blockID  = makeBlockIDRandom()
		header   = makeHeaderRandom()
		bTime, _ = time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
		val      = NewMockPV()
		vote     = makeVote(t, val, header.ChainID, 0, header.Height, 0, 2, blockID)
	)

	header.Time = bTime

	ev := &PhantomValidatorEvidence{
		Header:                      header,
		Vote:                        vote,
		LastHeightValidatorWasInSet: header.Height - 1,
	}

	assert.Equal(t, header.Height, ev.Height())
	assert.Equal(t, bTime, ev.Time())
	assert.EqualValues(t, vote.ValidatorAddress, ev.Address())
	assert.NotEmpty(t, ev.Hash())
	assert.NotEmpty(t, ev.Bytes())
	pubKey, err := val.GetPubKey()
	require.NoError(t, err)
	assert.NoError(t, ev.Verify(header.ChainID, pubKey))
	assert.Error(t, ev.Verify("other", pubKey))
	privKey2 := ed25519.GenPrivKey()
	pubKey2 := privKey2.PubKey()
	assert.Error(t, ev.Verify("other", pubKey2))
	assert.True(t, ev.Equal(ev))
	assert.NoError(t, ev.ValidateBasic())
	assert.NotEmpty(t, ev.String())
}

func TestConflictingHeadersEvidence(t *testing.T) {
	const (
		chainID       = "TestConflictingHeadersEvidence"
		height  int64 = 37
	)

	var (
		blockID = makeBlockIDRandom()
		header1 = makeHeaderRandom()
		header2 = makeHeaderRandom()
	)

	header1.Height = height
	header1.LastBlockID = blockID
	header1.ChainID = chainID

	header2.Height = height
	header2.LastBlockID = blockID
	header2.ChainID = chainID

	voteSet1, valSet, vals := randVoteSet(height, 1, PrecommitType, 10, 1)
	voteSet2 := NewVoteSet(chainID, height, 1, PrecommitType, valSet)

	commit1, err := MakeCommit(BlockID{
		Hash: header1.Hash(),
		PartsHeader: PartSetHeader{
			Total: 100,
			Hash:  crypto.CRandBytes(tmhash.Size),
		},
	}, height, 1, voteSet1, vals, time.Now())
	require.NoError(t, err)
	commit2, err := MakeCommit(BlockID{
		Hash: header2.Hash(),
		PartsHeader: PartSetHeader{
			Total: 100,
			Hash:  crypto.CRandBytes(tmhash.Size),
		},
	}, height, 1, voteSet2, vals, time.Now())
	require.NoError(t, err)

	ev := &ConflictingHeadersEvidence{
		H1: &SignedHeader{
			Header: header1,
			Commit: commit1,
		},
		H2: &SignedHeader{
			Header: header2,
			Commit: commit2,
		},
	}

	assert.Panics(t, func() {
		ev.Address()
	})

	assert.Panics(t, func() {
		pubKey, _ := vals[0].GetPubKey()
		ev.Verify(chainID, pubKey)
	})

	assert.Equal(t, height, ev.Height())
	// assert.Equal(t, bTime, ev.Time())
	assert.NotEmpty(t, ev.Hash())
	assert.NotEmpty(t, ev.Bytes())
	assert.NoError(t, ev.VerifyComposite(header1, valSet))
	assert.True(t, ev.Equal(ev))
	assert.NoError(t, ev.ValidateBasic())
	assert.NotEmpty(t, ev.String())
}

func TestPotentialAmnesiaEvidence(t *testing.T) {
	const (
		chainID       = "TestPotentialAmnesiaEvidence"
		height  int64 = 37
	)

	var (
		val      = NewMockPV()
		blockID  = makeBlockID(tmhash.Sum([]byte("blockhash")), math.MaxInt64, tmhash.Sum([]byte("partshash")))
		blockID2 = makeBlockID(tmhash.Sum([]byte("blockhash2")), math.MaxInt64, tmhash.Sum([]byte("partshash")))
		vote1    = makeVote(t, val, chainID, 0, height, 0, 2, blockID)
		vote2    = makeVote(t, val, chainID, 0, height, 1, 2, blockID2)
	)

	ev := &PotentialAmnesiaEvidence{
		VoteA: vote2,
		VoteB: vote1,
	}

	assert.Equal(t, height, ev.Height())
	// assert.Equal(t, bTime, ev.Time())
	assert.EqualValues(t, vote1.ValidatorAddress, ev.Address())
	assert.NotEmpty(t, ev.Hash())
	assert.NotEmpty(t, ev.Bytes())
	pubKey, err := val.GetPubKey()
	require.NoError(t, err)
	assert.NoError(t, ev.Verify(chainID, pubKey))
	assert.Error(t, ev.Verify("other", pubKey))
	privKey2 := ed25519.GenPrivKey()
	pubKey2 := privKey2.PubKey()
	assert.Error(t, ev.Verify("other", pubKey2))
	assert.True(t, ev.Equal(ev))
	assert.NoError(t, ev.ValidateBasic())
	assert.NotEmpty(t, ev.String())
}

func TestProofOfLockChange(t *testing.T) {
	const (
		chainID       = "TestProofOfLockChange"
		height  int64 = 37
	)
	// 1: valid POLC - nothing should fail
	voteSet, valSet, privValidators, blockID := buildVoteSet(height, 1, 3, 7, 0, PrecommitType)
	pubKey, err := privValidators[7].GetPubKey()
	require.NoError(t, err)
	polc := makePOLCFromVoteSet(voteSet, pubKey, blockID)

	assert.Equal(t, height, polc.Height())
	assert.NoError(t, polc.ValidateBasic())
	assert.True(t, polc.MajorityOfVotes(valSet))
	assert.NotEmpty(t, polc.String())

	// test validate basic on a set of bad cases
	var badPOLCs []ProofOfLockChange
	// 2: node has already voted in next round
	pubKey, err = privValidators[0].GetPubKey()
	require.NoError(t, err)
	polc2 := makePOLCFromVoteSet(voteSet, pubKey, blockID)
	badPOLCs = append(badPOLCs, polc2)
	// 3: one vote was from a different round
	voteSet, _, privValidators, blockID = buildVoteSet(height, 1, 3, 7, 0, PrecommitType)
	pubKey, err = privValidators[7].GetPubKey()
	require.NoError(t, err)
	polc = makePOLCFromVoteSet(voteSet, pubKey, blockID)
	badVote := makeVote(t, privValidators[8], chainID, 8, height, 2, 2, blockID)
	polc.Votes = append(polc.Votes, *badVote)
	badPOLCs = append(badPOLCs, polc)
	// 4: one vote was from a different height
	polc = makePOLCFromVoteSet(voteSet, pubKey, blockID)
	badVote = makeVote(t, privValidators[8], chainID, 8, height+1, 1, 2, blockID)
	polc.Votes = append(polc.Votes, *badVote)
	badPOLCs = append(badPOLCs, polc)
	// 5: one vote was from a different vote type
	polc = makePOLCFromVoteSet(voteSet, pubKey, blockID)
	badVote = makeVote(t, privValidators[8], chainID, 8, height, 1, 1, blockID)
	polc.Votes = append(polc.Votes, *badVote)
	badPOLCs = append(badPOLCs, polc)
	// 5: one of the votes was for a nil block
	polc = makePOLCFromVoteSet(voteSet, pubKey, blockID)
	badVote = makeVote(t, privValidators[8], chainID, 8, height, 1, 2, BlockID{})
	polc.Votes = append(polc.Votes, *badVote)
	badPOLCs = append(badPOLCs, polc)

	for idx, polc := range badPOLCs {
		err := polc.ValidateBasic()
		assert.Error(t, err)
		if err == nil {
			t.Errorf("test no. %d failed", idx+2)
		}
	}

}

func makeHeaderRandom() *Header {
	return &Header{
		ChainID:            tmrand.Str(12),
		Height:             int64(tmrand.Uint16()) + 1,
		Time:               time.Now(),
		LastBlockID:        makeBlockIDRandom(),
		LastCommitHash:     crypto.CRandBytes(tmhash.Size),
		DataHash:           crypto.CRandBytes(tmhash.Size),
		ValidatorsHash:     crypto.CRandBytes(tmhash.Size),
		NextValidatorsHash: crypto.CRandBytes(tmhash.Size),
		ConsensusHash:      crypto.CRandBytes(tmhash.Size),
		AppHash:            crypto.CRandBytes(tmhash.Size),
		LastResultsHash:    crypto.CRandBytes(tmhash.Size),
		EvidenceHash:       crypto.CRandBytes(tmhash.Size),
		ProposerAddress:    crypto.CRandBytes(crypto.AddressSize),
	}
}
