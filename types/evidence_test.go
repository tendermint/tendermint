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
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

type voteData struct {
	vote1 *Vote
	vote2 *Vote
	valid bool
}

var defaultVoteTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)

func TestDuplicateVoteEvidence(t *testing.T) {
	val := NewMockPV()
	val2 := NewMockPV()

	blockID := makeBlockID([]byte("blockhash"), 1000, []byte("partshash"))
	blockID2 := makeBlockID([]byte("blockhash2"), 1000, []byte("partshash"))
	blockID3 := makeBlockID([]byte("blockhash"), 10000, []byte("partshash"))
	blockID4 := makeBlockID([]byte("blockhash"), 10000, []byte("partshash2"))

	const chainID = "mychain"

	vote1 := makeVote(t, val, chainID, 0, 10, 2, 1, blockID, defaultVoteTime)
	v1 := vote1.ToProto()
	err := val.SignVote(chainID, v1)
	require.NoError(t, err)
	badVote := makeVote(t, val, chainID, 0, 10, 2, 1, blockID, defaultVoteTime)
	bv := badVote.ToProto()
	err = val2.SignVote(chainID, bv)
	require.NoError(t, err)

	vote1.Signature = v1.Signature
	badVote.Signature = bv.Signature

	cases := []voteData{
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID2, defaultVoteTime), true}, // different block ids
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID3, defaultVoteTime), true},
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID4, defaultVoteTime), true},
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID, defaultVoteTime), false},     // wrong block id
		{vote1, makeVote(t, val, "mychain2", 0, 10, 2, 1, blockID2, defaultVoteTime), false}, // wrong chain id
		{vote1, makeVote(t, val, chainID, 0, 11, 2, 1, blockID2, defaultVoteTime), false},    // wrong height
		{vote1, makeVote(t, val, chainID, 0, 10, 3, 1, blockID2, defaultVoteTime), false},    // wrong round
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 2, blockID2, defaultVoteTime), false},    // wrong step
		{vote1, makeVote(t, val2, chainID, 0, 10, 2, 1, blockID, defaultVoteTime), false},    // wrong validator
		{vote1, makeVote(t, val2, chainID, 0, 10, 2, 1, blockID, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)), false},
		{vote1, badVote, false}, // signed by wrong key
	}

	pubKey, err := val.GetPubKey()
	require.NoError(t, err)
	for _, c := range cases {
		ev := &DuplicateVoteEvidence{
			VoteA: c.vote1,
			VoteB: c.vote2,

			Timestamp: defaultVoteTime,
		}
		if c.valid {
			assert.Nil(t, ev.Verify(chainID, pubKey), "evidence should be valid")
		} else {
			assert.NotNil(t, ev.Verify(chainID, pubKey), "evidence should be invalid")
		}
	}

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
	blockID := makeBlockID(tmhash.Sum([]byte("blockhash")), math.MaxInt32, tmhash.Sum([]byte("partshash")))
	blockID2 := makeBlockID(tmhash.Sum([]byte("blockhash2")), math.MaxInt32, tmhash.Sum([]byte("partshash")))
	maxTime := time.Date(9999, 0, 0, 0, 0, 0, 0, time.UTC)
	const chainID = "mychain"
	ev := &DuplicateVoteEvidence{
		VoteA: makeVote(t, val, chainID, math.MaxInt32, math.MaxInt64, math.MaxInt32, math.MaxInt64, blockID, maxTime),
		VoteB: makeVote(t, val, chainID, math.MaxInt32, math.MaxInt64, math.MaxInt32, math.MaxInt64, blockID2, maxTime),
	}

	//TODO: Add other types of evidence to test and set MaxEvidenceBytes accordingly

	testCases := []struct {
		testName string
		evidence Evidence
	}{
		{"DuplicateVote", ev},
	}

	for _, tt := range testCases {
		pb, err := EvidenceToProto(tt.evidence)
		require.NoError(t, err, tt.testName)
		bz, err := pb.Marshal()
		require.NoError(t, err, tt.testName)

		assert.LessOrEqual(t, int64(len(bz)), MaxEvidenceBytes, tt.testName)
	}

}

func randomDuplicatedVoteEvidence(t *testing.T) *DuplicateVoteEvidence {
	val := NewMockPV()
	blockID := makeBlockID([]byte("blockhash"), 1000, []byte("partshash"))
	blockID2 := makeBlockID([]byte("blockhash2"), 1000, []byte("partshash"))
	const chainID = "mychain"
	return &DuplicateVoteEvidence{
		VoteA: makeVote(t, val, chainID, 0, 10, 2, 1, blockID, defaultVoteTime),
		VoteB: makeVote(t, val, chainID, 0, 10, 2, 1, blockID2, defaultVoteTime.Add(1*time.Minute)),
	}
}

func TestDuplicateVoteEvidenceValidation(t *testing.T) {
	val := NewMockPV()
	blockID := makeBlockID(tmhash.Sum([]byte("blockhash")), math.MaxInt32, tmhash.Sum([]byte("partshash")))
	blockID2 := makeBlockID(tmhash.Sum([]byte("blockhash2")), math.MaxInt32, tmhash.Sum([]byte("partshash")))
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
			ev.VoteA = makeVote(t, val, chainID, math.MaxInt32, math.MaxInt64, math.MaxInt32, 0, blockID2, defaultVoteTime)
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
			vote1 := makeVote(t, val, chainID, math.MaxInt32, math.MaxInt64, math.MaxInt32, 0x02, blockID, defaultVoteTime)
			vote2 := makeVote(t, val, chainID, math.MaxInt32, math.MaxInt64, math.MaxInt32, 0x02, blockID2, defaultVoteTime)
			ev := NewDuplicateVoteEvidence(vote1, vote2, vote1.Timestamp)
			tc.malleateEvidence(ev)
			assert.Equal(t, tc.expectErr, ev.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestMockEvidenceValidateBasic(t *testing.T) {
	goodEvidence := NewMockDuplicateVoteEvidence(int64(1), time.Now(), "mock-chain-id")
	assert.Nil(t, goodEvidence.ValidateBasic())
}

func TestPotentialAmnesiaEvidence(t *testing.T) {
	const (
		chainID       = "TestPotentialAmnesiaEvidence"
		height  int64 = 37
	)

	var (
		val      = NewMockPV()
		val2     = NewMockPV()
		blockID  = makeBlockID(tmhash.Sum([]byte("blockhash")), math.MaxInt32, tmhash.Sum([]byte("partshash")))
		blockID2 = makeBlockID(tmhash.Sum([]byte("blockhash2")), math.MaxInt32, tmhash.Sum([]byte("partshash")))
		vote1    = makeVote(t, val, chainID, 0, height, 0, 2, blockID, defaultVoteTime)
		vote2    = makeVote(t, val, chainID, 0, height, 1, 2, blockID2, defaultVoteTime.Add(1*time.Second))
		vote3    = makeVote(t, val, chainID, 0, height, 2, 2, blockID, defaultVoteTime)
	)

	ev := NewPotentialAmnesiaEvidence(vote1, vote2, vote1.Timestamp)

	assert.Equal(t, height, ev.Height())
	assert.Equal(t, vote1.Timestamp, ev.Time())
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

	ev2 := &PotentialAmnesiaEvidence{
		VoteA:       vote1,
		VoteB:       vote2,
		HeightStamp: 5,
	}

	assert.True(t, ev.Equal(ev2))
	assert.Equal(t, ev.Hash(), ev2.Hash())

	ev3 := NewPotentialAmnesiaEvidence(vote2, vote1, vote1.Timestamp)
	assert.True(t, ev3.Equal(ev))

	ev4 := &PotentialAmnesiaEvidence{
		VoteA: vote3,
		VoteB: vote2,
	}

	assert.NoError(t, ev4.ValidateBasic())
	assert.NotEqual(t, ev.Hash(), ev4.Hash())
	assert.False(t, ev.Equal(ev4))

	// bad evidence
	badEv := []*PotentialAmnesiaEvidence{
		// first vote is for a later time than the second vote
		{
			VoteA: vote2,
			VoteB: vote1,
		},

		// votes are for the same round
		{
			VoteA: vote1,
			VoteB: makeVote(t, val, chainID, 0, height, 0, 2, blockID2, defaultVoteTime.Add(1*time.Second)),
		},

		// first vote was for a nil block - not locked
		{
			VoteA: makeVote(t, val, chainID, 0, height, 0, 2, BlockID{}, defaultVoteTime.Add(1*time.Second)),
			VoteB: vote2,
		},

		// second vote is from a different validator
		{
			VoteA: vote1,
			VoteB: makeVote(t, val2, chainID, 0, height, 1, 2, blockID2, defaultVoteTime.Add(1*time.Second)),
		},
	}

	for _, ev := range badEv {
		assert.Error(t, ev.ValidateBasic())
	}

}

func TestProofOfLockChange(t *testing.T) {
	const (
		chainID       = "test_chain_id"
		height  int64 = 37
	)
	// 1: valid POLC - nothing should fail
	voteSet, valSet, privValidators, blockID := buildVoteSet(height, 1, 3, 7, 0, tmproto.PrecommitType)
	pubKey, err := privValidators[7].GetPubKey()
	require.NoError(t, err)
	polc, err := NewPOLCFromVoteSet(voteSet, pubKey, blockID)
	assert.NoError(t, err)

	assert.Equal(t, height, polc.Height())
	assert.NoError(t, polc.ValidateBasic())
	assert.NoError(t, polc.ValidateVotes(valSet, chainID))
	assert.NotEmpty(t, polc.String())

	// tamper with one of the votes
	polc.Votes[0].Timestamp = time.Now().Add(1 * time.Second)
	err = polc.ValidateVotes(valSet, chainID)
	t.Log(err)
	assert.Error(t, err)

	// remove a vote such that majority wasn't reached
	polc.Votes = polc.Votes[1:]
	err = polc.ValidateVotes(valSet, chainID)
	t.Log(err)
	assert.Error(t, err)

	// test validate basic on a set of bad cases
	var badPOLCs []*ProofOfLockChange
	// 2: node has already voted in next round
	pubKey, err = privValidators[0].GetPubKey()
	require.NoError(t, err)
	polc2 := newPOLCFromVoteSet(voteSet, pubKey, blockID)
	badPOLCs = append(badPOLCs, polc2)
	// 3: one vote was from a different round
	voteSet, _, privValidators, blockID = buildVoteSet(height, 1, 3, 7, 0, tmproto.PrecommitType)
	pubKey, err = privValidators[7].GetPubKey()
	require.NoError(t, err)
	polc = newPOLCFromVoteSet(voteSet, pubKey, blockID)
	badVote := makeVote(t, privValidators[8], chainID, 8, height, 2, 2, blockID, defaultVoteTime)
	polc.Votes = append(polc.Votes, badVote)
	badPOLCs = append(badPOLCs, polc)
	// 4: one vote was from a different height
	polc = newPOLCFromVoteSet(voteSet, pubKey, blockID)
	badVote = makeVote(t, privValidators[8], chainID, 8, height+1, 1, 2, blockID, defaultVoteTime)
	polc.Votes = append(polc.Votes, badVote)
	badPOLCs = append(badPOLCs, polc)
	// 5: one vote was from a different vote type
	polc = newPOLCFromVoteSet(voteSet, pubKey, blockID)
	badVote = makeVote(t, privValidators[8], chainID, 8, height, 1, 1, blockID, defaultVoteTime)
	polc.Votes = append(polc.Votes, badVote)
	badPOLCs = append(badPOLCs, polc)
	// 5: one of the votes was for a nil block
	polc = newPOLCFromVoteSet(voteSet, pubKey, blockID)
	badVote = makeVote(t, privValidators[8], chainID, 8, height, 1, 2, BlockID{}, defaultVoteTime)
	polc.Votes = append(polc.Votes, badVote)
	badPOLCs = append(badPOLCs, polc)

	for idx, polc := range badPOLCs {
		err := polc.ValidateBasic()
		t.Logf("case: %d: %v", idx+2, err)
		assert.Error(t, err)
		if err == nil {
			t.Errorf("test no. %d failed", idx+2)
		}
	}

}

func TestAmnesiaEvidence(t *testing.T) {
	const (
		chainID       = "test_chain_id"
		height  int64 = 37
	)

	voteSet, valSet, privValidators, blockID := buildVoteSet(height, 1, 2, 7, 0, tmproto.PrecommitType)

	var (
		val       = privValidators[7]
		pubKey, _ = val.GetPubKey()
		blockID2  = makeBlockID(tmhash.Sum([]byte("blockhash2")), math.MaxInt32, tmhash.Sum([]byte("partshash")))
		vote1     = makeVote(t, val, chainID, 7, height, 0, 2, blockID2, time.Now())
		vote2     = makeVote(t, val, chainID, 7, height, 1, 2, blockID,
			time.Now().Add(time.Second))
		vote3 = makeVote(t, val, chainID, 7, height, 2, 2, blockID2, time.Now())
		polc  = newPOLCFromVoteSet(voteSet, pubKey, blockID)
	)

	require.False(t, polc.IsAbsent())

	pe := &PotentialAmnesiaEvidence{
		VoteA: vote1,
		VoteB: vote2,
	}

	emptyAmnesiaEvidence := NewAmnesiaEvidence(pe, NewEmptyPOLC())

	assert.NoError(t, emptyAmnesiaEvidence.ValidateBasic())
	violated, reason := emptyAmnesiaEvidence.ViolatedConsensus()
	if assert.True(t, violated) {
		assert.Equal(t, reason, "no proof of lock was provided")
	}
	assert.NoError(t, emptyAmnesiaEvidence.Verify(chainID, pubKey))

	completeAmnesiaEvidence := NewAmnesiaEvidence(pe, polc)

	assert.NoError(t, completeAmnesiaEvidence.ValidateBasic())
	violated, reason = completeAmnesiaEvidence.ViolatedConsensus()
	if !assert.False(t, violated) {
		t.Log(reason)
	}
	assert.NoError(t, completeAmnesiaEvidence.Verify(chainID, pubKey))
	assert.NoError(t, completeAmnesiaEvidence.Polc.ValidateVotes(valSet, chainID))

	assert.True(t, completeAmnesiaEvidence.Equal(emptyAmnesiaEvidence))
	assert.Equal(t, completeAmnesiaEvidence.Hash(), emptyAmnesiaEvidence.Hash())
	assert.NotEmpty(t, completeAmnesiaEvidence.Hash())
	assert.NotEmpty(t, completeAmnesiaEvidence.Bytes())

	pe2 := &PotentialAmnesiaEvidence{
		VoteA: vote3,
		VoteB: vote2,
	}

	// validator has incorrectly voted for a previous round after voting for a later round
	ae := NewAmnesiaEvidence(pe2, NewEmptyPOLC())
	assert.NoError(t, ae.ValidateBasic())
	violated, reason = ae.ViolatedConsensus()
	if assert.True(t, violated) {
		assert.Equal(t, reason, "validator went back and voted on a previous round")
	}

	var badAE []*AmnesiaEvidence
	// 1) Polc is at an incorrect height
	voteSet, _, _ = buildVoteSetForBlock(height+1, 1, 2, 7, 0, tmproto.PrecommitType, blockID)
	polc = newPOLCFromVoteSet(voteSet, pubKey, blockID)
	badAE = append(badAE, NewAmnesiaEvidence(pe, polc))
	// 2) Polc is of a later round
	voteSet, _, _ = buildVoteSetForBlock(height, 2, 2, 7, 0, tmproto.PrecommitType, blockID)
	polc = newPOLCFromVoteSet(voteSet, pubKey, blockID)
	badAE = append(badAE, NewAmnesiaEvidence(pe, polc))
	// 3) Polc has a different public key
	voteSet, _, privValidators = buildVoteSetForBlock(height, 1, 2, 7, 0, tmproto.PrecommitType, blockID)
	pubKey2, _ := privValidators[7].GetPubKey()
	polc = newPOLCFromVoteSet(voteSet, pubKey2, blockID)
	badAE = append(badAE, NewAmnesiaEvidence(pe, polc))
	// 4) Polc has a different block ID
	voteSet, _, _, blockID = buildVoteSet(height, 1, 2, 7, 0, tmproto.PrecommitType)
	polc = newPOLCFromVoteSet(voteSet, pubKey, blockID)
	badAE = append(badAE, NewAmnesiaEvidence(pe, polc))

	for idx, ae := range badAE {
		t.Log(ae.ValidateBasic())
		if !assert.Error(t, ae.ValidateBasic()) {
			t.Errorf("test no. %d failed", idx+1)
		}
	}

}

func makeVote(
	t *testing.T, val PrivValidator, chainID string, valIndex int32, height int64, round int32, step int, blockID BlockID,
	time time.Time) *Vote {
	pubKey, err := val.GetPubKey()
	require.NoError(t, err)
	v := &Vote{
		ValidatorAddress: pubKey.Address(),
		ValidatorIndex:   valIndex,
		Height:           height,
		Round:            round,
		Type:             tmproto.SignedMsgType(step),
		BlockID:          blockID,
		Timestamp:        time,
	}

	vpb := v.ToProto()
	err = val.SignVote(chainID, vpb)
	if err != nil {
		panic(err)
	}
	v.Signature = vpb.Signature
	return v
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

func TestEvidenceProto(t *testing.T) {
	// -------- Votes --------
	val := NewMockPV()
	blockID := makeBlockID(tmhash.Sum([]byte("blockhash")), math.MaxInt32, tmhash.Sum([]byte("partshash")))
	blockID2 := makeBlockID(tmhash.Sum([]byte("blockhash2")), math.MaxInt32, tmhash.Sum([]byte("partshash")))
	const chainID = "mychain"
	v := makeVote(t, val, chainID, math.MaxInt32, math.MaxInt64, 1, 0x01, blockID, defaultVoteTime)
	v2 := makeVote(t, val, chainID, math.MaxInt32, math.MaxInt64, 2, 0x01, blockID2, defaultVoteTime)

	// -------- SignedHeaders --------
	const height int64 = 37

	var (
		header1 = makeHeaderRandom()
		header2 = makeHeaderRandom()
	)

	header1.Height = height
	header1.LastBlockID = blockID
	header1.ChainID = chainID

	header2.Height = height
	header2.LastBlockID = blockID
	header2.ChainID = chainID

	tests := []struct {
		testName     string
		evidence     Evidence
		toProtoErr   bool
		fromProtoErr bool
	}{
		{"nil fail", nil, true, true},
		{"DuplicateVoteEvidence empty fail", &DuplicateVoteEvidence{}, false, true},
		{"DuplicateVoteEvidence nil voteB", &DuplicateVoteEvidence{VoteA: v, VoteB: nil}, false, true},
		{"DuplicateVoteEvidence nil voteA", &DuplicateVoteEvidence{VoteA: nil, VoteB: v}, false, true},
		{"DuplicateVoteEvidence success", &DuplicateVoteEvidence{VoteA: v2, VoteB: v}, false, false},
		{"PotentialAmnesiaEvidence empty fail", &PotentialAmnesiaEvidence{}, false, true},
		{"PotentialAmnesiaEvidence nil VoteB", &PotentialAmnesiaEvidence{VoteA: v, VoteB: nil}, false, true},
		{"PotentialAmnesiaEvidence nil VoteA", &PotentialAmnesiaEvidence{VoteA: nil, VoteB: v2}, false, true},
		{"PotentialAmnesiaEvidence success", &PotentialAmnesiaEvidence{VoteA: v2, VoteB: v}, false, false},
		{"AmnesiaEvidence nil ProofOfLockChange", &AmnesiaEvidence{PotentialAmnesiaEvidence: &PotentialAmnesiaEvidence{},
			Polc: NewEmptyPOLC()}, false, true},
		{"AmnesiaEvidence nil Polc",
			&AmnesiaEvidence{PotentialAmnesiaEvidence: &PotentialAmnesiaEvidence{VoteA: v2, VoteB: v},
				Polc: &ProofOfLockChange{}}, false, false},
		{"AmnesiaEvidence success", &AmnesiaEvidence{PotentialAmnesiaEvidence: &PotentialAmnesiaEvidence{VoteA: v2, VoteB: v},
			Polc: NewEmptyPOLC()}, false, false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			pb, err := EvidenceToProto(tt.evidence)
			if tt.toProtoErr {
				assert.Error(t, err, tt.testName)
				return
			}
			assert.NoError(t, err, tt.testName)

			evi, err := EvidenceFromProto(pb)
			if tt.fromProtoErr {
				assert.Error(t, err, tt.testName)
				return
			}
			require.Equal(t, tt.evidence, evi, tt.testName)
		})
	}
}

func TestProofOfLockChangeProtoBuf(t *testing.T) {
	// -------- Votes --------
	val := NewMockPV()
	val2 := NewMockPV()
	val3 := NewMockPV()
	blockID := makeBlockID(tmhash.Sum([]byte("blockhash")), math.MaxInt32, tmhash.Sum([]byte("partshash")))
	const chainID = "mychain"
	v := makeVote(t, val, chainID, math.MaxInt32, math.MaxInt64, 1, 0x01, blockID, defaultVoteTime)
	v2 := makeVote(t, val2, chainID, math.MaxInt32, math.MaxInt64, 1, 0x01, blockID, defaultVoteTime)

	testCases := []struct {
		msg          string
		polc         *ProofOfLockChange
		toProtoErr   bool
		fromProtoErr bool
	}{
		{"failure, empty key", &ProofOfLockChange{Votes: []*Vote{v, v2}, PubKey: nil}, true, false},
		{"failure, empty votes", &ProofOfLockChange{PubKey: val3.PrivKey.PubKey()}, true, false},
		{"success empty ProofOfLockChange", NewEmptyPOLC(), false, false},
		{"success", &ProofOfLockChange{Votes: []*Vote{v, v2}, PubKey: val3.PrivKey.PubKey()}, false, false},
	}
	for _, tc := range testCases {
		tc := tc
		pbpolc, err := tc.polc.ToProto()
		if tc.toProtoErr {
			assert.Error(t, err, tc.msg)
		} else {
			assert.NoError(t, err, tc.msg)
		}

		c, err := ProofOfLockChangeFromProto(pbpolc)
		if !tc.fromProtoErr {
			assert.NoError(t, err, tc.msg)
			if !tc.toProtoErr {
				assert.Equal(t, tc.polc, c, tc.msg)
			}
		} else {
			assert.Error(t, err, tc.msg)
		}
	}
}
