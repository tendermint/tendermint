package evidence_test

import (
	"context"
	"encoding/hex"
	"math"
	mrand "math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/tmhash"
	test "github.com/tendermint/tendermint/internal/test/factory"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/pkg/consensus"
	"github.com/tendermint/tendermint/pkg/evidence"
	"github.com/tendermint/tendermint/pkg/light"
	"github.com/tendermint/tendermint/pkg/meta"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

var defaultVoteTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)

func TestEvidenceList(t *testing.T) {
	ev := randomDuplicateVoteEvidence(t)
	evl := evidence.EvidenceList([]evidence.Evidence{ev})

	assert.NotNil(t, evl.Hash())
	assert.True(t, evl.Has(ev))
	assert.False(t, evl.Has(&evidence.DuplicateVoteEvidence{}))
}

func randomDuplicateVoteEvidence(t *testing.T) *evidence.DuplicateVoteEvidence {
	val := consensus.NewMockPV()
	blockID := test.MakeBlockIDWithHash([]byte("blockhash"))
	blockID2 := test.MakeBlockIDWithHash([]byte("blockhash2"))
	const chainID = "mychain"
	return &evidence.DuplicateVoteEvidence{
		VoteA:            makeVote(t, val, chainID, 0, 10, 2, 1, blockID, defaultVoteTime),
		VoteB:            makeVote(t, val, chainID, 0, 10, 2, 1, blockID2, defaultVoteTime.Add(1*time.Minute)),
		TotalVotingPower: 30,
		ValidatorPower:   10,
		Timestamp:        defaultVoteTime,
	}
}

func TestDuplicateVoteEvidence(t *testing.T) {
	const height = int64(13)
	ev := evidence.NewMockDuplicateVoteEvidence(height, time.Now(), "mock-chain-id")
	assert.Equal(t, ev.Hash(), tmhash.Sum(ev.Bytes()))
	assert.NotNil(t, ev.String())
	assert.Equal(t, ev.Height(), height)
}

func TestDuplicateVoteEvidenceValidation(t *testing.T) {
	val := consensus.NewMockPV()
	blockID := test.MakeBlockIDWithHash(tmhash.Sum([]byte("blockhash")))
	blockID2 := test.MakeBlockIDWithHash(tmhash.Sum([]byte("blockhash2")))
	const chainID = "mychain"

	testCases := []struct {
		testName         string
		malleateEvidence func(*evidence.DuplicateVoteEvidence)
		expectErr        bool
	}{
		{"Good DuplicateVoteEvidence", func(ev *evidence.DuplicateVoteEvidence) {}, false},
		{"Nil vote A", func(ev *evidence.DuplicateVoteEvidence) { ev.VoteA = nil }, true},
		{"Nil vote B", func(ev *evidence.DuplicateVoteEvidence) { ev.VoteB = nil }, true},
		{"Nil votes", func(ev *evidence.DuplicateVoteEvidence) {
			ev.VoteA = nil
			ev.VoteB = nil
		}, true},
		{"Invalid vote type", func(ev *evidence.DuplicateVoteEvidence) {
			ev.VoteA = makeVote(t, val, chainID, math.MaxInt32, math.MaxInt64, math.MaxInt32, 0, blockID2, defaultVoteTime)
		}, true},
		{"Invalid vote order", func(ev *evidence.DuplicateVoteEvidence) {
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
			valSet := consensus.NewValidatorSet([]*consensus.Validator{val.ExtractIntoValidator(10)})
			ev := evidence.NewDuplicateVoteEvidence(vote1, vote2, defaultVoteTime, valSet)
			tc.malleateEvidence(ev)
			assert.Equal(t, tc.expectErr, ev.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestLightClientAttackEvidenceBasic(t *testing.T) {
	height := int64(5)
	commonHeight := height - 1
	nValidators := 10
	voteSet, valSet, privVals := test.RandVoteSet(height, 1, tmproto.PrecommitType, nValidators, 1)
	header := makeHeaderRandom()
	header.Height = height
	blockID := test.MakeBlockIDWithHash(tmhash.Sum([]byte("blockhash")))
	commit, err := test.MakeCommit(blockID, height, 1, voteSet, privVals, defaultVoteTime)
	require.NoError(t, err)
	lcae := &evidence.LightClientAttackEvidence{
		ConflictingBlock: &light.LightBlock{
			SignedHeader: &light.SignedHeader{
				Header: header,
				Commit: commit,
			},
			ValidatorSet: valSet,
		},
		CommonHeight:        commonHeight,
		TotalVotingPower:    valSet.TotalVotingPower(),
		Timestamp:           header.Time,
		ByzantineValidators: valSet.Validators[:nValidators/2],
	}
	assert.NotNil(t, lcae.String())
	assert.NotNil(t, lcae.Hash())
	assert.Equal(t, lcae.Height(), commonHeight) // Height should be the common Height
	assert.NotNil(t, lcae.Bytes())

	// maleate evidence to test hash uniqueness
	testCases := []struct {
		testName         string
		malleateEvidence func(*evidence.LightClientAttackEvidence)
	}{
		{"Different header", func(ev *evidence.LightClientAttackEvidence) { ev.ConflictingBlock.Header = makeHeaderRandom() }},
		{"Different common height", func(ev *evidence.LightClientAttackEvidence) {
			ev.CommonHeight = height + 1
		}},
	}

	for _, tc := range testCases {
		lcae := &evidence.LightClientAttackEvidence{
			ConflictingBlock: &light.LightBlock{
				SignedHeader: &light.SignedHeader{
					Header: header,
					Commit: commit,
				},
				ValidatorSet: valSet,
			},
			CommonHeight:        commonHeight,
			TotalVotingPower:    valSet.TotalVotingPower(),
			Timestamp:           header.Time,
			ByzantineValidators: valSet.Validators[:nValidators/2],
		}
		hash := lcae.Hash()
		tc.malleateEvidence(lcae)
		assert.NotEqual(t, hash, lcae.Hash(), tc.testName)
	}
}

func TestLightClientAttackEvidenceValidation(t *testing.T) {
	height := int64(5)
	commonHeight := height - 1
	nValidators := 10
	voteSet, valSet, privVals := test.RandVoteSet(height, 1, tmproto.PrecommitType, nValidators, 1)
	header := makeHeaderRandom()
	header.Height = height
	header.ValidatorsHash = valSet.Hash()
	blockID := test.MakeBlockIDWithHash(header.Hash())
	commit, err := test.MakeCommit(blockID, height, 1, voteSet, privVals, time.Now())
	require.NoError(t, err)
	lcae := &evidence.LightClientAttackEvidence{
		ConflictingBlock: &light.LightBlock{
			SignedHeader: &light.SignedHeader{
				Header: header,
				Commit: commit,
			},
			ValidatorSet: valSet,
		},
		CommonHeight:        commonHeight,
		TotalVotingPower:    valSet.TotalVotingPower(),
		Timestamp:           header.Time,
		ByzantineValidators: valSet.Validators[:nValidators/2],
	}
	assert.NoError(t, lcae.ValidateBasic())

	testCases := []struct {
		testName         string
		malleateEvidence func(*evidence.LightClientAttackEvidence)
		expectErr        bool
	}{
		{"Good LightClientAttackEvidence", func(ev *evidence.LightClientAttackEvidence) {}, false},
		{"Negative height", func(ev *evidence.LightClientAttackEvidence) { ev.CommonHeight = -10 }, true},
		{"Height is greater than divergent block", func(ev *evidence.LightClientAttackEvidence) {
			ev.CommonHeight = height + 1
		}, true},
		{"Height is equal to the divergent block", func(ev *evidence.LightClientAttackEvidence) {
			ev.CommonHeight = height
		}, false},
		{"Nil conflicting header", func(ev *evidence.LightClientAttackEvidence) { ev.ConflictingBlock.Header = nil }, true},
		{"Nil conflicting blocl", func(ev *evidence.LightClientAttackEvidence) { ev.ConflictingBlock = nil }, true},
		{"Nil validator set", func(ev *evidence.LightClientAttackEvidence) {
			ev.ConflictingBlock.ValidatorSet = &consensus.ValidatorSet{}
		}, true},
		{"Negative total voting power", func(ev *evidence.LightClientAttackEvidence) {
			ev.TotalVotingPower = -1
		}, true},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			lcae := &evidence.LightClientAttackEvidence{
				ConflictingBlock: &light.LightBlock{
					SignedHeader: &light.SignedHeader{
						Header: header,
						Commit: commit,
					},
					ValidatorSet: valSet,
				},
				CommonHeight:        commonHeight,
				TotalVotingPower:    valSet.TotalVotingPower(),
				Timestamp:           header.Time,
				ByzantineValidators: valSet.Validators[:nValidators/2],
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
	goodEvidence := evidence.NewMockDuplicateVoteEvidence(int64(1), time.Now(), "mock-chain-id")
	assert.Nil(t, goodEvidence.ValidateBasic())
}

func makeVote(
	t *testing.T, val consensus.PrivValidator, chainID string, valIndex int32, height int64, round int32, step int, blockID meta.BlockID,
	time time.Time) *consensus.Vote {
	pubKey, err := val.GetPubKey(context.Background())
	require.NoError(t, err)
	v := &consensus.Vote{
		ValidatorAddress: pubKey.Address(),
		ValidatorIndex:   valIndex,
		Height:           height,
		Round:            round,
		Type:             tmproto.SignedMsgType(step),
		BlockID:          blockID,
		Timestamp:        time,
	}

	vpb := v.ToProto()
	err = val.SignVote(context.Background(), chainID, vpb)
	if err != nil {
		panic(err)
	}
	v.Signature = vpb.Signature
	return v
}

func makeHeaderRandom() *meta.Header {
	return &meta.Header{
		Version:            version.Consensus{Block: version.BlockProtocol, App: 1},
		ChainID:            tmrand.Str(12),
		Height:             int64(mrand.Uint32() + 1),
		Time:               time.Now(),
		LastBlockID:        test.MakeBlockID(),
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
	val := consensus.NewMockPV()
	blockID := test.MakeBlockIDWithHash(tmhash.Sum([]byte("blockhash")))
	blockID2 := test.MakeBlockIDWithHash(tmhash.Sum([]byte("blockhash2")))
	const chainID = "mychain"
	v := makeVote(t, val, chainID, math.MaxInt32, math.MaxInt64, 1, 0x01, blockID, defaultVoteTime)
	v2 := makeVote(t, val, chainID, math.MaxInt32, math.MaxInt64, 2, 0x01, blockID2, defaultVoteTime)

	tests := []struct {
		testName     string
		evidence     evidence.Evidence
		toProtoErr   bool
		fromProtoErr bool
	}{
		{"nil fail", nil, true, true},
		{"DuplicateVoteEvidence empty fail", &evidence.DuplicateVoteEvidence{}, false, true},
		{"DuplicateVoteEvidence nil voteB", &evidence.DuplicateVoteEvidence{VoteA: v, VoteB: nil}, false, true},
		{"DuplicateVoteEvidence nil voteA", &evidence.DuplicateVoteEvidence{VoteA: nil, VoteB: v}, false, true},
		{"DuplicateVoteEvidence success", &evidence.DuplicateVoteEvidence{VoteA: v2, VoteB: v}, false, false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			pb, err := evidence.EvidenceToProto(tt.evidence)
			if tt.toProtoErr {
				assert.Error(t, err, tt.testName)
				return
			}
			assert.NoError(t, err, tt.testName)

			evi, err := evidence.EvidenceFromProto(pb)
			if tt.fromProtoErr {
				assert.Error(t, err, tt.testName)
				return
			}
			require.Equal(t, tt.evidence, evi, tt.testName)
		})
	}
}

func TestEvidenceVectors(t *testing.T) {
	// Votes for duplicateEvidence
	val := consensus.NewMockPV()
	val.PrivKey = ed25519.GenPrivKeyFromSecret([]byte("it's a secret")) // deterministic key
	blockID := test.MakeBlockIDWithHash(tmhash.Sum([]byte("blockhash")))
	blockID2 := test.MakeBlockIDWithHash(tmhash.Sum([]byte("blockhash2")))
	const chainID = "mychain"
	v := makeVote(t, val, chainID, math.MaxInt32, math.MaxInt64, 1, 0x01, blockID, defaultVoteTime)
	v2 := makeVote(t, val, chainID, math.MaxInt32, math.MaxInt64, 2, 0x01, blockID2, defaultVoteTime)

	// Data for LightClientAttackEvidence
	height := int64(5)
	commonHeight := height - 1
	nValidators := 10
	voteSet, valSet, privVals := test.DeterministicVoteSet(height, 1, tmproto.PrecommitType, 1)
	header := &meta.Header{
		Version:            version.Consensus{Block: 1, App: 1},
		ChainID:            chainID,
		Height:             height,
		Time:               time.Date(math.MaxInt64, 0, 0, 0, 0, 0, math.MaxInt64, time.UTC),
		LastBlockID:        meta.BlockID{},
		LastCommitHash:     []byte("f2564c78071e26643ae9b3e2a19fa0dc10d4d9e873aa0be808660123f11a1e78"),
		DataHash:           []byte("f2564c78071e26643ae9b3e2a19fa0dc10d4d9e873aa0be808660123f11a1e78"),
		ValidatorsHash:     valSet.Hash(),
		NextValidatorsHash: []byte("f2564c78071e26643ae9b3e2a19fa0dc10d4d9e873aa0be808660123f11a1e78"),
		ConsensusHash:      []byte("f2564c78071e26643ae9b3e2a19fa0dc10d4d9e873aa0be808660123f11a1e78"),
		AppHash:            []byte("f2564c78071e26643ae9b3e2a19fa0dc10d4d9e873aa0be808660123f11a1e78"),

		LastResultsHash: []byte("f2564c78071e26643ae9b3e2a19fa0dc10d4d9e873aa0be808660123f11a1e78"),

		EvidenceHash:    []byte("f2564c78071e26643ae9b3e2a19fa0dc10d4d9e873aa0be808660123f11a1e78"),
		ProposerAddress: []byte("2915b7b15f979e48ebc61774bb1d86ba3136b7eb"),
	}
	blockID3 := test.MakeBlockIDWithHash(header.Hash())
	commit, err := test.MakeCommit(blockID3, height, 1, voteSet, privVals, defaultVoteTime)
	require.NoError(t, err)
	lcae := &evidence.LightClientAttackEvidence{
		ConflictingBlock: &light.LightBlock{
			SignedHeader: &light.SignedHeader{
				Header: header,
				Commit: commit,
			},
			ValidatorSet: valSet,
		},
		CommonHeight:        commonHeight,
		TotalVotingPower:    valSet.TotalVotingPower(),
		Timestamp:           header.Time,
		ByzantineValidators: valSet.Validators[:nValidators/2],
	}
	// assert.NoError(t, lcae.ValidateBasic())

	testCases := []struct {
		testName string
		evList   evidence.EvidenceList
		expBytes string
	}{
		{"duplicateVoteEvidence",
			evidence.EvidenceList{&evidence.DuplicateVoteEvidence{VoteA: v2, VoteB: v}},
			"a9ce28d13bb31001fc3e5b7927051baf98f86abdbd64377643a304164c826923",
		},
		{"LightClientAttackEvidence",
			evidence.EvidenceList{lcae},
			"2f8782163c3905b26e65823ababc977fe54e97b94e60c0360b1e4726b668bb8e",
		},
		{"LightClientAttackEvidence & DuplicateVoteEvidence",
			evidence.EvidenceList{&evidence.DuplicateVoteEvidence{VoteA: v2, VoteB: v}, lcae},
			"eedb4b47d6dbc9d43f53da8aa50bb826e8d9fc7d897da777c8af6a04aa74163e",
		},
	}

	for _, tc := range testCases {
		tc := tc
		hash := tc.evList.Hash()
		require.Equal(t, tc.expBytes, hex.EncodeToString(hash), tc.testName)
	}
}
