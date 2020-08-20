package evidence

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/evidence/mocks"
	tmrand "github.com/tendermint/tendermint/libs/rand"
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
	errMsg := fmt.Sprintf("address %X was not a validator at height 1", evidence.Address())
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

func TestVerifyEvidenceWithLunaticValidatorEvidence(t *testing.T) {
	var height int64 = 4
	val := types.NewMockPV()
	stateStore := initializeValidatorState(val, height)
	blockID := types.BlockID{
		Hash: tmrand.Bytes(tmhash.Size),
		PartSetHeader: types.PartSetHeader{
			Total: 1,
			Hash:  tmrand.Bytes(tmhash.Size),
		},
	}
	h := &types.Header{
		ChainID:            evidenceChainID,
		Height:             3,
		Time:               defaultEvidenceTime,
		LastBlockID:        blockID,
		LastCommitHash:     tmhash.Sum([]byte("last_commit_hash")),
		DataHash:           tmhash.Sum([]byte("data_hash")),
		ValidatorsHash:     tmhash.Sum([]byte("validators_hash")),
		NextValidatorsHash: tmhash.Sum([]byte("next_validators_hash")),
		ConsensusHash:      tmhash.Sum([]byte("consensus_hash")),
		AppHash:            tmhash.Sum([]byte("app_hash")),
		LastResultsHash:    tmhash.Sum([]byte("last_results_hash")),
		EvidenceHash:       tmhash.Sum([]byte("evidence_hash")),
		ProposerAddress:    crypto.AddressHash([]byte("proposer_address")),
	}
	blockStore := &mocks.BlockStore{}
	blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(
		&types.BlockMeta{Header: *h},
	)

	validH1 := *h
	validH1.ValidatorsHash = tmhash.Sum([]byte("different_validators_hash"))

	validH2 := validH1
	validH2.Time = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	badH1 := validH1
	badH1.ChainID = "different_chain_id"

	badH2 := *h
	badH2.DataHash = tmhash.Sum([]byte("different_data_hash"))

	testCases := []struct {
		Header *types.Header
		ExpErr bool
		ErrMsg string
	}{
		{
			h,
			true,
			"ValidatorsHash matches committed hash",
		},
		{
			&validH1,
			false,
			"",
		},
		{
			&validH2,
			false,
			"",
		},
		{
			&badH1,
			true,
			"chainID do not match: test_chain vs different_chain_id",
		},
		{
			&badH2,
			true,
			"ValidatorsHash matches committed hash", // it doesn't recognise that the data hashes are different
		},
	}

	for idx, tc := range testCases {
		ev := types.NewLunaticValidatorEvidence(tc.Header,
			makeValidVoteForHeader(tc.Header, val), "ValidatorsHash", defaultEvidenceTime)
		err := VerifyEvidence(ev, stateStore.LoadState(), stateStore, blockStore)
		if tc.ExpErr {
			if assert.Error(t, err, fmt.Sprintf("expected an error for case: %d", idx)) {
				assert.Equal(t, tc.ErrMsg, err.Error(), fmt.Sprintf("case: %d", idx))
			}
		} else {
			assert.NoError(t, err, fmt.Sprintf("did not expect an error for case: %d", idx))
		}

	}
}

func makeValidVoteForHeader(header *types.Header, val types.MockPV) *types.Vote {
	vote := makeVote(header.Height, 1, 0, val.PrivKey.PubKey().Address(), types.BlockID{
		Hash: header.Hash(),
		PartSetHeader: types.PartSetHeader{
			Total: 100,
			Hash:  crypto.CRandBytes(tmhash.Size),
		},
	}, defaultEvidenceTime)
	v := vote.ToProto()
	err := val.SignVote(evidenceChainID, v)
	if err != nil {
		panic("verify_test: failed to sign vote for header")
	}
	vote.Signature = v.Signature
	return vote
}
