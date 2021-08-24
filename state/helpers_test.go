package state_test

import (
	"bytes"
	"fmt"
	"time"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/internal/test/factory"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmtime "github.com/tendermint/tendermint/libs/time"
	"github.com/tendermint/tendermint/pkg/abci"
	"github.com/tendermint/tendermint/pkg/consensus"
	"github.com/tendermint/tendermint/pkg/evidence"
	"github.com/tendermint/tendermint/pkg/metadata"
	tmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	sf "github.com/tendermint/tendermint/state/test/factory"
)

type paramsChangeTestCase struct {
	height int64
	params consensus.ConsensusParams
}

func newTestApp() proxy.AppConns {
	app := &testApp{}
	cc := proxy.NewLocalClientCreator(app)
	return proxy.NewAppConns(cc)
}

func makeAndCommitGoodBlock(
	state sm.State,
	height int64,
	lastCommit *metadata.Commit,
	proposerAddr []byte,
	blockExec *sm.BlockExecutor,
	privVals map[string]consensus.PrivValidator,
	evidence []evidence.Evidence) (sm.State, metadata.BlockID, *metadata.Commit, error) {
	// A good block passes
	state, blockID, err := makeAndApplyGoodBlock(state, height, lastCommit, proposerAddr, blockExec, evidence)
	if err != nil {
		return state, metadata.BlockID{}, nil, err
	}

	// Simulate a lastCommit for this block from all validators for the next height
	commit, err := makeValidCommit(height, blockID, state.Validators, privVals)
	if err != nil {
		return state, metadata.BlockID{}, nil, err
	}
	return state, blockID, commit, nil
}

func makeAndApplyGoodBlock(state sm.State, height int64, lastCommit *metadata.Commit, proposerAddr []byte,
	blockExec *sm.BlockExecutor, evidence []evidence.Evidence) (sm.State, metadata.BlockID, error) {
	block, _ := state.MakeBlock(height, factory.MakeTenTxs(height), lastCommit, evidence, proposerAddr)
	if err := blockExec.ValidateBlock(state, block); err != nil {
		return state, metadata.BlockID{}, err
	}
	blockID := metadata.BlockID{Hash: block.Hash(),
		PartSetHeader: metadata.PartSetHeader{Total: 3, Hash: tmrand.Bytes(32)}}
	state, err := blockExec.ApplyBlock(state, blockID, block)
	if err != nil {
		return state, metadata.BlockID{}, err
	}
	return state, blockID, nil
}

func makeValidCommit(
	height int64,
	blockID metadata.BlockID,
	vals *consensus.ValidatorSet,
	privVals map[string]consensus.PrivValidator,
) (*metadata.Commit, error) {
	sigs := make([]metadata.CommitSig, 0)
	for i := 0; i < vals.Size(); i++ {
		_, val := vals.GetByIndex(int32(i))
		vote, err := factory.MakeVote(privVals[val.Address.String()], chainID, int32(i), height, 0, 2, blockID, time.Now())
		if err != nil {
			return nil, err
		}
		sigs = append(sigs, vote.CommitSig())
	}
	return metadata.NewCommit(height, 0, blockID, sigs), nil
}

func makeState(nVals, height int) (sm.State, dbm.DB, map[string]consensus.PrivValidator) {
	vals := make([]consensus.GenesisValidator, nVals)
	privVals := make(map[string]consensus.PrivValidator, nVals)
	for i := 0; i < nVals; i++ {
		secret := []byte(fmt.Sprintf("test%d", i))
		pk := ed25519.GenPrivKeyFromSecret(secret)
		valAddr := pk.PubKey().Address()
		vals[i] = consensus.GenesisValidator{
			Address: valAddr,
			PubKey:  pk.PubKey(),
			Power:   1000,
			Name:    fmt.Sprintf("test%d", i),
		}
		privVals[valAddr.String()] = consensus.NewMockPVWithParams(pk, false, false)
	}
	s, _ := sm.MakeGenesisState(&consensus.GenesisDoc{
		ChainID:    chainID,
		Validators: vals,
		AppHash:    nil,
	})

	stateDB := dbm.NewMemDB()
	stateStore := sm.NewStore(stateDB)
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

func genValSet(size int) *consensus.ValidatorSet {
	vals := make([]*consensus.Validator, size)
	for i := 0; i < size; i++ {
		vals[i] = consensus.NewValidator(ed25519.GenPrivKey().PubKey(), 10)
	}
	return consensus.NewValidatorSet(vals)
}

func makeHeaderPartsResponsesValPubKeyChange(
	state sm.State,
	pubkey crypto.PubKey,
) (metadata.Header, metadata.BlockID, *tmstate.ABCIResponses) {

	block := sf.MakeBlock(state, state.LastBlockHeight+1, new(metadata.Commit))
	abciResponses := &tmstate.ABCIResponses{
		BeginBlock: &abci.ResponseBeginBlock{},
		EndBlock:   &abci.ResponseEndBlock{ValidatorUpdates: nil},
	}
	// If the pubkey is new, remove the old and add the new.
	_, val := state.NextValidators.GetByIndex(0)
	if !bytes.Equal(pubkey.Bytes(), val.PubKey.Bytes()) {
		vPbPk, err := cryptoenc.PubKeyToProto(val.PubKey)
		if err != nil {
			panic(err)
		}
		pbPk, err := cryptoenc.PubKeyToProto(pubkey)
		if err != nil {
			panic(err)
		}
		abciResponses.EndBlock = &abci.ResponseEndBlock{
			ValidatorUpdates: []abci.ValidatorUpdate{
				{PubKey: vPbPk, Power: 0},
				{PubKey: pbPk, Power: 10},
			},
		}
	}

	return block.Header, metadata.BlockID{Hash: block.Hash(), PartSetHeader: metadata.PartSetHeader{}}, abciResponses
}

func makeHeaderPartsResponsesValPowerChange(
	state sm.State,
	power int64,
) (metadata.Header, metadata.BlockID, *tmstate.ABCIResponses) {

	block := sf.MakeBlock(state, state.LastBlockHeight+1, new(metadata.Commit))
	abciResponses := &tmstate.ABCIResponses{
		BeginBlock: &abci.ResponseBeginBlock{},
		EndBlock:   &abci.ResponseEndBlock{ValidatorUpdates: nil},
	}

	// If the pubkey is new, remove the old and add the new.
	_, val := state.NextValidators.GetByIndex(0)
	if val.VotingPower != power {
		vPbPk, err := cryptoenc.PubKeyToProto(val.PubKey)
		if err != nil {
			panic(err)
		}
		abciResponses.EndBlock = &abci.ResponseEndBlock{
			ValidatorUpdates: []abci.ValidatorUpdate{
				{PubKey: vPbPk, Power: power},
			},
		}
	}

	return block.Header, metadata.BlockID{Hash: block.Hash(), PartSetHeader: metadata.PartSetHeader{}}, abciResponses
}

func makeHeaderPartsResponsesParams(
	state sm.State,
	params *consensus.ConsensusParams,
) (metadata.Header, metadata.BlockID, *tmstate.ABCIResponses) {

	block := sf.MakeBlock(state, state.LastBlockHeight+1, new(metadata.Commit))
	pbParams := params.ToProto()
	abciResponses := &tmstate.ABCIResponses{
		BeginBlock: &abci.ResponseBeginBlock{},
		EndBlock:   &abci.ResponseEndBlock{ConsensusParamUpdates: &pbParams},
	}
	return block.Header, metadata.BlockID{Hash: block.Hash(), PartSetHeader: metadata.PartSetHeader{}}, abciResponses
}

func randomGenesisDoc() *consensus.GenesisDoc {
	pubkey := ed25519.GenPrivKey().PubKey()
	return &consensus.GenesisDoc{
		GenesisTime: tmtime.Now(),
		ChainID:     "abc",
		Validators: []consensus.GenesisValidator{
			{
				Address: pubkey.Address(),
				PubKey:  pubkey,
				Power:   10,
				Name:    "myval",
			},
		},
		ConsensusParams: consensus.DefaultConsensusParams(),
	}
}

// used for testing by state store
func makeRandomStateFromValidatorSet(
	lastValSet *consensus.ValidatorSet,
	height, lastHeightValidatorsChanged int64,
) sm.State {
	return sm.State{
		LastBlockHeight:                  height - 1,
		NextValidators:                   lastValSet.CopyIncrementProposerPriority(2),
		Validators:                       lastValSet.CopyIncrementProposerPriority(1),
		LastValidators:                   lastValSet.Copy(),
		LastHeightConsensusParamsChanged: height,
		ConsensusParams:                  *consensus.DefaultConsensusParams(),
		LastHeightValidatorsChanged:      lastHeightValidatorsChanged,
		InitialHeight:                    1,
	}
}

func makeRandomStateFromConsensusParams(consensusParams *consensus.ConsensusParams,
	height, lastHeightConsensusParamsChanged int64) sm.State {
	val, _ := factory.RandValidator(true, 10)
	valSet := consensus.NewValidatorSet([]*consensus.Validator{val})
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

func (app *testApp) BeginBlock(req abci.RequestBeginBlock) abci.ResponseBeginBlock {
	app.CommitVotes = req.LastCommitInfo.Votes
	app.ByzantineValidators = req.ByzantineValidators
	return abci.ResponseBeginBlock{}
}

func (app *testApp) EndBlock(req abci.RequestEndBlock) abci.ResponseEndBlock {
	return abci.ResponseEndBlock{
		ValidatorUpdates: app.ValidatorUpdates,
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
