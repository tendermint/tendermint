package node

import (
	"bytes"
	"context"
	"fmt"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/internal/libs/clist"
	mempl "github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/libs/log"
	tmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

// syncWithApplication is called every time on start up. It handshakes with the app
// to understand the current state of the app. If this is the first time running
// Tendermint it will call InitChain, feeding in the genesis doc and produce
// the initial state. If the application is behind, Tendermint will replay all
// blocks in between and assert that the app hashes produced are the same as
// before.
func syncWithApplication(
	stateStore sm.Store,
	blockStore sm.BlockStore,
	genDoc *types.GenesisDoc,
	state sm.State,
	eventBus types.BlockEventPublisher,
	proxyApp proxy.AppConns,
	stateSync bool,
	logger log.Logger,
) (sm.State, error) {

	// Handshake is done via ABCI Info on the query conn. This gives the node
	// information on the app's latest height, hash and version
	res, err := proxyApp.Query().InfoSync(context.Background(), proxy.RequestInfo)
	if err != nil {
		return sm.State{}, fmt.Errorf("error calling Info: %v", err)
	}
	appHash := res.LastBlockAppHash
	appBlockHeight := res.LastBlockHeight

	// Only set the version if there is no existing state.
	if state.LastBlockHeight == 0 {
		state.Version.Consensus.App = res.AppVersion
	}

	// If application's blockHeight is 0 it means that the app is at genesis and
	// if the node is not running state sync it will need to InitChain. If
	// Tendermint itself is at genesis it will save this initial state in the
	// state store.
	// NOTE: that the Tendermint node might have existing state and will need to
	// catch up the application. We nonetheless still need to run InitChain.
	if appBlockHeight == 0 || !stateSync {
		state, err = initializeChain(proxyApp, stateStore, state, genDoc, logger)
		if err != nil {
			return sm.State{}, fmt.Errorf("error initializing chain: %w", err)
		}

		appHash = state.AppHash
	}

	// Check if the Tendermint has prior state. If so it may need to replay
	// blocks to the application
	storeBlockBase := blockStore.Base()
	storeBlockHeight := blockStore.Height()
	stateBlockHeight := state.LastBlockHeight

	// First handle edge cases and constraints on the storeBlockHeight and storeBlockBase.
	switch {
	case storeBlockHeight == 0:
		// Fresh instance of Tendermint. Return early
		return state, nil

	case appBlockHeight == 0 && state.InitialHeight < storeBlockBase:
		// the app has no state, and the block store is truncated above the
		// initial height. The node can't replay those blocks.
		return sm.State{}, sm.ErrAppBlockHeightTooLow{AppHeight: appBlockHeight, StoreBase: storeBlockBase}

	case appBlockHeight > 0 && appBlockHeight < storeBlockBase-1:
		// the app is too far behind truncated store (can be 1 behind since we replay the next)
		return sm.State{}, sm.ErrAppBlockHeightTooLow{AppHeight: appBlockHeight, StoreBase: storeBlockBase}

	case storeBlockHeight < appBlockHeight:
		// the app should never be ahead of the store (but this is under app's control)
		return sm.State{}, sm.ErrAppBlockHeightTooHigh{CoreHeight: storeBlockHeight, AppHeight: appBlockHeight}

	case storeBlockHeight < stateBlockHeight:
		// the state should never be ahead of the store (this is under Tendermint's control)
		panic(fmt.Sprintf("StateBlockHeight (%d) > StoreBlockHeight (%d)", stateBlockHeight, storeBlockHeight))

	case storeBlockHeight > stateBlockHeight+1:
		// store should be at most one ahead of the state (this is under Tendermint's control)
		panic(fmt.Sprintf("StoreBlockHeight (%d) > StateBlockHeight + 1 (%d)", storeBlockHeight, stateBlockHeight+1))
	}

	// Now either store is equal to state, or one ahead.
	// For each, consider all cases of where the app could be, given app <= store
	if storeBlockHeight == stateBlockHeight {
		// Tendermint ran Commit and saved the state.
		// Either the app is asking for replay, or we're all synced up.
		if appBlockHeight < storeBlockHeight {
			// the app is behind, so replay blocks, but no need to go through WAL (state is already synced to store)
			return state, replayBlocks(
				state, proxyApp, blockStore, stateStore, appBlockHeight, storeBlockHeight, false, eventBus, genDoc, logger,
			)

		} else if appBlockHeight == storeBlockHeight {
			// We're all synced up
			return state, nil
		}

	} else if storeBlockHeight == stateBlockHeight+1 {
		// We saved the block in the store but haven't updated the state,
		// so we'll need to replay a block using the WAL.
		switch {
		case appBlockHeight < stateBlockHeight:
			// the app is further behind than it should be, so replay blocks
			// up to storeBlockHeight and run the
			return state, replayBlocks(
				state, proxyApp, blockStore, stateStore, appBlockHeight, storeBlockHeight, true, eventBus, genDoc, logger,
			)

		case appBlockHeight == stateBlockHeight:
			// We haven't run Commit (both the state and app are one block behind),
			// so replayBlock with the real app.
			// NOTE: We could instead use the cs.WAL on cs.Start,
			// but we'd have to allow the WAL to replay a block that wrote it's #ENDHEIGHT
			logger.Info("Replay last block using real app")
			_, err = replayBlock(state, blockStore, stateStore, storeBlockHeight, proxyApp.Consensus(), eventBus, logger)
			return state, err

		case appBlockHeight == storeBlockHeight:
			// We ran Commit, but didn't save the state, so replayBlock with mock app.
			abciResponses, err := stateStore.LoadABCIResponses(storeBlockHeight)
			if err != nil {
				return state, err
			}
			mockApp := newMockProxyApp(appHash, abciResponses)
			logger.Info("Replay last block using mock app")
			_, err = replayBlock(state, blockStore, stateStore, storeBlockHeight, mockApp, eventBus, logger)
			return state, err
		}

	}

	panic(fmt.Sprintf("uncovered case! appHeight: %d, storeHeight: %d, stateHeight: %d",
		appBlockHeight, storeBlockHeight, stateBlockHeight))
}

// initializeChain sends the genDoc information to the app as part of InitChain.
// If Tendermint state hasn't been initialized yet it takes any state changes
// from the app, applies them and then persists the initial state.
func initializeChain(
	proxyApp proxy.AppConns,
	stateStore sm.Store,
	currentState sm.State,
	genDoc *types.GenesisDoc,
	logger log.Logger,
) (sm.State, error) {
	logger.Info("Initializing Chain with Application")

	validators := make([]*types.Validator, len(genDoc.Validators))
	for i, val := range genDoc.Validators {
		validators[i] = types.NewValidator(val.PubKey, val.Power)
	}
	validatorSet := types.NewValidatorSet(validators)
	nextVals := types.TM2PB.ValidatorUpdates(validatorSet)
	pbParams := genDoc.ConsensusParams.ToProto()
	req := abci.RequestInitChain{
		Time:            genDoc.GenesisTime,
		ChainId:         genDoc.ChainID,
		InitialHeight:   genDoc.InitialHeight,
		ConsensusParams: &pbParams,
		Validators:      nextVals,
		AppStateBytes:   genDoc.AppState,
	}
	res, err := proxyApp.Consensus().InitChainSync(context.Background(), req)
	if err != nil {
		return sm.State{}, err
	}

	// we only update state when we are in initial state
	if currentState.LastBlockHeight == 0 {
		// If the app did not return an app hash, we keep the one set from the genesis doc in
		// the state. We don't set appHash since we don't want the genesis doc app hash
		// recorded in the genesis block. We should probably just remove GenesisDoc.AppHash.
		if len(res.AppHash) > 0 {
			currentState.AppHash = res.AppHash
		}
		// If the app returned validators or consensus params, update the state.
		if len(res.Validators) > 0 {
			vals, err := types.PB2TM.ValidatorUpdates(res.Validators)
			if err != nil {
				return sm.State{}, err
			}
			currentState.Validators = types.NewValidatorSet(vals)
			currentState.NextValidators = types.NewValidatorSet(vals).CopyIncrementProposerPriority(1)
		} else if len(genDoc.Validators) == 0 {
			// If validator set is not set in genesis and still empty after InitChain, exit.
			return sm.State{}, fmt.Errorf("validator set is nil in genesis and still empty after InitChain")
		}

		if res.ConsensusParams != nil {
			currentState.ConsensusParams = currentState.ConsensusParams.UpdateConsensusParams(res.ConsensusParams)
			currentState.Version.Consensus.App = currentState.ConsensusParams.Version.AppVersion
		}
		// We update the last results hash with the empty hash, to conform with RFC-6962.
		currentState.LastResultsHash = merkle.HashFromByteSlices(nil)

		// We now save the initial state to the stateStore
		if err := stateStore.Save(currentState); err != nil {
			return sm.State{}, err
		}
	}

	return currentState, nil
}

// replayBlocks loads blocks from appBlockHeight to storeBlockHeight and
// executes each block against the application. It then checks that the app hash
// produced from executing the block matches that of the next block. It does not
// mutate Tendermint state in anyway except for when mutateState is true in
// which case we persist the response from the final block.
func replayBlocks(
	state sm.State,
	proxyApp proxy.AppConns,
	blockStore sm.BlockStore,
	stateStore sm.Store,
	appBlockHeight,
	storeBlockHeight int64,
	mutateState bool,
	eventBus types.BlockEventPublisher,
	genDoc *types.GenesisDoc,
	logger log.Logger,
) error {
	var appHash []byte
	var err error
	finalBlock := storeBlockHeight
	if mutateState {
		finalBlock--
	}
	firstBlock := appBlockHeight + 1
	if firstBlock == 1 {
		firstBlock = state.InitialHeight
	}
	for i := firstBlock; i <= finalBlock; i++ {
		logger.Info("Applying block", "height", i)
		block := blockStore.LoadBlock(i)

		// Extra check to ensure the app was not changed in a way which changes
		// the app hash
		if !bytes.Equal(appHash, block.AppHash) {
			return fmt.Errorf("block.AppHash does not match AppHash at height %d during replay. Got %X, expected %X",
				block.Height, appHash, block.AppHash)
		}

		if i == finalBlock && !mutateState {
			// We emit events for the index services at the final block due to the sync issue when
			// the node shutdown during the block committing status.
			blockExec := sm.NewBlockExecutor(
				stateStore, logger, proxyApp.Consensus(), emptyMempool{}, sm.EmptyEvidencePool{}, blockStore)
			blockExec.SetEventBus(eventBus)
			appHash, err = sm.ExecCommitBlock(
				blockExec, proxyApp.Consensus(), block, logger, stateStore, genDoc.InitialHeight, state)
			if err != nil {
				return err
			}
		} else {
			appHash, err = sm.ExecCommitBlock(
				nil, proxyApp.Consensus(), block, logger, stateStore, genDoc.InitialHeight, state)
			if err != nil {
				return err
			}
		}
	}

	if mutateState {
		// sync the final block
		state, err = replayBlock(state, blockStore, stateStore, storeBlockHeight, proxyApp.Consensus(), eventBus, logger)
		if err != nil {
			return err
		}
		appHash = state.AppHash
	}

	return nil
}

// replayBlock uses the block executor to
func replayBlock(
	state sm.State,
	store sm.BlockStore,
	stateStore sm.Store,
	height int64,
	proxyApp proxy.AppConnConsensus,
	eventBus types.BlockEventPublisher,
	logger log.Logger,
) (sm.State, error) {
	block := store.LoadBlock(height)
	meta := store.LoadBlockMeta(height)

	// Use stubs for both mempool and evidence pool since no transactions nor
	// evidence are needed here - block already exists.
	blockExec := sm.NewBlockExecutor(stateStore, logger, proxyApp, emptyMempool{}, sm.EmptyEvidencePool{}, store)
	blockExec.SetEventBus(eventBus)

	var err error
	state, err = blockExec.ApplyBlock(state, meta.BlockID, block)
	if err != nil {
		return sm.State{}, err
	}

	return state, nil
}

//-----------------------------------------------------------------------------
// mockProxyApp uses ABCIResponses to give the right results.
//
// Useful because we don't want to call Commit() twice for the same block on
// the real app.

func newMockProxyApp(appHash []byte, abciResponses *tmstate.ABCIResponses) proxy.AppConnConsensus {
	clientCreator := proxy.NewLocalClientCreator(&mockProxyApp{
		appHash:       appHash,
		abciResponses: abciResponses,
	})
	cli, _ := clientCreator.NewABCIClient()
	err := cli.Start()
	if err != nil {
		panic(err)
	}
	return proxy.NewAppConnConsensus(cli)
}

type mockProxyApp struct {
	abci.BaseApplication

	appHash       []byte
	txCount       int
	abciResponses *tmstate.ABCIResponses
}

func (mock *mockProxyApp) DeliverTx(req abci.RequestDeliverTx) abci.ResponseDeliverTx {
	r := mock.abciResponses.DeliverTxs[mock.txCount]
	mock.txCount++
	if r == nil {
		return abci.ResponseDeliverTx{}
	}
	return *r
}

func (mock *mockProxyApp) EndBlock(req abci.RequestEndBlock) abci.ResponseEndBlock {
	mock.txCount = 0
	return *mock.abciResponses.EndBlock
}

func (mock *mockProxyApp) Commit() abci.ResponseCommit {
	return abci.ResponseCommit{Data: mock.appHash}
}

//-----------------------------------------------------------------------------

type emptyMempool struct{}

var _ mempl.Mempool = emptyMempool{}

func (emptyMempool) Lock()     {}
func (emptyMempool) Unlock()   {}
func (emptyMempool) Size() int { return 0 }
func (emptyMempool) CheckTx(_ context.Context, _ types.Tx, _ func(*abci.Response), _ mempl.TxInfo) error {
	return nil
}
func (emptyMempool) ReapMaxBytesMaxGas(_, _ int64) types.Txs { return types.Txs{} }
func (emptyMempool) ReapMaxTxs(n int) types.Txs              { return types.Txs{} }
func (emptyMempool) Update(
	_ int64,
	_ types.Txs,
	_ []*abci.ResponseDeliverTx,
	_ mempl.PreCheckFunc,
	_ mempl.PostCheckFunc,
) error {
	return nil
}
func (emptyMempool) Flush()                        {}
func (emptyMempool) FlushAppConn() error           { return nil }
func (emptyMempool) TxsAvailable() <-chan struct{} { return make(chan struct{}) }
func (emptyMempool) EnableTxsAvailable()           {}
func (emptyMempool) SizeBytes() int64              { return 0 }

func (emptyMempool) TxsFront() *clist.CElement    { return nil }
func (emptyMempool) TxsWaitChan() <-chan struct{} { return nil }

func (emptyMempool) InitWAL() error { return nil }
func (emptyMempool) CloseWAL()      {}
