package state_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	dbm "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/internal/test"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

type paramsChangeTestCase struct {
	height int64
	params types.ConsensusParams
}

func newTestApp() proxy.AppConns {
	app := &testApp{}
	cc := proxy.NewLocalClientCreator(app)
	return proxy.NewAppConns(cc, proxy.NopMetrics())
}

func makeAndCommitGoodBlock(
	state sm.State,
	height int64,
	lastCommit *types.Commit,
	proposerAddr []byte,
	blockExec *sm.BlockExecutor,
	privVals map[string]types.PrivValidator,
	evidence []types.Evidence) (sm.State, types.BlockID, *types.ExtendedCommit, error) {
	// A good block passes
	state, blockID, err := makeAndApplyGoodBlock(state, height, lastCommit, proposerAddr, blockExec, evidence)
	if err != nil {
		return state, types.BlockID{}, nil, err
	}

	// Simulate a lastCommit for this block from all validators for the next height
	commit, _, err := makeValidCommit(height, blockID, state.Validators, privVals)
	if err != nil {
		return state, types.BlockID{}, nil, err
	}
	return state, blockID, commit, nil
}

func makeAndApplyGoodBlock(state sm.State, height int64, lastCommit *types.Commit, proposerAddr []byte,
	blockExec *sm.BlockExecutor, evidence []types.Evidence) (sm.State, types.BlockID, error) {
	block := state.MakeBlock(height, test.MakeNTxs(height, 10), lastCommit, evidence, proposerAddr)
	partSet, err := block.MakePartSet(types.BlockPartSizeBytes)
	if err != nil {
		return state, types.BlockID{}, err
	}

	if err := blockExec.ValidateBlock(state, block); err != nil {
		return state, types.BlockID{}, err
	}
	blockID := types.BlockID{Hash: block.Hash(),
		PartSetHeader: partSet.Header()}
	state, err = blockExec.ApplyBlock(state, blockID, block)
	if err != nil {
		return state, types.BlockID{}, err
	}
	return state, blockID, nil
}

func makeBlock(state sm.State, height int64, c *types.Commit) *types.Block {
	return state.MakeBlock(
		height,
		test.MakeNTxs(state.LastBlockHeight, 10),
		c,
		nil,
		state.Validators.GetProposer().Address,
	)
}

func makeValidCommit(
	height int64,
	blockID types.BlockID,
	vals *types.ValidatorSet,
	privVals map[string]types.PrivValidator,
) (*types.ExtendedCommit, []*types.Vote, error) {
	sigs := make([]types.ExtendedCommitSig, vals.Size())
	votes := make([]*types.Vote, vals.Size())
	for i := 0; i < vals.Size(); i++ {
		_, val := vals.GetByIndex(int32(i))
		vote, err := types.MakeVote(height, blockID, vals, privVals[val.Address.String()], chainID, time.Now())
		if err != nil {
			return nil, nil, err
		}
		sigs[i] = vote.ExtendedCommitSig()
		votes[i] = vote
	}
	return &types.ExtendedCommit{
		Height:             height,
		BlockID:            blockID,
		ExtendedSignatures: sigs,
	}, votes, nil
}

func makeState(nVals, height int) (sm.State, dbm.DB, map[string]types.PrivValidator) {
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
	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardABCIResponses: false,
	})
	if err := stateStore.Save(s); err != nil {
		panic(err)
	}

	for i := 1; i < height; i++ {
		s.LastBlockHeight++
		s.LastValidators = s.Validators.Copy()
		if err := stateStore.Save(s); err != nil {
			panic(err)
		}
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

	block := makeBlock(state, state.LastBlockHeight+1, new(types.Commit))
	abciResponses := &abci.ResponseFinalizeBlock{}
	// If the pubkey is new, remove the old and add the new.
	_, val := state.NextValidators.GetByIndex(0)
	if !bytes.Equal(pubkey.Bytes(), val.PubKey.Bytes()) {
		abciResponses.ValidatorUpdates = []abci.ValidatorUpdate{
			types.TM2PB.NewValidatorUpdate(val.PubKey, 0),
			types.TM2PB.NewValidatorUpdate(pubkey, 10),
		}
	}

	return block.Header, types.BlockID{Hash: block.Hash(), PartSetHeader: types.PartSetHeader{}}, abciResponses
}

func makeHeaderPartsResponsesValPowerChange(
	t *testing.T,
	state sm.State,
	power int64,
) (types.Header, types.BlockID, *abci.ResponseFinalizeBlock) {

	block := makeBlock(state, state.LastBlockHeight+1, new(types.Commit))
	abciResponses := &abci.ResponseFinalizeBlock{}

	// If the pubkey is new, remove the old and add the new.
	_, val := state.NextValidators.GetByIndex(0)
	if val.VotingPower != power {
		abciResponses.ValidatorUpdates = []abci.ValidatorUpdate{
			types.TM2PB.NewValidatorUpdate(val.PubKey, power),
		}
	}

	return block.Header, types.BlockID{Hash: block.Hash(), PartSetHeader: types.PartSetHeader{}}, abciResponses
}

func makeHeaderPartsResponsesParams(
	t *testing.T,
	state sm.State,
	params tmproto.ConsensusParams,
) (types.Header, types.BlockID, *abci.ResponseFinalizeBlock) {

	block := makeBlock(state, state.LastBlockHeight+1, new(types.Commit))
	abciResponses := &abci.ResponseFinalizeBlock{
		ConsensusParamUpdates: &params,
	}
	return block.Header, types.BlockID{Hash: block.Hash(), PartSetHeader: types.PartSetHeader{}}, abciResponses
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

//----------------------------------------------------------------------------

type testApp struct {
	abci.BaseApplication

	CommitVotes      []abci.VoteInfo
	Misbehavior      []abci.Misbehavior
	ValidatorUpdates []abci.ValidatorUpdate
	AgreedAppData    []byte
}

var _ abci.Application = (*testApp)(nil)

func (app *testApp) FinalizeBlock(_ context.Context, req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
	app.CommitVotes = req.DecidedLastCommit.Votes
	app.Misbehavior = req.Misbehavior
	txResults := make([]*abci.ExecTxResult, len(req.Txs))
	for idx := range req.Txs {
		txResults[idx] = &abci.ExecTxResult{
			Code: abci.CodeTypeOK,
		}
	}

	return &abci.ResponseFinalizeBlock{
		ValidatorUpdates: app.ValidatorUpdates,
		ConsensusParamUpdates: &tmproto.ConsensusParams{
			Version: &tmproto.VersionParams{
				App: 1}},
		TxResults:     txResults,
		AgreedAppData: app.AgreedAppData,
	}, nil
}

func (app *testApp) Commit(_ context.Context, _ *abci.RequestCommit) (*abci.ResponseCommit, error) {
	return &abci.ResponseCommit{RetainHeight: 1}, nil
}

func (app *testApp) ProcessProposal(_ context.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
	for _, tx := range req.Txs {
		if len(tx) == 0 {
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, nil
		}
	}
	return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil
}
