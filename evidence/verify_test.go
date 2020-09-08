package evidence

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/mock"

	// "github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/evidence/mocks"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

func TestVerifyEvidenceWrongAddress(t *testing.T) {
	var height int64 = 4
	val := types.NewMockPV()
	stateStore := initializeValidatorState(val, height)
	state := stateStore.LoadState()
	blockStore := &mocks.BlockStore{}
	blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(
		&types.BlockMeta{Header: types.Header{Time: defaultEvidenceTime}},
	)
	evidence := types.NewMockDuplicateVoteEvidence(1, defaultEvidenceTime, evidenceChainID)
	err := VerifyEvidence(evidence, state, stateStore, blockStore)
	errMsg := fmt.Sprintf("address %X was not a validator at height 1", evidence.Addresses()[0])
	if assert.Error(t, err) {
		assert.Equal(t, err.Error(), errMsg)
	}
}

func TestVerifyEvidenceExpiredEvidence(t *testing.T) {
	var height int64 = 4
	val := types.NewMockPV()
	stateStore := initializeValidatorState(val, height)
	state := stateStore.LoadState()
	state.ConsensusParams.Evidence.MaxAgeNumBlocks = 1
	expiredEvidenceTime := time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	blockStore := &mocks.BlockStore{}
	blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(
		&types.BlockMeta{Header: types.Header{Time: expiredEvidenceTime}},
	)

	expiredEv := types.NewMockDuplicateVoteEvidenceWithValidator(1, expiredEvidenceTime, val, evidenceChainID)
	err := VerifyEvidence(expiredEv, state, stateStore, blockStore)
	errMsg := "evidence from height 1 (created at: 2018-01-01 00:00:00 +0000 UTC) is too old"
	if assert.Error(t, err) {
		assert.Equal(t, err.Error()[:len(errMsg)], errMsg)
	}
}

func TestVerifyEvidenceInvalidTime(t *testing.T) {
	height := int64(4)
	val := types.NewMockPV()
	stateStore := initializeValidatorState(val, height)
	state := stateStore.LoadState()
	blockStore := &mocks.BlockStore{}
	blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(
		&types.BlockMeta{Header: types.Header{Time: defaultEvidenceTime}},
	)

	differentTime := time.Date(2019, 2, 1, 0, 0, 0, 0, time.UTC)
	ev := types.NewMockDuplicateVoteEvidenceWithValidator(height, differentTime, val, evidenceChainID)
	err := VerifyEvidence(ev, state, stateStore, blockStore)
	errMsg := "evidence time (2019-02-01 00:00:00 +0000 UTC) is different to the time" +
		" of the header we have for the same height (2019-01-01 00:00:00 +0000 UTC)"
	if assert.Error(t, err) {
		assert.Equal(t, errMsg, err.Error())
	}
}

type voteData struct {
	vote1 *types.Vote
	vote2 *types.Vote
	valid bool
}

func TestVerifyDuplicateVoteEvidence(t *testing.T) {
	val := types.NewMockPV()
	val2 := types.NewMockPV()
	valSet := types.NewValidatorSet([]*types.Validator{val.ExtractIntoValidator(1)})
	stateStore := &mocks.StateStore{}
	stateStore.On("LoadValidators", mock.AnythingOfType("int64")).Return(valSet)

	blockID := makeBlockID([]byte("blockhash"), 1000, []byte("partshash"))
	blockID2 := makeBlockID([]byte("blockhash2"), 1000, []byte("partshash"))
	blockID3 := makeBlockID([]byte("blockhash"), 10000, []byte("partshash"))
	blockID4 := makeBlockID([]byte("blockhash"), 10000, []byte("partshash2"))

	const chainID = "mychain"

	vote1 := makeVote(t, val, chainID, 0, 10, 2, 1, blockID, defaultEvidenceTime)
	v1 := vote1.ToProto()
	err := val.SignVote(chainID, v1)
	require.NoError(t, err)
	badVote := makeVote(t, val, chainID, 0, 10, 2, 1, blockID, defaultEvidenceTime)
	bv := badVote.ToProto()
	err = val2.SignVote(chainID, bv)
	require.NoError(t, err)

	vote1.Signature = v1.Signature
	badVote.Signature = bv.Signature

	cases := []voteData{
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID2, defaultEvidenceTime), true}, // different block ids
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID3, defaultEvidenceTime), true},
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID4, defaultEvidenceTime), true},
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID, defaultEvidenceTime), false},     // wrong block id
		{vote1, makeVote(t, val, "mychain2", 0, 10, 2, 1, blockID2, defaultEvidenceTime), false}, // wrong chain id
		{vote1, makeVote(t, val, chainID, 0, 11, 2, 1, blockID2, defaultEvidenceTime), false},    // wrong height
		{vote1, makeVote(t, val, chainID, 0, 10, 3, 1, blockID2, defaultEvidenceTime), false},    // wrong round
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 2, blockID2, defaultEvidenceTime), false},    // wrong step
		{vote1, makeVote(t, val2, chainID, 0, 10, 2, 1, blockID, defaultEvidenceTime), false},    // wrong validator
		{vote1, makeVote(t, val2, chainID, 0, 10, 2, 1, blockID, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)), false},
		{vote1, badVote, false}, // signed by wrong key
	}

	require.NoError(t, err)
	for _, c := range cases {
		ev := &types.DuplicateVoteEvidence{
			VoteA: c.vote1,
			VoteB: c.vote2,

			Timestamp: defaultEvidenceTime,
		}
		if c.valid {
			assert.Nil(t, VerifyDuplicateVote(ev, chainID, stateStore), "evidence should be valid")
		} else {
			assert.NotNil(t, VerifyDuplicateVote(ev, chainID, stateStore), "evidence should be invalid")
		}
	}
}

func makeVote(
	t *testing.T, val types.PrivValidator, chainID string, valIndex int32, height int64, 
	round int32, step int, blockID types.BlockID, time time.Time) *types.Vote {
	pubKey, err := val.GetPubKey()
	require.NoError(t, err)
	v := &types.Vote{
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

// func makeHeaderRandom() *Header {
// 	return &Header{
// 		ChainID:            tmrand.Str(12),
// 		Height:             int64(tmrand.Uint16()) + 1,
// 		Time:               time.Now(),
// 		LastBlockID:        makeBlockIDRandom(),
// 		LastCommitHash:     crypto.CRandBytes(tmhash.Size),
// 		DataHash:           crypto.CRandBytes(tmhash.Size),
// 		ValidatorsHash:     crypto.CRandBytes(tmhash.Size),
// 		NextValidatorsHash: crypto.CRandBytes(tmhash.Size),
// 		ConsensusHash:      crypto.CRandBytes(tmhash.Size),
// 		AppHash:            crypto.CRandBytes(tmhash.Size),
// 		LastResultsHash:    crypto.CRandBytes(tmhash.Size),
// 		EvidenceHash:       crypto.CRandBytes(tmhash.Size),
// 		ProposerAddress:    crypto.CRandBytes(crypto.AddressSize),
// 	}
// }

func makeBlockID(hash []byte, partSetSize uint32, partSetHash []byte) types.BlockID {
	var (
		h   = make([]byte, tmhash.Size)
		psH = make([]byte, tmhash.Size)
	)
	copy(h, hash)
	copy(psH, partSetHash)
	return types.BlockID{
		Hash: h,
		PartSetHeader: types.PartSetHeader{
			Total: partSetSize,
			Hash:  psH,
		},
	}
}
