//nolint: lll
package state_test

import (
	"fmt"
	"github.com/tendermint/tendermint/crypto/bls12381"
	dbm "github.com/tendermint/tm-db"

	abciclient "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/proxy"
	sm "github.com/tendermint/tendermint/internal/state"
	sf "github.com/tendermint/tendermint/internal/state/test/factory"
	"github.com/tendermint/tendermint/internal/test/factory"
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
	return proxy.NewAppConns(cc)
}

func makeAndCommitGoodBlock(
	state sm.State,
	nodeProTxHash *crypto.ProTxHash,
	height int64,
	lastCommit *types.Commit,
	proposerProTxHash crypto.ProTxHash,
	blockExec *sm.BlockExecutor,
	privVals map[string]types.PrivValidator,
	evidence []types.Evidence, proposedAppVersion uint64,
) (sm.State, types.BlockID, *types.Commit, error) {

	// A good block passes
	state, blockID, err := makeAndApplyGoodBlock(state, nodeProTxHash, height, lastCommit, proposerProTxHash, blockExec, evidence, proposedAppVersion)
	if err != nil {
		return state, types.BlockID{}, nil, err
	}

	// Simulate a lastCommit for this block from all validators for the next height
	commit, err := makeValidCommit(height, blockID, state.LastStateID, state.Validators, privVals)
	if err != nil {
		return state, types.BlockID{}, nil, err
	}
	return state, blockID, commit, nil
}

func makeAndApplyGoodBlock(state sm.State, nodeProTxHash *crypto.ProTxHash, height int64, lastCommit *types.Commit, proposerProTxHash []byte,
	blockExec *sm.BlockExecutor, evidence []types.Evidence, proposedAppVersion uint64) (sm.State, types.BlockID, error) {
	block, _ := state.MakeBlock(height, nil, factory.MakeTenTxs(height), lastCommit, evidence, proposerProTxHash, proposedAppVersion)
	if err := blockExec.ValidateBlock(state, block); err != nil {
		return state, types.BlockID{}, err
	}
	blockID := types.BlockID{Hash: block.Hash(),
		PartSetHeader: types.PartSetHeader{Total: 3, Hash: tmrand.Bytes(32)}}
	state, err := blockExec.ApplyBlock(state, nodeProTxHash, blockID, block)
	if err != nil {
		return state, types.BlockID{}, err
	}

	return state, blockID, nil
}

func makeValidCommit(
	height int64,
	blockID types.BlockID,
	stateID types.StateID,
	vals *types.ValidatorSet,
	privVals map[string]types.PrivValidator,
) (*types.Commit, error) {
	var blockSigs [][]byte
	var stateSigs [][]byte
	var blsIDs [][]byte
	for i := 0; i < vals.Size(); i++ {
		_, val := vals.GetByIndex(int32(i))
		vote, err := factory.MakeVote(privVals[val.ProTxHash.String()], vals, chainID, int32(i), height, 0, 2, blockID, stateID)
		if err != nil {
			return nil, err
		}
		blockSigs = append(blockSigs, vote.BlockSignature)
		stateSigs = append(stateSigs, vote.StateSignature)
		blsIDs = append(blsIDs, vote.ValidatorProTxHash)
	}

	thresholdBlockSig, _ := bls12381.RecoverThresholdSignatureFromShares(blockSigs, blsIDs)
	thresholdStateSig, _ := bls12381.RecoverThresholdSignatureFromShares(stateSigs, blsIDs)

	return types.NewCommit(height, 0, blockID, stateID, vals.QuorumHash, thresholdBlockSig, thresholdStateSig), nil
}

func makeState(nVals int, height int64) (sm.State, dbm.DB, map[string]types.PrivValidator) {
	privValsByProTxHash := make(map[string]types.PrivValidator, nVals)
	vals, privVals, quorumHash, thresholdPublicKey := factory.GenerateMockGenesisValidators(nVals)
	for i := 0; i < nVals; i++ {
		vals[i].Name = fmt.Sprintf("test%d", i)
		proTxHash := vals[i].ProTxHash
		privValsByProTxHash[proTxHash.String()] = privVals[i]
	}
	s, _ := sm.MakeGenesisState(&types.GenesisDoc{
		ChainID:            chainID,
		Validators:         vals,
		ThresholdPublicKey: thresholdPublicKey,
		QuorumHash:         quorumHash,
		AppHash:            nil,
	})

	stateDB := dbm.NewMemDB()
	stateStore := sm.NewStore(stateDB)
	if err := stateStore.Save(s); err != nil {
		panic(err)
	}

	for i := int64(1); i < height; i++ {
		s.LastBlockHeight++
		s.LastValidators = s.Validators.Copy()
		if err := stateStore.Save(s); err != nil {
			panic(err)
		}
	}

	return s, stateDB, privValsByProTxHash
}

func makeHeaderPartsResponsesValKeysRegenerate(state sm.State, regenerate bool) (types.Header, *types.CoreChainLock, types.BlockID, *tmstate.ABCIResponses, error) {
	block, err := sf.MakeBlock(state, state.LastBlockHeight+1, new(types.Commit), nil, 0)
	if err != nil {
		return types.Header{}, nil, types.BlockID{}, nil, err
	}
	abciResponses := &tmstate.ABCIResponses{
		BeginBlock: &abci.ResponseBeginBlock{},
		EndBlock:   &abci.ResponseEndBlock{ValidatorSetUpdate: nil},
	}
	if regenerate == true {
		proTxHashes := state.Validators.GetProTxHashes()
		valUpdates := types.ValidatorUpdatesRegenerateOnProTxHashes(proTxHashes)
		abciResponses.EndBlock = &abci.ResponseEndBlock{
			ValidatorSetUpdate: &valUpdates,
		}
	}

	return block.Header, block.CoreChainLock, types.BlockID{Hash: block.Hash(), PartSetHeader: types.PartSetHeader{}}, abciResponses, nil
}

func makeHeaderPartsResponsesParams(
	state sm.State,
	params types.ConsensusParams,
) (types.Header, *types.CoreChainLock, types.BlockID, *tmstate.ABCIResponses, error) {

	block, err := sf.MakeBlock(state, state.LastBlockHeight+1, new(types.Commit), nil, 0)
	pbParams := params.ToProto()
	abciResponses := &tmstate.ABCIResponses{
		BeginBlock: &abci.ResponseBeginBlock{},
		EndBlock:   &abci.ResponseEndBlock{ConsensusParamUpdates: &pbParams},
	}
	return block.Header, block.CoreChainLock, types.BlockID{Hash: block.Hash(), PartSetHeader: types.PartSetHeader{}}, abciResponses, err
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

func makeRandomStateFromConsensusParams(consensusParams *types.ConsensusParams,
	height, lastHeightConsensusParamsChanged int64) sm.State {
	valSet, _ := factory.RandValidatorSet(1)
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

	ByzantineValidators []abci.Evidence
	ValidatorSetUpdate  *abci.ValidatorSetUpdate
}

var _ abci.Application = (*testApp)(nil)

func (app *testApp) Info(req abci.RequestInfo) (resInfo abci.ResponseInfo) {
	return abci.ResponseInfo{}
}

func (app *testApp) BeginBlock(req abci.RequestBeginBlock) abci.ResponseBeginBlock {
	app.ByzantineValidators = req.ByzantineValidators
	return abci.ResponseBeginBlock{}
}

func (app *testApp) EndBlock(req abci.RequestEndBlock) abci.ResponseEndBlock {
	return abci.ResponseEndBlock{
		ValidatorSetUpdate: app.ValidatorSetUpdate,
		ConsensusParamUpdates: &tmproto.ConsensusParams{
			Version: &tmproto.VersionParams{
				AppVersion: 1}}}
}

func (app *testApp) DeliverTx(req abci.RequestDeliverTx) abci.ResponseDeliverTx {
	return abci.ResponseDeliverTx{Events: []abci.Event{}}
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
