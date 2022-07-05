package state_test

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
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
	height int64,
	lastCommit *types.Commit,
	proposerAddr []byte,
	blockExec *sm.BlockExecutor,
	privVals map[string]types.PrivValidator,
	evidence []types.Evidence,
) (sm.State, types.BlockID, *types.ExtendedCommit) {
	t.Helper()

	// A good block passes
	state, blockID := makeAndApplyGoodBlock(ctx, t, state, height, lastCommit, proposerAddr, blockExec, evidence)

	// Simulate a lastCommit for this block from all validators for the next height
	commit, _ := makeValidCommit(ctx, t, height, blockID, state.Validators, privVals)

	return state, blockID, commit
}

func makeAndApplyGoodBlock(
	ctx context.Context,
	t *testing.T,
	state sm.State,
	height int64,
	lastCommit *types.Commit,
	proposerAddr []byte,
	blockExec *sm.BlockExecutor,
	evidence []types.Evidence,
) (sm.State, types.BlockID) {
	t.Helper()
	block := state.MakeBlock(height, factory.MakeNTxs(height, 10), lastCommit, evidence, proposerAddr)
	partSet, err := block.MakePartSet(types.BlockPartSizeBytes)
	require.NoError(t, err)

	require.NoError(t, blockExec.ValidateBlock(ctx, state, block))
	blockID := types.BlockID{Hash: block.Hash(),
		PartSetHeader: partSet.Header()}
	state, err = blockExec.ApplyBlock(ctx, state, blockID, block)
	require.NoError(t, err)

	return state, blockID
}

func makeValidCommit(
	ctx context.Context,
	t *testing.T,
	height int64,
	blockID types.BlockID,
	vals *types.ValidatorSet,
	privVals map[string]types.PrivValidator,
) (*types.ExtendedCommit, []*types.Vote) {
	t.Helper()
	sigs := make([]types.ExtendedCommitSig, vals.Size())
	votes := make([]*types.Vote, vals.Size())
	for i := 0; i < vals.Size(); i++ {
		_, val := vals.GetByIndex(int32(i))
		vote, err := factory.MakeVote(ctx, privVals[val.Address.String()], chainID, int32(i), height, 0, 2, blockID, time.Now())
		require.NoError(t, err)
		sigs[i] = vote.ExtendedCommitSig()
		votes[i] = vote
	}

	return &types.ExtendedCommit{
		Height:             height,
		BlockID:            blockID,
		ExtendedSignatures: sigs,
	}, votes
}

func makeState(t *testing.T, nVals, height int) (sm.State, dbm.DB, map[string]types.PrivValidator) {
	vals := make([]types.GenesisValidator, nVals)
	privVals := make(map[string]types.PrivValidator, nVals)
	for i := 0; i < nVals; i++ {
		secret := []byte(fmt.Sprintf("test%d", i))
		pk := ed25519.GenPrivKeyFromSecret(secret)
		valAddr := pk.PubKey().Address()
		vals[i] = types.GenesisValidator{
			Address: valAddr,
			PubKey:  pk.PubKey(),
			Power:   1000,
			Name:    fmt.Sprintf("test%d", i),
		}
		privVals[valAddr.String()] = types.NewMockPVWithParams(pk, false, false)
	}
	s, _ := sm.MakeGenesisState(&types.GenesisDoc{
		ChainID:    chainID,
		Validators: vals,
		AppHash:    nil,
	})

	stateDB := dbm.NewMemDB()
	stateStore := sm.NewStore(stateDB)
	require.NoError(t, stateStore.Save(s))

	for i := 1; i < height; i++ {
		s.LastBlockHeight++
		s.LastValidators = s.Validators.Copy()

		require.NoError(t, stateStore.Save(s))
	}

	return s, stateDB, privVals
}

func genValSet(size int) *types.ValidatorSet {
	vals := make([]*types.Validator, size)
	for i := 0; i < size; i++ {
		vals[i] = types.NewValidator(ed25519.GenPrivKey().PubKey(), 10)
	}
	return types.NewValidatorSet(vals)
}

func makeHeaderPartsResponsesValPubKeyChange(
	t *testing.T,
	state sm.State,
	pubkey crypto.PubKey,
) (types.Header, types.BlockID, *abci.ResponseFinalizeBlock) {

	block := sf.MakeBlock(state, state.LastBlockHeight+1, new(types.Commit))
	finalizeBlockResponses := &abci.ResponseFinalizeBlock{}
	// If the pubkey is new, remove the old and add the new.
	_, val := state.NextValidators.GetByIndex(0)
	if !bytes.Equal(pubkey.Bytes(), val.PubKey.Bytes()) {
		vPbPk, err := encoding.PubKeyToProto(val.PubKey)
		require.NoError(t, err)
		pbPk, err := encoding.PubKeyToProto(pubkey)
		require.NoError(t, err)

		finalizeBlockResponses.ValidatorUpdates = []abci.ValidatorUpdate{
			{PubKey: vPbPk, Power: 0},
			{PubKey: pbPk, Power: 10},
		}
	}

	return block.Header, types.BlockID{Hash: block.Hash(), PartSetHeader: types.PartSetHeader{}}, finalizeBlockResponses
}

func makeHeaderPartsResponsesValPowerChange(
	t *testing.T,
	state sm.State,
	power int64,
) (types.Header, types.BlockID, *abci.ResponseFinalizeBlock) {
	t.Helper()

	block := sf.MakeBlock(state, state.LastBlockHeight+1, new(types.Commit))
	finalizeBlockResponses := &abci.ResponseFinalizeBlock{}

	// If the pubkey is new, remove the old and add the new.
	_, val := state.NextValidators.GetByIndex(0)
	if val.VotingPower != power {
		vPbPk, err := encoding.PubKeyToProto(val.PubKey)
		require.NoError(t, err)

		finalizeBlockResponses.ValidatorUpdates = []abci.ValidatorUpdate{
			{PubKey: vPbPk, Power: power},
		}
	}

	return block.Header, types.BlockID{Hash: block.Hash(), PartSetHeader: types.PartSetHeader{}}, finalizeBlockResponses
}

func makeHeaderPartsResponsesParams(
	t *testing.T,
	state sm.State,
	params *types.ConsensusParams,
) (types.Header, types.BlockID, *abci.ResponseFinalizeBlock) {
	t.Helper()

	block := sf.MakeBlock(state, state.LastBlockHeight+1, new(types.Commit))
	pbParams := params.ToProto()
	finalizeBlockResponses := &abci.ResponseFinalizeBlock{ConsensusParamUpdates: &pbParams}
	return block.Header, types.BlockID{Hash: block.Hash(), PartSetHeader: types.PartSetHeader{}}, finalizeBlockResponses
}

func randomGenesisDoc() *types.GenesisDoc {
	pubkey := ed25519.GenPrivKey().PubKey()
	return &types.GenesisDoc{
		GenesisTime: tmtime.Now(),
		ChainID:     "abc",
		Validators: []types.GenesisValidator{
			{
				Address: pubkey.Address(),
				PubKey:  pubkey,
				Power:   10,
				Name:    "myval",
			},
		},
		ConsensusParams: types.DefaultConsensusParams(),
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
	val, _, err := factory.Validator(ctx, 10+int64(rand.Uint32()))
	require.NoError(t, err)
	valSet := types.NewValidatorSet([]*types.Validator{val})
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

	CommitVotes      []abci.VoteInfo
	Misbehavior      []abci.Misbehavior
	ValidatorUpdates []abci.ValidatorUpdate
}

var _ abci.Application = (*testApp)(nil)

func (app *testApp) Info(_ context.Context, req *abci.RequestInfo) (*abci.ResponseInfo, error) {
	return &abci.ResponseInfo{}, nil
}

func (app *testApp) FinalizeBlock(_ context.Context, req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
	app.CommitVotes = req.DecidedLastCommit.Votes
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
		ValidatorUpdates: app.ValidatorUpdates,
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
