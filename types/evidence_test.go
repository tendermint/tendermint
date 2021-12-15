package types

import (
	"context"
	"encoding/hex"
	"github.com/tendermint/tendermint/crypto/bls12381"
	"math"
	mrand "math/rand"
	"testing"
	"time"

	"github.com/dashevo/dashd-go/btcjson"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
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
	quorumHash := crypto.RandQuorumHash()
	val := NewMockPVForQuorum(quorumHash)
	blockID := makeBlockID([]byte("blockhash"), 1000, []byte("partshash"))
	blockID2 := makeBlockID([]byte("blockhash2"), 1000, []byte("partshash"))
	quorumType := btcjson.LLMQType_5_60
	const chainID = "mychain"
	const height = int64(10)
	stateID := RandStateID().WithHeight(height - 1)
	return &DuplicateVoteEvidence{

		VoteA:            makeVote(t, val, chainID, 0, height, 2, 1, quorumType, quorumHash, blockID, stateID),
		VoteB:            makeVote(t, val, chainID, 0, height, 2, 1, quorumType, quorumHash, blockID2, stateID),
		TotalVotingPower: 3 * DefaultDashVotingPower,
		ValidatorPower:   DefaultDashVotingPower,
		Timestamp:        defaultVoteTime,
	}
}

func TestDuplicateVoteEvidence(t *testing.T) {
	const height = int64(13)
	quorumType := btcjson.LLMQType_5_60
	ev, err := NewMockDuplicateVoteEvidence(height, time.Now(), "mock-chain-id", quorumType, crypto.RandQuorumHash())
	assert.NoError(t, err)
	assert.Equal(t, ev.Hash(), tmhash.Sum(ev.Bytes()))
	assert.NotNil(t, ev.String())
	assert.Equal(t, ev.Height(), height)
}

func TestDuplicateVoteEvidenceValidation(t *testing.T) {
	quorumHash := crypto.RandQuorumHash()
	val := NewMockPVForQuorum(quorumHash)
	blockID := makeBlockID(tmhash.Sum([]byte("blockhash")), math.MaxInt32, tmhash.Sum([]byte("partshash")))
	blockID2 := makeBlockID(tmhash.Sum([]byte("blockhash2")), math.MaxInt32, tmhash.Sum([]byte("partshash")))
	quorumType := btcjson.LLMQType_5_60
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
			ev.VoteA = makeVote(
				t, val, chainID, math.MaxInt32, math.MaxInt64, math.MaxInt32, 0, quorumType,
				quorumHash, blockID2, RandStateID().WithHeight(math.MaxInt64-1),
			)
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
			const height int64 = math.MaxInt64
			stateID := RandStateID().WithHeight(height - 1)
			vote1 := makeVote(t, val, chainID, math.MaxInt32, height, math.MaxInt32, 0x02, quorumType,
				quorumHash, blockID, stateID)
			vote2 := makeVote(t, val, chainID, math.MaxInt32, height, math.MaxInt32, 0x02, quorumType,
				quorumHash, blockID2, stateID)
			thresholdPublicKey, err := val.GetThresholdPublicKey(context.Background(), quorumHash)
			assert.NoError(t, err)
			valSet := NewValidatorSet(
				[]*Validator{val.ExtractIntoValidator(context.Background(), quorumHash)}, thresholdPublicKey, quorumType, quorumHash, true)
			ev, err := NewDuplicateVoteEvidence(vote1, vote2, defaultVoteTime, valSet)
			require.NoError(t, err)
			tc.malleateEvidence(ev)
			assert.Equal(t, tc.expectErr, ev.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestMockEvidenceValidateBasic(t *testing.T) {
	goodEvidence, err := NewMockDuplicateVoteEvidence(int64(1), time.Now(), "mock-chain-id", btcjson.LLMQType_5_60,
		crypto.RandQuorumHash())
	assert.NoError(t, err)
	assert.Nil(t, goodEvidence.ValidateBasic())
}

func makeVote(
	t *testing.T, val PrivValidator, chainID string,
	valIndex int32, height int64, round int32, step int, quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash, blockID BlockID, stateID StateID,
) *Vote {
	proTxHash, err := val.GetProTxHash(context.Background())
	require.NoError(t, err)
	v := &Vote{
		ValidatorProTxHash: proTxHash,
		ValidatorIndex:     valIndex,
		Height:             height,
		Round:              round,
		Type:               tmproto.SignedMsgType(step),
		BlockID:            blockID,
	}

	vpb := v.ToProto()
	err = val.SignVote(context.Background(), chainID, quorumType, quorumHash, vpb, stateID, nil)
	if err != nil {
		panic(err)
	}
	v.BlockSignature = vpb.BlockSignature
	v.StateSignature = vpb.StateSignature
	return v
}

func makeHeaderRandom() *Header {
	return &Header{
		Version:            version.Consensus{Block: version.BlockProtocol, App: 1},
		ChainID:            tmrand.Str(12),
		Height:             int64(mrand.Uint32() + 1),
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
		ProposerProTxHash:  crypto.CRandBytes(crypto.DefaultHashSize),
	}
}

func TestEvidenceProto(t *testing.T) {
	// -------- Votes --------
	quorumHash := crypto.RandQuorumHash()
	val := NewMockPVForQuorum(quorumHash)
	blockID := makeBlockID(tmhash.Sum([]byte("blockhash")), math.MaxInt32, tmhash.Sum([]byte("partshash")))
	blockID2 := makeBlockID(tmhash.Sum([]byte("blockhash2")), math.MaxInt32, tmhash.Sum([]byte("partshash")))
	quorumType := btcjson.LLMQType_5_60
	const chainID = "mychain"
	var height int64 = math.MaxInt64
	stateID := RandStateID().WithHeight(height - 1)

	v := makeVote(t, val, chainID, math.MaxInt32, height, 1, 0x01, quorumType, quorumHash, blockID, stateID)
	v2 := makeVote(t, val, chainID, math.MaxInt32, height, 2, 0x01, quorumType, quorumHash, blockID2, stateID)

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
	// Votes for duplicateEvidence
	quorumType := btcjson.LLMQType_5_60
	quorumHash := crypto.RandQuorumHash()
	val := NewMockPVForQuorum(quorumHash)
	key := bls12381.GenPrivKeyFromSecret([]byte("it's a secret")) // deterministic key
	val.UpdatePrivateKey(context.Background(), key, quorumHash, key.PubKey(), 10)
	blockID := makeBlockID(tmhash.Sum([]byte("blockhash")), math.MaxInt32, tmhash.Sum([]byte("partshash")))
	blockID2 := makeBlockID(tmhash.Sum([]byte("blockhash2")), math.MaxInt32, tmhash.Sum([]byte("partshash")))
	const chainID = "mychain"
	stateID := RandStateID().WithHeight(100)
	v := makeVote(t, val, chainID, math.MaxInt32, math.MaxInt64, 1, 0x01, quorumType, quorumHash, blockID, stateID)
	v2 := makeVote(t, val, chainID, math.MaxInt32, math.MaxInt64, 2, 0x01, quorumType, quorumHash, blockID2, stateID)

	testCases := []struct {
		testName string
		evList   EvidenceList
		expBytes string
	}{
		{"duplicateVoteEvidence",
			EvidenceList{&DuplicateVoteEvidence{VoteA: v2, VoteB: v}},
			"a9ce28d13bb31001fc3e5b7927051baf98f86abdbd64377643a304164c826923",
		},
	}

	for _, tc := range testCases {
		tc := tc
		hash := tc.evList.Hash()
		require.Equal(t, tc.expBytes, hex.EncodeToString(hash), tc.testName)
	}
}
