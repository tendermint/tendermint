package state

import (
	"bytes"
	"fmt"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/fail"
	"github.com/tendermint/tendermint/libs/log"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

//-----------------------------------------------------------------------------
// BlockExecutor handles block execution and state updates.
// It exposes ApplyBlock(), which validates & executes the block, updates state w/ ABCI responses,
// then commits and updates the mempool atomically, then saves state.

// BlockExecutor provides the context and accessories for properly executing a block.
type BlockExecutor struct {
	// save state, validators, consensus params, abci responses here
	db dbm.DB

	// execute the app against this
	proxyApp proxy.AppConnConsensus

	// events
	eventBus types.BlockEventPublisher

	// manage the mempool lock during commit
	// and update both with block results after commit.
	mempool mempl.Mempool
	evpool  EvidencePool

	logger log.Logger

	metrics *Metrics

	// [peppermint] fast sync
	fastSyncFunc func() bool
}

type BlockExecutorOption func(executor *BlockExecutor)

func BlockExecutorWithMetrics(metrics *Metrics) BlockExecutorOption {
	return func(blockExec *BlockExecutor) {
		blockExec.metrics = metrics
	}
}

// NewBlockExecutor returns a new BlockExecutor with a NopEventBus.
// Call SetEventBus to provide one.
func NewBlockExecutor(db dbm.DB, logger log.Logger, proxyApp proxy.AppConnConsensus, mempool mempl.Mempool, evpool EvidencePool, options ...BlockExecutorOption) *BlockExecutor {
	res := &BlockExecutor{
		db:       db,
		proxyApp: proxyApp,
		eventBus: types.NopEventBus{},
		mempool:  mempool,
		evpool:   evpool,
		logger:   logger,
		metrics:  NopMetrics(),
	}

	for _, option := range options {
		option(res)
	}

	return res
}

func (blockExec *BlockExecutor) DB() dbm.DB {
	return blockExec.db
}

// SetFastSyncFunc sets fast sync function
func (blockExec *BlockExecutor) SetFastSyncFunc(f func() bool) {
	blockExec.fastSyncFunc = f
}

// SetEventBus - sets the event bus for publishing block related events.
// If not called, it defaults to types.NopEventBus.
func (blockExec *BlockExecutor) SetEventBus(eventBus types.BlockEventPublisher) {
	blockExec.eventBus = eventBus
}

// CreateProposalBlock calls state.MakeBlock with evidence from the evpool
// and txs from the mempool. The max bytes must be big enough to fit the commit.
// Up to 1/10th of the block space is allcoated for maximum sized evidence.
// The rest is given to txs, up to the max gas.
func (blockExec *BlockExecutor) CreateProposalBlock(
	height int64,
	state State, commit *types.Commit,
	proposerAddr []byte,
) (*types.Block, *types.PartSet) {

	maxBytes := state.ConsensusParams.Block.MaxBytes
	maxGas := state.ConsensusParams.Block.MaxGas

	// Fetch a limited amount of valid evidence
	maxNumEvidence, _ := types.MaxEvidencePerBlock(maxBytes)
	evidence := blockExec.evpool.PendingEvidence(maxNumEvidence)

	// Fetch a limited amount of valid txs
	maxDataBytes := types.MaxDataBytes(maxBytes, state.Validators.Size(), len(evidence))
	txs := blockExec.mempool.ReapMaxBytesMaxGas(maxDataBytes, maxGas)

	return state.MakeBlock(height, txs, commit, evidence, proposerAddr)
}

// ValidateBlock validates the given block against the given state.
// If the block is invalid, it returns an error.
// Validation does not mutate state, but does require historical information from the stateDB,
// ie. to verify evidence from a validator at an old height.
func (blockExec *BlockExecutor) ValidateBlock(state State, block *types.Block) error {
	return validateBlock(blockExec.evpool, blockExec.db, state, block)
}

// ApplyBlock validates the block against the state, executes it against the app,
// fires the relevant events, commits the app, and saves the new state and responses.
// It's the only function that needs to be called
// from outside this package to process and commit an entire block.
// It takes a blockID to avoid recomputing the parts hash.
func (blockExec *BlockExecutor) ApplyBlock(state State, blockID types.BlockID, block *types.Block) (State, error) {
	blockExec.logger.Debug("[Peppermint] Applying block", "height", block.Height, "numTxs", block.NumTxs)

	if err := blockExec.ValidateBlock(state, block); err != nil {
		return state, ErrInvalidBlock(err)
	}

	// Execute side deliver tx if node is fast syncing
	executeSideDeliverTx := true
	if blockExec.fastSyncFunc != nil {
		executeSideDeliverTx = !blockExec.fastSyncFunc()
	}

	startTime := time.Now().UnixNano()
	abciResponses, sideTxResponses, err := execBlockOnProxyApp(blockExec.logger, blockExec.proxyApp, block, blockExec.db, executeSideDeliverTx)
	endTime := time.Now().UnixNano()
	blockExec.metrics.BlockProcessingTime.Observe(float64(endTime-startTime) / 1000000)
	if err != nil {
		return state, ErrProxyAppConn(err)
	}

	fail.Fail() // XXX

	// Save the results before we commit.
	saveABCIResponses(blockExec.db, block.Height, abciResponses)

	fail.Fail() // XXX

	// validate the validator updates and convert to tendermint types
	abciValUpdates := abciResponses.EndBlock.ValidatorUpdates
	err = validateValidatorUpdates(abciValUpdates, state.ConsensusParams.Validator)
	if err != nil {
		return state, fmt.Errorf("Error in validator updates: %v", err)
	}
	validatorUpdates, err := types.PB2TM.ValidatorUpdates(abciValUpdates)
	if err != nil {
		return state, err
	}
	if len(validatorUpdates) > 0 {
		blockExec.logger.Info("Updates to validators", "updates", types.ValidatorListString(validatorUpdates))
	}

	// Update the state with the block and responses.
	state, err = updateState(state, blockID, &block.Header, abciResponses, validatorUpdates)
	if err != nil {
		return state, fmt.Errorf("Commit failed for application: %v", err)
	}

	// Lock mempool, commit app state, update mempoool.
	appHash, err := blockExec.Commit(state, block, abciResponses.DeliverTx)
	if err != nil {
		return state, fmt.Errorf("Commit failed for application: %v", err)
	}

	// Update evpool with the block and state.
	blockExec.evpool.Update(block, state)

	fail.Fail() // XXX

	// Save side tx responses
	state.SideTxResponses = sideTxResponses

	// Update the app hash and save the state.
	state.AppHash = appHash
	SaveState(blockExec.db, state)

	fail.Fail() // XXX

	// Events are fired after everything else.
	// NOTE: if we crash between Commit and Save, events wont be fired during replay
	fireEvents(blockExec.logger, blockExec.eventBus, block, abciResponses, validatorUpdates)

	return state, nil
}

// Commit locks the mempool, runs the ABCI Commit message, and updates the
// mempool.
// It returns the result of calling abci.Commit (the AppHash), and an error.
// The Mempool must be locked during commit and update because state is
// typically reset on Commit and old txs must be replayed against committed
// state before new txs are run in the mempool, lest they be invalid.
func (blockExec *BlockExecutor) Commit(
	state State,
	block *types.Block,
	deliverTxResponses []*abci.ResponseDeliverTx,
) ([]byte, error) {
	blockExec.mempool.Lock()
	defer blockExec.mempool.Unlock()

	// while mempool is Locked, flush to ensure all async requests have completed
	// in the ABCI app before Commit.
	err := blockExec.mempool.FlushAppConn()
	if err != nil {
		blockExec.logger.Error("Client error during mempool.FlushAppConn", "err", err)
		return nil, err
	}

	// Commit block, get hash back
	res, err := blockExec.proxyApp.CommitSync()
	if err != nil {
		blockExec.logger.Error(
			"Client error during proxyAppConn.CommitSync",
			"err", err,
		)
		return nil, err
	}
	// ResponseCommit has no error code - just data

	blockExec.logger.Info(
		"Committed state",
		"height", block.Height,
		"txs", block.NumTxs,
		"appHash", fmt.Sprintf("%X", res.Data),
	)

	// Update mempool.
	err = blockExec.mempool.Update(
		block.Height,
		block.Txs,
		deliverTxResponses,
		TxPreCheck(state),
		TxPostCheck(state),
	)

	return res.Data, err
}

//---------------------------------------------------------
// Helper functions for executing blocks and updating state

// Executes block's transactions on proxyAppConn.
// Returns a list of transaction results and updates to the validator set
func execBlockOnProxyApp(
	logger log.Logger,
	proxyAppConn proxy.AppConnConsensus,
	block *types.Block,
	stateDB dbm.DB,
	executeSideDeliverTx bool,
) (*ABCIResponses, []*types.SideTxResultWithData, error) {
	var validTxs, invalidTxs = 0, 0

	txIndex := 0
	abciResponses := NewABCIResponses(block)
	sideTxResponses := make([]*types.SideTxResultWithData, 0)

	hash := block.Hash()
	header := types.TM2PB.Header(&block.Header)

	// Execute transactions and get hash.
	proxyCb := func(req *abci.Request, res *abci.Response) {
		if r, ok := res.Value.(*abci.Response_DeliverTx); ok {
			// TODO: make use of res.Log
			// TODO: make use of this info
			// Blocks may include invalid txs.
			txRes := r.DeliverTx
			if txRes.Code == abci.CodeTypeOK {
				validTxs++
			} else {
				logger.Debug("Invalid tx", "code", txRes.Code, "log", txRes.Log)
				invalidTxs++
			}
			abciResponses.DeliverTx[txIndex] = txRes
			txIndex++
		}
	}
	proxyAppConn.SetResponseCallback(proxyCb)

	// get begin block validator info
	commitInfo, byzVals := getBeginBlockValidatorInfo(block, stateDB)

	// Begin block
	var err error
	abciResponses.BeginBlock, err = proxyAppConn.BeginBlockSync(abci.RequestBeginBlock{
		Hash:                hash,
		Header:              header,
		LastCommitInfo:      commitInfo,
		ByzantineValidators: byzVals,
	})
	if err != nil {
		logger.Error("Error in proxyAppConn.BeginBlock", "err", err)
		return nil, nil, err
	}

	//
	// Side begin block
	//

	// get side-tx results for begin side-block
	sideTxResults := getBeginSideBlockData(block, stateDB)

	// TODO get votes from last commit
	// Side hook for begin block
	sideBlockResponse, err := proxyAppConn.BeginSideBlockSync(abci.RequestBeginSideBlock{
		Hash:          hash,
		Header:        header,
		SideTxResults: sideTxResults,
	})
	if err != nil {
		logger.Error("Error in proxyAppConn.BeginSideBlock", "err", err)
		return nil, nil, err
	}

	abciResponses.BeginBlock.Events = append(abciResponses.BeginBlock.Events, sideBlockResponse.Events...)

	//
	// Deliver tx
	//

	// Run txs of block.
	for _, tx := range block.Txs {
		proxyAppConn.DeliverTxAsync(abci.RequestDeliverTx{Tx: tx})
		if err := proxyAppConn.Error(); err != nil {
			return nil, nil, err
		}
	}

	//
	// Deliver side-tx
	//

	// execute side deliver-tx when not syncing
	if executeSideDeliverTx {
		// Execute side-transactions and store in side-tx responses
		proxySideCb := func(req *abci.Request, res *abci.Response) {
			if vreq, okreq := req.Value.(*abci.Request_DeliverSideTx); okreq {
				if vres, okres := res.Value.(*abci.Response_DeliverSideTx); okres {

					txReq := vreq.DeliverSideTx
					txRes := vres.DeliverSideTx

					if txRes.Code == abci.CodeTypeOK && txRes.Result != abci.SideTxResultType_Skip {
						tx := types.Tx(txReq.Tx)
						// add into side tx responses
						sideTxResponses = append(sideTxResponses, &types.SideTxResultWithData{
							SideTxResult: types.SideTxResult{
								TxHash: tx.Hash(),
								Result: int32(txRes.Result),
							},
							Data: txRes.Data,
						})
					}

					// ignore invalid side-tx responses
				}
			}
		}
		proxyAppConn.SetResponseCallback(proxySideCb)

		// Run side-txs of block.
		for txIndex, tx := range block.Txs {
			txRes := abciResponses.DeliverTx[txIndex]

			// execute side-tx only if tx is valid
			if txRes.Code == abci.CodeTypeOK {
				proxyAppConn.DeliverSideTxAsync(abci.RequestDeliverSideTx{Tx: tx})
				if err := proxyAppConn.Error(); err != nil {
					return nil, nil, err
				}
			}
		}
	}

	//
	// End block
	//

	// End block.
	abciResponses.EndBlock, err = proxyAppConn.EndBlockSync(abci.RequestEndBlock{Height: block.Height})
	if err != nil {
		logger.Error("Error in proxyAppConn.EndBlock", "err", err)
		return nil, nil, err
	}

	logger.Info("Executed block", "height", block.Height, "validTxs", validTxs, "invalidTxs", invalidTxs)
	if len(sideTxResponses) > 0 {
		logger.Debug("Executed side block", "height", block.Height, "validSideTxs", len(sideTxResponses))
	}

	return abciResponses, sideTxResponses, nil
}

func getBeginBlockValidatorInfo(block *types.Block, stateDB dbm.DB) (abci.LastCommitInfo, []abci.Evidence) {
	voteInfos := make([]abci.VoteInfo, block.LastCommit.Size())
	byzVals := make([]abci.Evidence, len(block.Evidence.Evidence))
	var lastValSet *types.ValidatorSet
	var err error
	if block.Height > 1 {
		lastValSet, err = LoadValidators(stateDB, block.Height-1)
		if err != nil {
			panic(err) // shouldn't happen
		}

		// Sanity check that commit length matches validator set size -
		// only applies after first block

		precommitLen := block.LastCommit.Size()
		valSetLen := len(lastValSet.Validators)
		if precommitLen != valSetLen {
			// sanity check
			panic(fmt.Sprintf("precommit length (%d) doesn't match valset length (%d) at height %d\n\n%v\n\n%v",
				precommitLen, valSetLen, block.Height, block.LastCommit.Precommits, lastValSet.Validators))
		}
	} else {
		lastValSet = types.NewValidatorSet(nil)
	}

	for i, val := range lastValSet.Validators {
		var vote *types.CommitSig
		if i < len(block.LastCommit.Precommits) {
			vote = block.LastCommit.Precommits[i]
		}
		voteInfo := abci.VoteInfo{
			Validator:       types.TM2PB.Validator(val),
			SignedLastBlock: vote != nil,
		}
		voteInfos[i] = voteInfo
	}

	for i, ev := range block.Evidence.Evidence {
		// We need the validator set. We already did this in validateBlock.
		// TODO: Should we instead cache the valset in the evidence itself and add
		// `SetValidatorSet()` and `ToABCI` methods ?
		valset, err := LoadValidators(stateDB, ev.Height())
		if err != nil {
			panic(err) // shouldn't happen
		}
		byzVals[i] = types.TM2PB.Evidence(ev, valset, block.Time)
	}

	commitInfo := abci.LastCommitInfo{
		Round: int32(block.LastCommit.Round()),
		Votes: voteInfos,
	}
	return commitInfo, byzVals

}

func validateValidatorUpdates(abciUpdates []abci.ValidatorUpdate,
	params types.ValidatorParams) error {
	for _, valUpdate := range abciUpdates {
		if valUpdate.GetPower() < 0 {
			return fmt.Errorf("Voting power can't be negative %v", valUpdate)
		} else if valUpdate.GetPower() == 0 {
			// continue, since this is deleting the validator, and thus there is no
			// pubkey to check
			continue
		}

		// Check if validator's pubkey matches an ABCI type in the consensus params
		thisKeyType := valUpdate.PubKey.Type
		if !params.IsValidPubkeyType(thisKeyType) {
			return fmt.Errorf("Validator %v is using pubkey %s, which is unsupported for consensus",
				valUpdate, thisKeyType)
		}
	}
	return nil
}

// updateState returns a new State updated according to the header and responses.
func updateState(
	state State,
	blockID types.BlockID,
	header *types.Header,
	abciResponses *ABCIResponses,
	validatorUpdates []*types.Validator,
) (State, error) {

	// Copy the valset so we can apply changes from EndBlock
	// and update s.LastValidators and s.Validators.
	nValSet := state.NextValidators.Copy()

	// Update the validator set with the latest abciResponses.
	lastHeightValsChanged := state.LastHeightValidatorsChanged
	if len(validatorUpdates) > 0 {
		err := nValSet.UpdateWithChangeSet(validatorUpdates)
		if err != nil {
			return state, fmt.Errorf("Error changing validator set: %v", err)
		}
		// Change results from this height but only applies to the next next height.
		lastHeightValsChanged = header.Height + 1 + 1
	}

	// Update validator proposer priority and set state variables.
	nValSet.IncrementProposerPriority(1)

	// Update the params with the latest abciResponses.
	nextParams := state.ConsensusParams
	lastHeightParamsChanged := state.LastHeightConsensusParamsChanged
	if abciResponses.EndBlock.ConsensusParamUpdates != nil {
		// NOTE: must not mutate s.ConsensusParams
		nextParams = state.ConsensusParams.Update(abciResponses.EndBlock.ConsensusParamUpdates)
		err := nextParams.Validate()
		if err != nil {
			return state, fmt.Errorf("Error updating consensus params: %v", err)
		}
		// Change results from this height but only applies to the next height.
		lastHeightParamsChanged = header.Height + 1
	}

	// TODO: allow app to upgrade version
	nextVersion := state.Version

	// NOTE: the AppHash has not been populated.
	// It will be filled on state.Save.
	return State{
		Version:                          nextVersion,
		ChainID:                          state.ChainID,
		LastBlockHeight:                  header.Height,
		LastBlockTotalTx:                 state.LastBlockTotalTx + header.NumTxs,
		LastBlockID:                      blockID,
		LastBlockTime:                    header.Time,
		NextValidators:                   nValSet,
		Validators:                       state.NextValidators.Copy(),
		LastValidators:                   state.Validators.Copy(),
		LastHeightValidatorsChanged:      lastHeightValsChanged,
		ConsensusParams:                  nextParams,
		LastHeightConsensusParamsChanged: lastHeightParamsChanged,
		LastResultsHash:                  abciResponses.ResultsHash(),
		AppHash:                          nil,
	}, nil
}

// Fire NewBlock, NewBlockHeader.
// Fire TxEvent for every tx.
// NOTE: if Tendermint crashes before commit, some or all of these events may be published again.
func fireEvents(logger log.Logger, eventBus types.BlockEventPublisher, block *types.Block, abciResponses *ABCIResponses, validatorUpdates []*types.Validator) {
	eventBus.PublishEventNewBlock(types.EventDataNewBlock{
		Block:            block,
		ResultBeginBlock: *abciResponses.BeginBlock,
		ResultEndBlock:   *abciResponses.EndBlock,
	})
	eventBus.PublishEventNewBlockHeader(types.EventDataNewBlockHeader{
		Header:           block.Header,
		ResultBeginBlock: *abciResponses.BeginBlock,
		ResultEndBlock:   *abciResponses.EndBlock,
	})

	for i, tx := range block.Data.Txs {
		eventBus.PublishEventTx(types.EventDataTx{TxResult: types.TxResult{
			Height: block.Height,
			Index:  uint32(i),
			Tx:     tx,
			Result: *(abciResponses.DeliverTx[i]),
		}})
	}

	if len(validatorUpdates) > 0 {
		eventBus.PublishEventValidatorSetUpdates(
			types.EventDataValidatorSetUpdates{ValidatorUpdates: validatorUpdates})
	}
}

//----------------------------------------------------------------------------------------------------
// Execute block without state. TODO: eliminate

// ExecCommitBlock executes and commits a block on the proxyApp without validating or mutating the state.
// It returns the application root hash (result of abci.Commit).
func ExecCommitBlock(
	appConnConsensus proxy.AppConnConsensus,
	block *types.Block,
	logger log.Logger,
	stateDB dbm.DB,
) ([]byte, error) {
	logger.Info("[Peppermint] Exec commit block", "height", block.Height)
	_, _, err := execBlockOnProxyApp(logger, appConnConsensus, block, stateDB, false)
	if err != nil {
		logger.Error("Error executing block on proxy app", "height", block.Height, "err", err)
		return nil, err
	}
	// Commit block, get hash back
	res, err := appConnConsensus.CommitSync()
	if err != nil {
		logger.Error("Client error during proxyAppConn.CommitSync", "err", res)
		return nil, err
	}
	// ResponseCommit has no error or log, just data
	return res.Data, nil
}

//
// Side channel utils
//

func getBeginSideBlockData(block *types.Block, stateDB dbm.DB) []abci.SideTxResult {
	// returns [
	//   {
	//     txHash: txHash,
	//     sigs: [
	//       {
	//         result: 1,
	//         sig: 2
	//       },
	//       ...
	//     ]
	//   },
	//   ...
	// ]

	// prepare result
	result := make([]abci.SideTxResult, 0)

	// return if prev block is empty result (mostly block 0)
	if block == nil || block.Height <= 2 {
		return make([]abci.SideTxResult, 0)
	}

	// iterate all votes
	for _, vote := range block.LastCommit.Precommits {
		if vote != nil {
			txMapping := make(map[int]bool)
			for _, sideTxResult := range vote.SideTxResults {
				// find if result object is already created
				resultIndex := -1
				for i, rr := range result {
					if bytes.Equal(rr.TxHash, sideTxResult.TxHash) {
						resultIndex = i
						break
					}
				}

				// create tx-hash based object, if not found yet
				if resultIndex == -1 {
					result = append(result, abci.SideTxResult{
						TxHash: sideTxResult.TxHash,
						Sigs:   make([]abci.SideTxSig, 0),
					})
					// set new result index
					resultIndex = len(result) - 1
				}

				// if tx is not processed for current vote, add it into sigs for particular side-tx result
				if _, ok := txMapping[resultIndex]; !ok {
					// get result object from result index
					result[resultIndex].Sigs = append(result[resultIndex].Sigs, abci.SideTxSig{
						Result:  abci.SideTxResultType(sideTxResult.Result),
						Sig:     sideTxResult.Sig,
						Address: vote.ValidatorAddress,
					})

					// add tx hash for the record for particular vote to avoid duplicate votes
					txMapping[resultIndex] = true
				}
			}
		}
	}

	return result
}
