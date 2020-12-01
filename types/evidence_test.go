package types

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	"github.com/tendermint/tendermint/version"
)

var defaultVoteTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)

func TestEvidenceList(t *testing.T) {
	ev := randomDuplicateVoteEvidence(t)
	evl := EvidenceList([]Evidence{ev})

	assert.NotNil(t, evl.Hash())
	assert.True(t, evl.Has(ev))
	assert.False(t, evl.Has(&DuplicateVoteEvidence{}))
}

func randomDuplicateVoteEvidence(t *testing.T) *DuplicateVoteEvidence {
	val := NewMockPV()
	blockID := makeBlockID([]byte("blockhash"), 1000, []byte("partshash"))
	blockID2 := makeBlockID([]byte("blockhash2"), 1000, []byte("partshash"))
	const chainID = "mychain"
	return &DuplicateVoteEvidence{
		VoteA:            makeVote(t, val, chainID, 0, 10, 2, 1, blockID, defaultVoteTime),
		VoteB:            makeVote(t, val, chainID, 0, 10, 2, 1, blockID2, defaultVoteTime.Add(1*time.Minute)),
		TotalVotingPower: 30,
		ValidatorPower:   10,
		Timestamp:        defaultVoteTime,
	}
}

func TestDuplicateVoteEvidence(t *testing.T) {
	const height = int64(13)
	ev := NewMockDuplicateVoteEvidence(height, time.Now(), "mock-chain-id")
	assert.Equal(t, ev.Hash(), tmhash.Sum(ev.Bytes()))
	assert.NotNil(t, ev.String())
	assert.Equal(t, ev.Height(), height)
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
			valSet := NewValidatorSet([]*Validator{val.ExtractIntoValidator(10)})
			ev := NewDuplicateVoteEvidence(vote1, vote2, defaultVoteTime, valSet)
			tc.malleateEvidence(ev)
			assert.Equal(t, tc.expectErr, ev.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestLightClientAttackEvidence(t *testing.T) {
	height := int64(5)
	voteSet, valSet, privVals := randVoteSet(height, 1, tmproto.PrecommitType, 10, 1)
	header := makeHeaderRandom()
	header.Height = height
	blockID := makeBlockID(tmhash.Sum([]byte("blockhash")), math.MaxInt32, tmhash.Sum([]byte("partshash")))
	commit, err := MakeCommit(blockID, height, 1, voteSet, privVals, defaultVoteTime)
	require.NoError(t, err)
	lcae := &LightClientAttackEvidence{
		ConflictingBlock: &LightBlock{
			SignedHeader: &SignedHeader{
				Header: header,
				Commit: commit,
			},
			ValidatorSet: valSet,
		},
		CommonHeight: height - 1,
	}
	assert.NotNil(t, lcae.String())
	assert.NotNil(t, lcae.Hash())
	// only 7 validators sign
	differentCommit, err := MakeCommit(blockID, height, 1, voteSet, privVals[:7], defaultVoteTime)
	require.NoError(t, err)
	differentEv := &LightClientAttackEvidence{
		ConflictingBlock: &LightBlock{
			SignedHeader: &SignedHeader{
				Header: header,
				Commit: differentCommit,
			},
			ValidatorSet: valSet,
		},
		CommonHeight: height - 1,
	}
	assert.Equal(t, lcae.Hash(), differentEv.Hash())
	// different header hash
	differentHeader := makeHeaderRandom()
	differentEv = &LightClientAttackEvidence{
		ConflictingBlock: &LightBlock{
			SignedHeader: &SignedHeader{
				Header: differentHeader,
				Commit: differentCommit,
			},
			ValidatorSet: valSet,
		},
		CommonHeight: height - 1,
	}
	assert.NotEqual(t, lcae.Hash(), differentEv.Hash())
	// different common height should produce a different header
	differentEv = &LightClientAttackEvidence{
		ConflictingBlock: &LightBlock{
			SignedHeader: &SignedHeader{
				Header: header,
				Commit: differentCommit,
			},
			ValidatorSet: valSet,
		},
		CommonHeight: height - 2,
	}
	assert.NotEqual(t, lcae.Hash(), differentEv.Hash())
	assert.Equal(t, lcae.Height(), int64(4)) // Height should be the common Height
	assert.NotNil(t, lcae.Bytes())
}

func TestLightClientAttackEvidenceValidation(t *testing.T) {
	height := int64(5)
	voteSet, valSet, privVals := randVoteSet(height, 1, tmproto.PrecommitType, 10, 1)
	header := makeHeaderRandom()
	header.Height = height
	header.ValidatorsHash = valSet.Hash()
	blockID := makeBlockID(header.Hash(), math.MaxInt32, tmhash.Sum([]byte("partshash")))
	commit, err := MakeCommit(blockID, height, 1, voteSet, privVals, time.Now())
	require.NoError(t, err)
	lcae := &LightClientAttackEvidence{
		ConflictingBlock: &LightBlock{
			SignedHeader: &SignedHeader{
				Header: header,
				Commit: commit,
			},
			ValidatorSet: valSet,
		},
		CommonHeight: height - 1,
	}
	assert.NoError(t, lcae.ValidateBasic())

	testCases := []struct {
		testName         string
		malleateEvidence func(*LightClientAttackEvidence)
		expectErr        bool
	}{
		{"Good DuplicateVoteEvidence", func(ev *LightClientAttackEvidence) {}, false},
		{"Negative height", func(ev *LightClientAttackEvidence) { ev.CommonHeight = -10 }, true},
		{"Height is greater than divergent block", func(ev *LightClientAttackEvidence) {
			ev.CommonHeight = height + 1
		}, true},
		{"Nil conflicting header", func(ev *LightClientAttackEvidence) { ev.ConflictingBlock.Header = nil }, true},
		{"Nil conflicting blocl", func(ev *LightClientAttackEvidence) { ev.ConflictingBlock = nil }, true},
		{"Nil validator set", func(ev *LightClientAttackEvidence) {
			ev.ConflictingBlock.ValidatorSet = &ValidatorSet{}
		}, true},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			lcae := &LightClientAttackEvidence{
				ConflictingBlock: &LightBlock{
					SignedHeader: &SignedHeader{
						Header: header,
						Commit: commit,
					},
					ValidatorSet: valSet,
				},
				CommonHeight: height - 1,
			}
			tc.malleateEvidence(lcae)
			if tc.expectErr {
				assert.Error(t, lcae.ValidateBasic(), tc.testName)
			} else {
				assert.NoError(t, lcae.ValidateBasic(), tc.testName)
			}
		})
	}

}

func TestMockEvidenceValidateBasic(t *testing.T) {
	goodEvidence := NewMockDuplicateVoteEvidence(int64(1), time.Now(), "mock-chain-id")
	assert.Nil(t, goodEvidence.ValidateBasic())
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
		Version:            tmversion.Consensus{Block: version.BlockProtocol, App: 1},
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
