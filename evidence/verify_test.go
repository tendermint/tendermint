package evidence

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/tendermint/tendermint/evidence/mocks"
	"github.com/tendermint/tendermint/types"
)

func TestVerifyEvidenceWrongAddress(t *testing.T) {
	var height int64 = 4
	val := types.NewMockPV()
	stateStore := initializeValidatorState(val, height)
	state, err := stateStore.Load()
	if err != nil {
		t.Error(err)
	}
	blockStore := &mocks.BlockStore{}
	blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(
		&types.BlockMeta{Header: types.Header{Time: defaultEvidenceTime}},
	)
	evidence := types.NewMockDuplicateVoteEvidence(1, defaultEvidenceTime, evidenceChainID)
	err = VerifyEvidence(evidence, state, stateStore, blockStore)
	errMsg := fmt.Sprintf("address %X was not a validator at height 1", evidence.Address())
	if assert.Error(t, err) {
		assert.Equal(t, err.Error(), errMsg)
	}
}

func TestVerifyEvidenceExpiredEvidence(t *testing.T) {
	var height int64 = 4
	val := types.NewMockPV()
	stateStore := initializeValidatorState(val, height)
	state, err := stateStore.Load()
	if err != nil {
		t.Error(err)
	}
	state.ConsensusParams.Evidence.MaxAgeNumBlocks = 1
	expiredEvidenceTime := time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	blockStore := &mocks.BlockStore{}
	blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(
		&types.BlockMeta{Header: types.Header{Time: expiredEvidenceTime}},
	)

	expiredEv := types.NewMockDuplicateVoteEvidenceWithValidator(1, expiredEvidenceTime, val, evidenceChainID)
	err = VerifyEvidence(expiredEv, state, stateStore, blockStore)
	errMsg := "evidence from height 1 (created at: 2018-01-01 00:00:00 +0000 UTC) is too old"
	if assert.Error(t, err) {
		assert.Equal(t, err.Error()[:len(errMsg)], errMsg)
	}
}

func TestVerifyEvidenceInvalidTime(t *testing.T) {
	height := int64(4)
	val := types.NewMockPV()
	stateStore := initializeValidatorState(val, height)
	state, err := stateStore.Load()
	if err != nil {
		t.Error(err)
	}
	blockStore := &mocks.BlockStore{}
	blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(
		&types.BlockMeta{Header: types.Header{Time: defaultEvidenceTime}},
	)

	differentTime := time.Date(2019, 2, 1, 0, 0, 0, 0, time.UTC)
	ev := types.NewMockDuplicateVoteEvidenceWithValidator(height, differentTime, val, evidenceChainID)
	err = VerifyEvidence(ev, state, stateStore, blockStore)
	errMsg := "evidence time (2019-02-01 00:00:00 +0000 UTC) is different to the time" +
		" of the header we have for the same height (2019-01-01 00:00:00 +0000 UTC)"
	if assert.Error(t, err) {
		assert.Equal(t, errMsg, err.Error())
	}
}
