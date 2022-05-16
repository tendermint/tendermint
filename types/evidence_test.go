package types

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
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

var defaultVoteTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)

func TestEvidenceList(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ev := randomDuplicateVoteEvidence(ctx, t)
	evl := EvidenceList([]Evidence{ev})

	assert.NotNil(t, evl.Hash())
	assert.True(t, evl.Has(ev))
	assert.False(t, evl.Has(&DuplicateVoteEvidence{}))
}

// TestEvidenceListProtoBuf to ensure parity in protobuf output and input
func TestEvidenceListProtoBuf(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const chainID = "mychain"
	ev, err := NewMockDuplicateVoteEvidence(ctx, math.MaxInt64, time.Now(), chainID)
	require.NoError(t, err)
	data := EvidenceList{ev}
	testCases := []struct {
		msg      string
		data1    *EvidenceList
		expPass1 bool
		expPass2 bool
	}{
		{"success", &data, true, true},
		{"empty evidenceData", &EvidenceList{}, true, true},
		{"fail nil Data", nil, false, false},
	}

	for _, tc := range testCases {
		protoData, err := tc.data1.ToProto()
		if tc.expPass1 {
			require.NoError(t, err, tc.msg)
		} else {
			require.Error(t, err, tc.msg)
		}

		eviD := new(EvidenceList)
		err = eviD.FromProto(protoData)
		if tc.expPass2 {
			require.NoError(t, err, tc.msg)
			require.Equal(t, tc.data1, eviD, tc.msg)
		} else {
			require.Error(t, err, tc.msg)
		}
	}
}
func randomDuplicateVoteEvidence(ctx context.Context, t *testing.T) *DuplicateVoteEvidence {
	t.Helper()
	val := NewMockPV()
	blockID := makeBlockID([]byte("blockhash"), 1000, []byte("partshash"))
	blockID2 := makeBlockID([]byte("blockhash2"), 1000, []byte("partshash"))
	const chainID = "mychain"
	return &DuplicateVoteEvidence{
		VoteA:            makeVote(ctx, t, val, chainID, 0, 10, 2, 1, blockID, defaultVoteTime),
		VoteB:            makeVote(ctx, t, val, chainID, 0, 10, 2, 1, blockID2, defaultVoteTime.Add(1*time.Minute)),
		TotalVotingPower: 30,
		ValidatorPower:   10,
		Timestamp:        defaultVoteTime,
	}
}

func TestDuplicateVoteEvidence(t *testing.T) {
	const height = int64(13)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ev, err := NewMockDuplicateVoteEvidence(ctx, height, time.Now(), "mock-chain-id")
	require.NoError(t, err)
	assert.Equal(t, ev.Hash(), crypto.Checksum(ev.Bytes()))
	assert.NotNil(t, ev.String())
	assert.Equal(t, ev.Height(), height)
}

func TestDuplicateVoteEvidenceValidation(t *testing.T) {
	val := NewMockPV()
	blockID := makeBlockID(crypto.Checksum([]byte("blockhash")), math.MaxInt32, crypto.Checksum([]byte("partshash")))
	blockID2 := makeBlockID(crypto.Checksum([]byte("blockhash2")), math.MaxInt32, crypto.Checksum([]byte("partshash")))
	const chainID = "mychain"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
			ev.VoteA = makeVote(ctx, t, val, chainID, math.MaxInt32, math.MaxInt64, math.MaxInt32, 0, blockID2, defaultVoteTime)
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
			vote1 := makeVote(ctx, t, val, chainID, math.MaxInt32, math.MaxInt64, math.MaxInt32, 0x02, blockID, defaultVoteTime)
			vote2 := makeVote(ctx, t, val, chainID, math.MaxInt32, math.MaxInt64, math.MaxInt32, 0x02, blockID2, defaultVoteTime)
			valSet := NewValidatorSet([]*Validator{val.ExtractIntoValidator(ctx, 10)})
			ev, err := NewDuplicateVoteEvidence(vote1, vote2, defaultVoteTime, valSet)
			require.NoError(t, err)
			tc.malleateEvidence(ev)
			assert.Equal(t, tc.expectErr, ev.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestLightClientAttackEvidenceBasic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	height := int64(5)
	commonHeight := height - 1
	nValidators := 10
	voteSet, valSet, privVals := randVoteSet(ctx, t, height, 1, tmproto.PrecommitType, nValidators, 1)

	header := makeHeaderRandom()
	header.Height = height
	blockID := makeBlockID(crypto.Checksum([]byte("blockhash")), math.MaxInt32, crypto.Checksum([]byte("partshash")))
	extCommit, err := makeExtCommit(ctx, blockID, height, 1, voteSet, privVals, defaultVoteTime)
	require.NoError(t, err)
	commit := extCommit.ToCommit()

	lcae := &LightClientAttackEvidence{
		ConflictingBlock: &LightBlock{
			SignedHeader: &SignedHeader{
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
		malleateEvidence func(*LightClientAttackEvidence)
	}{
		{"Different header", func(ev *LightClientAttackEvidence) { ev.ConflictingBlock.Header = makeHeaderRandom() }},
		{"Different common height", func(ev *LightClientAttackEvidence) {
			ev.CommonHeight = height + 1
		}},
	}

	for _, tc := range testCases {
		lcae := &LightClientAttackEvidence{
			ConflictingBlock: &LightBlock{
				SignedHeader: &SignedHeader{
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	height := int64(5)
	commonHeight := height - 1
	nValidators := 10
	voteSet, valSet, privVals := randVoteSet(ctx, t, height, 1, tmproto.PrecommitType, nValidators, 1)

	header := makeHeaderRandom()
	header.Height = height
	header.ValidatorsHash = valSet.Hash()
	blockID := makeBlockID(header.Hash(), math.MaxInt32, crypto.Checksum([]byte("partshash")))
	extCommit, err := makeExtCommit(ctx, blockID, height, 1, voteSet, privVals, time.Now())
	require.NoError(t, err)
	commit := extCommit.ToCommit()

	lcae := &LightClientAttackEvidence{
		ConflictingBlock: &LightBlock{
			SignedHeader: &SignedHeader{
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
		malleateEvidence func(*LightClientAttackEvidence)
		expectErr        bool
	}{
		{"Good LightClientAttackEvidence", func(ev *LightClientAttackEvidence) {}, false},
		{"Negative height", func(ev *LightClientAttackEvidence) { ev.CommonHeight = -10 }, true},
		{"Height is greater than divergent block", func(ev *LightClientAttackEvidence) {
			ev.CommonHeight = height + 1
		}, true},
		{"Height is equal to the divergent block", func(ev *LightClientAttackEvidence) {
			ev.CommonHeight = height
		}, false},
		{"Nil conflicting header", func(ev *LightClientAttackEvidence) { ev.ConflictingBlock.Header = nil }, true},
		{"Nil conflicting blocl", func(ev *LightClientAttackEvidence) { ev.ConflictingBlock = nil }, true},
		{"Nil validator set", func(ev *LightClientAttackEvidence) {
			ev.ConflictingBlock.ValidatorSet = &ValidatorSet{}
		}, true},
		{"Negative total voting power", func(ev *LightClientAttackEvidence) {
			ev.TotalVotingPower = -1
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	goodEvidence, err := NewMockDuplicateVoteEvidence(ctx, int64(1), time.Now(), "mock-chain-id")
	require.NoError(t, err)
	assert.Nil(t, goodEvidence.ValidateBasic())
}

func makeVote(
	ctx context.Context,
	t *testing.T,
	val PrivValidator,
	chainID string,
	valIndex int32,
	height int64,
	round int32,
	step int,
	blockID BlockID,
	time time.Time,
) *Vote {
	pubKey, err := val.GetPubKey(ctx)
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
	err = val.SignVote(ctx, chainID, vpb)
	require.NoError(t, err)

	v.Signature = vpb.Signature
	return v
}

func makeHeaderRandom() *Header {
	return &Header{
		Version:            version.Consensus{Block: version.BlockProtocol, App: 1},
		ChainID:            tmrand.Str(12),
		Height:             int64(mrand.Uint32() + 1),
		Time:               time.Now(),
		LastBlockID:        makeBlockIDRandom(),
		LastCommitHash:     crypto.CRandBytes(crypto.HashSize),
		DataHash:           crypto.CRandBytes(crypto.HashSize),
		ValidatorsHash:     crypto.CRandBytes(crypto.HashSize),
		NextValidatorsHash: crypto.CRandBytes(crypto.HashSize),
		ConsensusHash:      crypto.CRandBytes(crypto.HashSize),
		AppHash:            crypto.CRandBytes(crypto.HashSize),
		LastResultsHash:    crypto.CRandBytes(crypto.HashSize),
		EvidenceHash:       crypto.CRandBytes(crypto.HashSize),
		ProposerAddress:    crypto.CRandBytes(crypto.AddressSize),
	}
}

func TestEvidenceProto(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// -------- Votes --------
	val := NewMockPV()
	blockID := makeBlockID(crypto.Checksum([]byte("blockhash")), math.MaxInt32, crypto.Checksum([]byte("partshash")))
	blockID2 := makeBlockID(crypto.Checksum([]byte("blockhash2")), math.MaxInt32, crypto.Checksum([]byte("partshash")))
	const chainID = "mychain"
	v := makeVote(ctx, t, val, chainID, math.MaxInt32, math.MaxInt64, 1, 0x01, blockID, defaultVoteTime)
	v2 := makeVote(ctx, t, val, chainID, math.MaxInt32, math.MaxInt64, 2, 0x01, blockID2, defaultVoteTime)

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

func TestEvidenceVectors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Votes for duplicateEvidence
	val := NewMockPV()
	val.PrivKey = ed25519.GenPrivKeyFromSecret([]byte("it's a secret")) // deterministic key
	blockID := makeBlockID(crypto.Checksum([]byte("blockhash")), math.MaxInt32, crypto.Checksum([]byte("partshash")))
	blockID2 := makeBlockID(crypto.Checksum([]byte("blockhash2")), math.MaxInt32, crypto.Checksum([]byte("partshash")))
	const chainID = "mychain"
	v := makeVote(ctx, t, val, chainID, math.MaxInt32, math.MaxInt64, 1, 0x01, blockID, defaultVoteTime)
	v2 := makeVote(ctx, t, val, chainID, math.MaxInt32, math.MaxInt64, 2, 0x01, blockID2, defaultVoteTime)

	// Data for LightClientAttackEvidence
	height := int64(5)
	commonHeight := height - 1
	nValidators := 10
	voteSet, valSet, privVals := deterministicVoteSet(ctx, t, height, 1, tmproto.PrecommitType, 1)
	header := &Header{
		Version:            version.Consensus{Block: 1, App: 1},
		ChainID:            chainID,
		Height:             height,
		Time:               time.Date(math.MaxInt64, 0, 0, 0, 0, 0, math.MaxInt64, time.UTC),
		LastBlockID:        BlockID{},
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
	blockID3 := makeBlockID(header.Hash(), math.MaxInt32, crypto.Checksum([]byte("partshash")))
	extCommit, err := makeExtCommit(ctx, blockID3, height, 1, voteSet, privVals, defaultVoteTime)
	require.NoError(t, err)
	lcae := &LightClientAttackEvidence{
		ConflictingBlock: &LightBlock{
			SignedHeader: &SignedHeader{
				Header: header,
				Commit: extCommit.ToCommit(),
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
		evList   EvidenceList
		expBytes string
	}{
		{"duplicateVoteEvidence",
			EvidenceList{&DuplicateVoteEvidence{VoteA: v2, VoteB: v}},
			"a9ce28d13bb31001fc3e5b7927051baf98f86abdbd64377643a304164c826923",
		},
		{"LightClientAttackEvidence",
			EvidenceList{lcae},
			"2f8782163c3905b26e65823ababc977fe54e97b94e60c0360b1e4726b668bb8e",
		},
		{"LightClientAttackEvidence & DuplicateVoteEvidence",
			EvidenceList{&DuplicateVoteEvidence{VoteA: v2, VoteB: v}, lcae},
			"eedb4b47d6dbc9d43f53da8aa50bb826e8d9fc7d897da777c8af6a04aa74163e",
		},
	}

	for _, tc := range testCases {
		tc := tc
		hash := tc.evList.Hash()
		require.Equal(t, tc.expBytes, hex.EncodeToString(hash), tc.testName)
	}
}
