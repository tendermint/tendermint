//nolint: lll
package state_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	"github.com/tendermint/tendermint/crypto/encoding"
	sm "github.com/tendermint/tendermint/internal/state"
	sf "github.com/tendermint/tendermint/internal/state/test/factory"
	"github.com/tendermint/tendermint/internal/test/factory"
	tmtime "github.com/tendermint/tendermint/libs/time"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

type paramsChangeTestCase struct {
	height int64
	params types.ConsensusParams
}

func makeAndCommitGoodBlock(
	ctx context.Context,
	t *testing.T,
	state sm.State,
	nodeProTxHash crypto.ProTxHash,
	height int64,
	lastCommit *types.Commit,
	proposerProTxHash crypto.ProTxHash,
	blockExec *sm.BlockExecutor,
	privVals map[string]types.PrivValidator,
	evidence []types.Evidence,
	proposedAppVersion uint64,
) (sm.State, types.BlockID, *types.Commit) {
	t.Helper()

	// A good block passes
	state, blockID := makeAndApplyGoodBlock(ctx, t, state, nodeProTxHash, height, lastCommit, proposerProTxHash, blockExec, evidence, proposedAppVersion)

	// Simulate a lastCommit for this block from all validators for the next height
	commit, _ := makeValidCommit(ctx, t, height, blockID, state.LastStateID, state.Validators, privVals)

	return state, blockID, commit
}

func makeAndApplyGoodBlock(
	ctx context.Context,
	t *testing.T,
	state sm.State,
	nodeProTxHash crypto.ProTxHash,
	height int64,
	lastCommit *types.Commit,
	proposerProTxHash []byte,
	blockExec *sm.BlockExecutor,
	evidence []types.Evidence,
	proposedAppVersion uint64,
) (sm.State, types.BlockID) {
	t.Helper()
	block := state.MakeBlock(height, nil, factory.MakeNTxs(height, 10), lastCommit, evidence, proposerProTxHash, proposedAppVersion)
	partSet, err := block.MakePartSet(types.BlockPartSizeBytes)
	require.NoError(t, err)

	require.NoError(t, blockExec.ValidateBlock(ctx, state, block))
	blockID := types.BlockID{Hash: block.Hash(),
		PartSetHeader: partSet.Header()}
	state, err = blockExec.ApplyBlock(ctx, state, nodeProTxHash, blockID, block)
	require.NoError(t, err)

	return state, blockID
}

func makeValidCommit(
	ctx context.Context,
	t *testing.T,
	height int64,
	blockID types.BlockID,
	stateID types.StateID,
	vals *types.ValidatorSet,
	privVals map[string]types.PrivValidator,
) (*types.Commit, []*types.Vote) {
	t.Helper()
	votes := make([]*types.Vote, vals.Size())
	for i := 0; i < vals.Size(); i++ {
		_, val := vals.GetByIndex(int32(i))
		vote, err := factory.MakeVote(ctx, privVals[val.ProTxHash.String()], vals, chainID, int32(i), height, 0, 2, blockID, stateID)
		require.NoError(t, err)
		votes[i] = vote
	}
	thresholdSigns, err := types.NewSignsRecoverer(votes).Recover()
	require.NoError(t, err)
	return types.NewCommit(
		height, 0,
		blockID,
		stateID,
		&types.CommitSigns{
			QuorumSigns: *thresholdSigns,
			QuorumHash:  vals.QuorumHash,
		},
	), votes
}

func makeState(t *testing.T, nVals, height int) (sm.State, dbm.DB, map[string]types.PrivValidator) {
	privValsByProTxHash := make(map[string]types.PrivValidator, nVals)
	vals, privVals := types.RandValidatorSet(nVals)
	genVals := types.MakeGenesisValsFromValidatorSet(vals)
	for i := 0; i < nVals; i++ {
		genVals[i].Name = fmt.Sprintf("test%d", i)
		proTxHash := genVals[i].ProTxHash
		privValsByProTxHash[proTxHash.String()] = privVals[i]
	}
	s, _ := sm.MakeGenesisState(&types.GenesisDoc{
		ChainID:            chainID,
		Validators:         genVals,
		ThresholdPublicKey: vals.ThresholdPublicKey,
		QuorumHash:         vals.QuorumHash,
		AppHash:            nil,
	})

	stateDB := dbm.NewMemDB()
	stateStore := sm.NewStore(stateDB)
	require.NoError(t, stateStore.Save(s))

	for i := 1; i < height; i++ {
		s.LastBlockHeight++
		s.LastValidators = s.Validators.Copy()

		require.NoError(t, stateStore.Save(s))
	}

	return s, stateDB, privValsByProTxHash
}

func makeHeaderPartsResponsesValPowerChange(
	t *testing.T,
	state sm.State,
	power int64,
) (types.Header, *types.CoreChainLock, types.BlockID, *abci.ResponseFinalizeBlock) {
	t.Helper()

	block, err := sf.MakeBlock(state, state.LastBlockHeight+1, new(types.Commit), nil, 0)
	require.NoError(t, err)
	finalizeBlockResponses := &abci.ResponseFinalizeBlock{}

	// If the pubkey is new, remove the old and add the new.
	_, val := state.NextValidators.GetByIndex(0)
	if val.VotingPower != power {
		vPbPk, err := encoding.PubKeyToProto(val.PubKey)
		require.NoError(t, err)
		thresholdPubKey, err := encoding.PubKeyToProto(state.NextValidators.ThresholdPublicKey)
		require.NoError(t, err)

		finalizeBlockResponses = &abci.ResponseFinalizeBlock{
			ValidatorSetUpdate: &abci.ValidatorSetUpdate{
				ValidatorUpdates: []abci.ValidatorUpdate{
					{PubKey: &vPbPk, Power: power},
				},
				ThresholdPublicKey: thresholdPubKey,
				QuorumHash:         state.NextValidators.QuorumHash,
			},
		}
	}

	return block.Header, block.CoreChainLock, types.BlockID{Hash: block.Hash(), PartSetHeader: types.PartSetHeader{}}, finalizeBlockResponses
}

func makeHeaderPartsResponsesValKeysRegenerate(t *testing.T, state sm.State, regenerate bool) (types.Header, *types.CoreChainLock, types.BlockID, *abci.ResponseFinalizeBlock) {
	block, err := sf.MakeBlock(state, state.LastBlockHeight+1, new(types.Commit), nil, 0)
	if err != nil {
		t.Error(err)
	}
	fbResp := &abci.ResponseFinalizeBlock{}
	if regenerate == true {
		proTxHashes := state.Validators.GetProTxHashes()
		valUpdates := types.ValidatorUpdatesRegenerateOnProTxHashes(proTxHashes)
		fbResp.ValidatorSetUpdate = &valUpdates
	}
	return block.Header, block.CoreChainLock, types.BlockID{Hash: block.Hash(), PartSetHeader: types.PartSetHeader{}}, fbResp
}

func makeHeaderPartsResponsesParams(
	t *testing.T,
	state sm.State,
	params *types.ConsensusParams,
) (types.Header, *types.CoreChainLock, types.BlockID, *abci.ResponseFinalizeBlock) {
	t.Helper()

	block, err := sf.MakeBlock(state, state.LastBlockHeight+1, new(types.Commit), nil, 0)
	require.NoError(t, err)
	pbParams := params.ToProto()
	finalizeBlockResponses := &abci.ResponseFinalizeBlock{ConsensusParamUpdates: &pbParams}
	return block.Header, block.CoreChainLock, types.BlockID{Hash: block.Hash(), PartSetHeader: types.PartSetHeader{}}, finalizeBlockResponses
}

func randomGenesisDoc() *types.GenesisDoc {
	pubkey := bls12381.GenPrivKey().PubKey()
	return &types.GenesisDoc{
		GenesisTime: tmtime.Now(),
		ChainID:     "abc",
		Validators: []types.GenesisValidator{
			{
				PubKey:    pubkey,
				ProTxHash: crypto.RandProTxHash(),
				Power:     types.DefaultDashVotingPower,
				Name:      "myval",
			},
		},
		ConsensusParams:    types.DefaultConsensusParams(),
		ThresholdPublicKey: pubkey,
		QuorumHash:         crypto.RandQuorumHash(),
	}
}

// used for testing by state store
func makeRandomStateFromValidatorSet(
	lastValSet *types.ValidatorSet,
	height, lastHeightValidatorsChanged int64,
) sm.State {
	return sm.State{
		LastBlockHeight:                  height - 1,
		NextValidators:                   lastValSet.CopyIncrementProposerPriority(2),
		Validators:                       lastValSet.CopyIncrementProposerPriority(1),
		LastValidators:                   lastValSet.Copy(),
		LastHeightConsensusParamsChanged: height,
		ConsensusParams:                  *types.DefaultConsensusParams(),
		LastHeightValidatorsChanged:      lastHeightValidatorsChanged,
		InitialHeight:                    1,
	}
}
func makeRandomStateFromConsensusParams(
	ctx context.Context,
	t *testing.T,
	consensusParams *types.ConsensusParams,
	height,
	lastHeightConsensusParamsChanged int64,
) sm.State {
	t.Helper()
	valSet, _ := types.RandValidatorSet(1)
	return sm.State{
		LastBlockHeight:                  height - 1,
		ConsensusParams:                  *consensusParams,
		LastHeightConsensusParamsChanged: lastHeightConsensusParamsChanged,
		NextValidators:                   valSet.CopyIncrementProposerPriority(2),
		Validators:                       valSet.CopyIncrementProposerPriority(1),
		LastValidators:                   valSet.Copy(),
		LastHeightValidatorsChanged:      height,
		InitialHeight:                    1,
	}
}

//----------------------------------------------------------------------------

type testApp struct {
	abci.BaseApplication

	Misbehavior        []abci.Misbehavior
	ValidatorSetUpdate *abci.ValidatorSetUpdate
}

var _ abci.Application = (*testApp)(nil)

func (app *testApp) Info(_ context.Context, req *abci.RequestInfo) (*abci.ResponseInfo, error) {
	return &abci.ResponseInfo{}, nil
}

func (app *testApp) FinalizeBlock(_ context.Context, req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
	app.Misbehavior = req.Misbehavior

	resTxs := make([]*abci.ExecTxResult, len(req.Txs))
	for i, tx := range req.Txs {
		if len(tx) > 0 {
			resTxs[i] = &abci.ExecTxResult{Code: abci.CodeTypeOK}
		} else {
			resTxs[i] = &abci.ExecTxResult{Code: abci.CodeTypeOK + 10} // error
		}
	}

	return &abci.ResponseFinalizeBlock{
		ValidatorSetUpdate: app.ValidatorSetUpdate,
		ConsensusParamUpdates: &tmproto.ConsensusParams{
			Version: &tmproto.VersionParams{
				AppVersion: 1,
			},
		},
		Events:    []abci.Event{},
		TxResults: resTxs,
	}, nil
}

func (app *testApp) CheckTx(_ context.Context, req *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
	return &abci.ResponseCheckTx{}, nil
}

func (app *testApp) Commit(context.Context) (*abci.ResponseCommit, error) {
	return &abci.ResponseCommit{RetainHeight: 1}, nil
}

func (app *testApp) Query(_ context.Context, req *abci.RequestQuery) (*abci.ResponseQuery, error) {
	return &abci.ResponseQuery{}, nil
}

func (app *testApp) ProcessProposal(_ context.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
	for _, tx := range req.Txs {
		if len(tx) == 0 {
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, nil
		}
	}
	return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil
}
