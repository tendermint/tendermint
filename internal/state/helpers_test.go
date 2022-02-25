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

	abciclient "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/internal/proxy"
	sm "github.com/tendermint/tendermint/internal/state"
	sf "github.com/tendermint/tendermint/internal/state/test/factory"
	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmtime "github.com/tendermint/tendermint/libs/time"
	tmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

type paramsChangeTestCase struct {
	height int64
	params types.ConsensusParams
}

func newTestApp() proxy.AppConns {
	app := &testApp{}
	cc := abciclient.NewLocalCreator(app)
	return proxy.NewAppConns(cc, log.NewNopLogger(), proxy.NopMetrics())
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
) (sm.State, types.BlockID, *types.Commit) {
	t.Helper()

	// A good block passes
	state, blockID := makeAndApplyGoodBlock(ctx, t, state, height, lastCommit, proposerAddr, blockExec, evidence)

	// Simulate a lastCommit for this block from all validators for the next height
	commit := makeValidCommit(ctx, t, height, blockID, state.Validators, privVals)

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
	block, _, err := state.MakeBlock(height, factory.MakeTenTxs(height), lastCommit, evidence, proposerAddr)
	require.NoError(t, err)

	require.NoError(t, blockExec.ValidateBlock(ctx, state, block))
	blockID := types.BlockID{Hash: block.Hash(),
		PartSetHeader: types.PartSetHeader{Total: 3, Hash: tmrand.Bytes(32)}}
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
) *types.Commit {
	t.Helper()
	sigs := make([]types.CommitSig, 0)
	for i := 0; i < vals.Size(); i++ {
		_, val := vals.GetByIndex(int32(i))
		vote, err := factory.MakeVote(ctx, privVals[val.Address.String()], chainID, int32(i), height, 0, 2, blockID, time.Now())
		require.NoError(t, err)
		sigs = append(sigs, vote.CommitSig())
	}

	return types.NewCommit(height, 0, blockID, sigs)
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
) (types.Header, types.BlockID, *tmstate.ABCIResponses) {

	block, err := sf.MakeBlock(state, state.LastBlockHeight+1, new(types.Commit))
	require.NoError(t, err)
	abciResponses := &tmstate.ABCIResponses{
		FinalizeBlock: &abci.ResponseFinalizeBlock{ValidatorUpdates: nil},
	}
	// If the pubkey is new, remove the old and add the new.
	_, val := state.NextValidators.GetByIndex(0)
	if !bytes.Equal(pubkey.Bytes(), val.PubKey.Bytes()) {
		vPbPk, err := encoding.PubKeyToProto(val.PubKey)
		require.NoError(t, err)
		pbPk, err := encoding.PubKeyToProto(pubkey)
		require.NoError(t, err)

		abciResponses.FinalizeBlock = &abci.ResponseFinalizeBlock{
			ValidatorUpdates: []abci.ValidatorUpdate{
				{PubKey: vPbPk, Power: 0},
				{PubKey: pbPk, Power: 10},
			},
		}
	}

	return block.Header, types.BlockID{Hash: block.Hash(), PartSetHeader: types.PartSetHeader{}}, abciResponses
}

func makeHeaderPartsResponsesValPowerChange(
	t *testing.T,
	state sm.State,
	power int64,
) (types.Header, types.BlockID, *tmstate.ABCIResponses) {
	t.Helper()

	block, err := sf.MakeBlock(state, state.LastBlockHeight+1, new(types.Commit))
	require.NoError(t, err)

	abciResponses := &tmstate.ABCIResponses{
		FinalizeBlock: &abci.ResponseFinalizeBlock{ValidatorUpdates: nil},
	}

	// If the pubkey is new, remove the old and add the new.
	_, val := state.NextValidators.GetByIndex(0)
	if val.VotingPower != power {
		vPbPk, err := encoding.PubKeyToProto(val.PubKey)
		require.NoError(t, err)

		abciResponses.FinalizeBlock = &abci.ResponseFinalizeBlock{
			ValidatorUpdates: []abci.ValidatorUpdate{
				{PubKey: vPbPk, Power: power},
			},
		}
	}

	return block.Header, types.BlockID{Hash: block.Hash(), PartSetHeader: types.PartSetHeader{}}, abciResponses
}

func makeHeaderPartsResponsesParams(
	t *testing.T,
	state sm.State,
	params *types.ConsensusParams,
) (types.Header, types.BlockID, *tmstate.ABCIResponses) {
	t.Helper()

	block, err := sf.MakeBlock(state, state.LastBlockHeight+1, new(types.Commit))
	require.NoError(t, err)
	pbParams := params.ToProto()
	abciResponses := &tmstate.ABCIResponses{
		FinalizeBlock: &abci.ResponseFinalizeBlock{ConsensusParamUpdates: &pbParams},
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

	CommitVotes         []abci.VoteInfo
	ByzantineValidators []abci.Evidence
	ValidatorUpdates    []abci.ValidatorUpdate
}

var _ abci.Application = (*testApp)(nil)

func (app *testApp) Info(req abci.RequestInfo) (resInfo abci.ResponseInfo) {
	return abci.ResponseInfo{}
}

func (app *testApp) FinalizeBlock(req abci.RequestFinalizeBlock) abci.ResponseFinalizeBlock {
	app.CommitVotes = req.LastCommitInfo.Votes
	app.ByzantineValidators = req.ByzantineValidators

	resTxs := make([]*abci.ResponseDeliverTx, len(req.Txs))
	for i, tx := range req.Txs {
		if len(tx) > 0 {
			resTxs[i] = &abci.ResponseDeliverTx{Code: abci.CodeTypeOK}
		} else {
			resTxs[i] = &abci.ResponseDeliverTx{Code: abci.CodeTypeOK + 10} // error
		}
	}

	return abci.ResponseFinalizeBlock{
		ValidatorUpdates: app.ValidatorUpdates,
		ConsensusParamUpdates: &tmproto.ConsensusParams{
			Version: &tmproto.VersionParams{
				AppVersion: 1,
			},
		},
		Events: []abci.Event{},
		Txs:    resTxs,
	}
}

func (app *testApp) CheckTx(req abci.RequestCheckTx) abci.ResponseCheckTx {
	return abci.ResponseCheckTx{}
}

func (app *testApp) Commit() abci.ResponseCommit {
	return abci.ResponseCommit{RetainHeight: 1}
}

func (app *testApp) Query(reqQuery abci.RequestQuery) (resQuery abci.ResponseQuery) {
	return
}

func (app *testApp) ProcessProposal(req abci.RequestProcessProposal) abci.ResponseProcessProposal {
	for _, tx := range req.Txs {
		if len(tx) == 0 {
			return abci.ResponseProcessProposal{Accept: false}
		}
	}
	return abci.ResponseProcessProposal{Accept: true}
}
