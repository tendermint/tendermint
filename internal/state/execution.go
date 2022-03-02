package state

import (
	"context"
	"errors"
	"fmt"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/internal/proxy"
	"github.com/tendermint/tendermint/libs/log"
	tmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------------------
// BlockExecutor handles block execution and state updates.
// It exposes ApplyBlock(), which validates & executes the block, updates state w/ ABCI responses,
// then commits and updates the mempool atomically, then saves state.

// BlockExecutor provides the context and accessories for properly executing a block.
type BlockExecutor struct {
	// save state, validators, consensus params, abci responses here
	store Store

	// use blockstore for the pruning functions.
	blockStore BlockStore

	// execute the app against this
	proxyApp proxy.AppConnConsensus

	// events
	eventBus types.BlockEventPublisher

	// manage the mempool lock during commit
	// and update both with block results after commit.
	mempool mempool.Mempool
	evpool  EvidencePool

	logger  log.Logger
	metrics *Metrics

	// cache the verification results over a single height
	cache map[string]struct{}
}

type BlockExecutorOption func(executor *BlockExecutor)

func BlockExecutorWithMetrics(metrics *Metrics) BlockExecutorOption {
	return func(blockExec *BlockExecutor) {
		blockExec.metrics = metrics
	}
}

// NewBlockExecutor returns a new BlockExecutor with a NopEventBus.
// Call SetEventBus to provide one.
func NewBlockExecutor(
	stateStore Store,
	logger log.Logger,
	proxyApp proxy.AppConnConsensus,
	pool mempool.Mempool,
	evpool EvidencePool,
	blockStore BlockStore,
	options ...BlockExecutorOption,
) *BlockExecutor {
	res := &BlockExecutor{
		store:      stateStore,
		proxyApp:   proxyApp,
		eventBus:   eventbus.NopEventBus{},
		mempool:    pool,
		evpool:     evpool,
		logger:     logger,
		metrics:    NopMetrics(),
		cache:      make(map[string]struct{}),
		blockStore: blockStore,
	}

	for _, option := range options {
		option(res)
	}

	return res
}

func (blockExec *BlockExecutor) Store() Store {
	return blockExec.store
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
//
// Contract: application will not return more bytes than are sent over the wire.
func (blockExec *BlockExecutor) CreateProposalBlock(
	ctx context.Context,
	height int64,
	state State, commit *types.Commit,
	proposerAddr []byte,
	votes []*types.Vote,
) (*types.Block, *types.PartSet, error) {

	maxBytes := state.ConsensusParams.Block.MaxBytes
	maxGas := state.ConsensusParams.Block.MaxGas

	evidence, evSize := blockExec.evpool.PendingEvidence(state.ConsensusParams.Evidence.MaxBytes)

	// Fetch a limited amount of valid txs
	maxDataBytes := types.MaxDataBytes(maxBytes, evSize, state.Validators.Size())

	txs := blockExec.mempool.ReapMaxBytesMaxGas(maxDataBytes, maxGas)

	preparedProposal, err := blockExec.proxyApp.PrepareProposal(
		ctx,
		abci.RequestPrepareProposal{
			BlockData:     txs.ToSliceOfBytes(),
			BlockDataSize: maxDataBytes,
			Votes:         types.VotesToProto(votes),
		},
	)
	if err != nil {
		// The App MUST ensure that only valid (and hence 'processable') transactions
		// enter the mempool. Hence, at this point, we can't have any non-processable
		// transaction causing an error.
		//
		// Also, the App can simply skip any transaction that could cause any kind of trouble.
		// Either way, we can not recover in a meaningful way, unless we skip proposing
		// this block, repair what caused the error and try again. Hence, we panic on
		// purpose for now.
		panic(err)
	}
	newTxs := preparedProposal.GetBlockData()
	var txSize int
	for _, tx := range newTxs {
		txSize += len(tx)

		if maxDataBytes < int64(txSize) {
			panic("block data exceeds max amount of allowed bytes")
		}
	}

	modifiedTxs := types.ToTxs(preparedProposal.GetBlockData())

	return state.MakeBlock(height, modifiedTxs, commit, evidence, proposerAddr)
}

func (blockExec *BlockExecutor) ProcessProposal(
	ctx context.Context,
	block *types.Block,
	state State,
) (bool, error) {
	req := abci.RequestProcessProposal{
		Hash:                block.Header.Hash(),
		Header:              *block.Header.ToProto(),
		Txs:                 block.Data.Txs.ToSliceOfBytes(),
		LastCommitInfo:      buildLastCommitInfo(block, blockExec.store, state.InitialHeight),
		ByzantineValidators: block.Evidence.ToABCI(),
	}

	resp, err := blockExec.proxyApp.ProcessProposal(ctx, req)
	if err != nil {
		return false, ErrInvalidBlock(err)
	}

	return resp.Accept, nil
}

// ValidateBlock validates the given block against the given state.
// If the block is invalid, it returns an error.
// Validation does not mutate state, but does require historical information from the stateDB,
// ie. to verify evidence from a validator at an old height.
func (blockExec *BlockExecutor) ValidateBlock(ctx context.Context, state State, block *types.Block) error {
	hash := block.Hash()
	if _, ok := blockExec.cache[hash.String()]; ok {
		return nil
	}

	err := validateBlock(state, block)
	if err != nil {
		return err
	}

	err = blockExec.evpool.CheckEvidence(ctx, block.Evidence)
	if err != nil {
		return err
	}

	blockExec.cache[hash.String()] = struct{}{}
	return nil
}

// ApplyBlock validates the block against the state, executes it against the app,
// fires the relevant events, commits the app, and saves the new state and responses.
// It returns the new state.
// It's the only function that needs to be called
// from outside this package to process and commit an entire block.
// It takes a blockID to avoid recomputing the parts hash.
func (blockExec *BlockExecutor) ApplyBlock(
	ctx context.Context,
	state State,
	blockID types.BlockID,
	block *types.Block,
) (State, error) {

	// validate the block if we haven't already
	if err := blockExec.ValidateBlock(ctx, state, block); err != nil {
		return state, ErrInvalidBlock(err)
	}

	startTime := time.Now().UnixNano()
	abciResponses, err := execBlockOnProxyApp(ctx,
		blockExec.logger, blockExec.proxyApp, block, blockExec.store, state.InitialHeight,
	)
	endTime := time.Now().UnixNano()
	blockExec.metrics.BlockProcessingTime.Observe(float64(endTime-startTime) / 1000000)
	if err != nil {
		return state, ErrProxyAppConn(err)
	}

	// Save the results before we commit.
	if err := blockExec.store.SaveABCIResponses(block.Height, abciResponses); err != nil {
		return state, err
	}

	// validate the validator updates and convert to tendermint types
	abciValUpdates := abciResponses.FinalizeBlock.ValidatorUpdates
	err = validateValidatorUpdates(abciValUpdates, state.ConsensusParams.Validator)
	if err != nil {
		return state, fmt.Errorf("error in validator updates: %w", err)
	}

	validatorUpdates, err := types.PB2TM.ValidatorUpdates(abciValUpdates)
	if err != nil {
		return state, err
	}
	if len(validatorUpdates) > 0 {
		blockExec.logger.Debug("updates to validators", "updates", types.ValidatorListString(validatorUpdates))
	}

	// Update the state with the block and responses.
	state, err = updateState(state, blockID, &block.Header, abciResponses, validatorUpdates)
	if err != nil {
		return state, fmt.Errorf("commit failed for application: %w", err)
	}

	// Lock mempool, commit app state, update mempoool.
	appHash, retainHeight, err := blockExec.Commit(ctx, state, block, abciResponses.FinalizeBlock.Txs)
	if err != nil {
		return state, fmt.Errorf("commit failed for application: %w", err)
	}

	// Update evpool with the latest state.
	blockExec.evpool.Update(ctx, state, block.Evidence)

	// Update the app hash and save the state.
	state.AppHash = appHash
	if err := blockExec.store.Save(state); err != nil {
		return state, err
	}

	// Prune old heights, if requested by ABCI app.
	if retainHeight > 0 {
		pruned, err := blockExec.pruneBlocks(retainHeight)
		if err != nil {
			blockExec.logger.Error("failed to prune blocks", "retain_height", retainHeight, "err", err)
		} else {
			blockExec.logger.Debug("pruned blocks", "pruned", pruned, "retain_height", retainHeight)
		}
	}

	// reset the verification cache
	blockExec.cache = make(map[string]struct{})

	// Events are fired after everything else.
	// NOTE: if we crash between Commit and Save, events wont be fired during replay
	fireEvents(ctx, blockExec.logger, blockExec.eventBus, block, blockID, abciResponses, validatorUpdates)

	return state, nil
}

func (blockExec *BlockExecutor) ExtendVote(ctx context.Context, vote *types.Vote) (types.VoteExtension, error) {
	req := abci.RequestExtendVote{
		Vote: vote.ToProto(),
	}

	resp, err := blockExec.proxyApp.ExtendVote(ctx, req)
	if err != nil {
		return types.VoteExtension{}, err
	}
	return types.VoteExtensionFromProto(resp.VoteExtension), nil
}

func (blockExec *BlockExecutor) VerifyVoteExtension(ctx context.Context, vote *types.Vote) error {
	req := abci.RequestVerifyVoteExtension{
		Vote: vote.ToProto(),
	}

	resp, err := blockExec.proxyApp.VerifyVoteExtension(ctx, req)
	if err != nil {
		return err
	}

	if resp.IsErr() {
		return types.ErrVoteInvalidExtension
	}

	return nil
}

// Commit locks the mempool, runs the ABCI Commit message, and updates the
// mempool.
// It returns the result of calling abci.Commit (the AppHash) and the height to retain (if any).
// The Mempool must be locked during commit and update because state is
// typically reset on Commit and old txs must be replayed against committed
// state before new txs are run in the mempool, lest they be invalid.
func (blockExec *BlockExecutor) Commit(
	ctx context.Context,
	state State,
	block *types.Block,
	deliverTxResponses []*abci.ResponseDeliverTx,
) ([]byte, int64, error) {
	blockExec.mempool.Lock()
	defer blockExec.mempool.Unlock()

	// while mempool is Locked, flush to ensure all async requests have completed
	// in the ABCI app before Commit.
	err := blockExec.mempool.FlushAppConn(ctx)
	if err != nil {
		blockExec.logger.Error("client error during mempool.FlushAppConn", "err", err)
		return nil, 0, err
	}

	// Commit block, get hash back
	res, err := blockExec.proxyApp.Commit(ctx)
	if err != nil {
		blockExec.logger.Error("client error during proxyAppConn.Commit", "err", err)
		return nil, 0, err
	}

	// ResponseCommit has no error code - just data
	blockExec.logger.Info(
		"committed state",
		"height", block.Height,
		"num_txs", len(block.Txs),
		"app_hash", fmt.Sprintf("%X", res.Data),
	)

	// Update mempool.
	err = blockExec.mempool.Update(
		ctx,
		block.Height,
		block.Txs,
		deliverTxResponses,
		TxPreCheck(state),
		TxPostCheck(state),
	)

	return res.Data, res.RetainHeight, err
}

//---------------------------------------------------------
// Helper functions for executing blocks and updating state

// Executes block's transactions on proxyAppConn.
// Returns a list of transaction results and updates to the validator set
func execBlockOnProxyApp(
	ctx context.Context,
	logger log.Logger,
	proxyAppConn proxy.AppConnConsensus,
	block *types.Block,
	store Store,
	initialHeight int64,
) (*tmstate.ABCIResponses, error) {
	abciResponses := new(tmstate.ABCIResponses)
	abciResponses.FinalizeBlock = &abci.ResponseFinalizeBlock{}
	dtxs := make([]*abci.ResponseDeliverTx, len(block.Txs))
	abciResponses.FinalizeBlock.Txs = dtxs

	// Begin block
	var err error
	pbh := block.Header.ToProto()
	if pbh == nil {
		return nil, errors.New("nil header")
	}

	abciResponses.FinalizeBlock, err = proxyAppConn.FinalizeBlock(
		ctx,
		abci.RequestFinalizeBlock{
			Txs:                 block.Txs.ToSliceOfBytes(),
			Hash:                block.Hash(),
			Header:              *pbh,
			Height:              block.Height,
			LastCommitInfo:      buildLastCommitInfo(block, store, initialHeight),
			ByzantineValidators: block.Evidence.ToABCI(),
		},
	)
	if err != nil {
		logger.Error("error in proxyAppConn.FinalizeBlock", "err", err)
		return nil, err
	}
	logger.Info("executed block", "height", block.Height)
	return abciResponses, nil
}

func buildLastCommitInfo(block *types.Block, store Store, initialHeight int64) abci.LastCommitInfo {
	if block.Height == initialHeight {
		// there is no last commmit for the initial height.
		// return an empty value.
		return abci.LastCommitInfo{}
	}

	lastValSet, err := store.LoadValidators(block.Height - 1)
	if err != nil {
		panic(err)
	}

	var (
		commitSize = block.LastCommit.Size()
		valSetLen  = len(lastValSet.Validators)
	)

	// ensure that the size of the validator set in the last commit matches
	// the size of the validator set in the state store.
	if commitSize != valSetLen {
		panic(fmt.Sprintf(
			"commit size (%d) doesn't match validator set length (%d) at height %d\n\n%v\n\n%v",
			commitSize, valSetLen, block.Height, block.LastCommit.Signatures, lastValSet.Validators,
		))
	}

	votes := make([]abci.VoteInfo, block.LastCommit.Size())
	for i, val := range lastValSet.Validators {
		commitSig := block.LastCommit.Signatures[i]
		votes[i] = abci.VoteInfo{
			Validator:       types.TM2PB.Validator(val),
			SignedLastBlock: !commitSig.Absent(),
		}
	}

	return abci.LastCommitInfo{
		Round: block.LastCommit.Round,
		Votes: votes,
	}
}

func validateValidatorUpdates(abciUpdates []abci.ValidatorUpdate,
	params types.ValidatorParams) error {
	for _, valUpdate := range abciUpdates {
		if valUpdate.GetPower() < 0 {
			return fmt.Errorf("voting power can't be negative %v", valUpdate)
		} else if valUpdate.GetPower() == 0 {
			// continue, since this is deleting the validator, and thus there is no
			// pubkey to check
			continue
		}

		// Check if validator's pubkey matches an ABCI type in the consensus params
		pk, err := encoding.PubKeyFromProto(valUpdate.PubKey)
		if err != nil {
			return err
		}

		if !params.IsValidPubkeyType(pk.Type()) {
			return fmt.Errorf("validator %v is using pubkey %s, which is unsupported for consensus",
				valUpdate, pk.Type())
		}
	}
	return nil
}

// updateState returns a new State updated according to the header and responses.
func updateState(
	state State,
	blockID types.BlockID,
	header *types.Header,
	abciResponses *tmstate.ABCIResponses,
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
			return state, fmt.Errorf("error changing validator set: %w", err)
		}
		// Change results from this height but only applies to the next next height.
		lastHeightValsChanged = header.Height + 1 + 1
	}

	// Update validator proposer priority and set state variables.
	nValSet.IncrementProposerPriority(1)

	// Update the params with the latest abciResponses.
	nextParams := state.ConsensusParams
	lastHeightParamsChanged := state.LastHeightConsensusParamsChanged
	if abciResponses.FinalizeBlock.ConsensusParamUpdates != nil {
		// NOTE: must not mutate s.ConsensusParams
		nextParams = state.ConsensusParams.UpdateConsensusParams(abciResponses.FinalizeBlock.ConsensusParamUpdates)
		err := nextParams.ValidateConsensusParams()
		if err != nil {
			return state, fmt.Errorf("error updating consensus params: %w", err)
		}

		state.Version.Consensus.App = nextParams.Version.AppVersion

		// Change results from this height but only applies to the next height.
		lastHeightParamsChanged = header.Height + 1
	}

	nextVersion := state.Version

	// NOTE: the AppHash and the VoteExtension has not been populated.
	// It will be filled on state.Save.
	return State{
		Version:                          nextVersion,
		ChainID:                          state.ChainID,
		InitialHeight:                    state.InitialHeight,
		LastBlockHeight:                  header.Height,
		LastBlockID:                      blockID,
		LastBlockTime:                    header.Time,
		NextValidators:                   nValSet,
		Validators:                       state.NextValidators.Copy(),
		LastValidators:                   state.Validators.Copy(),
		LastHeightValidatorsChanged:      lastHeightValsChanged,
		ConsensusParams:                  nextParams,
		LastHeightConsensusParamsChanged: lastHeightParamsChanged,
		LastResultsHash:                  ABCIResponsesResultsHash(abciResponses),
		AppHash:                          nil,
	}, nil
}

// Fire NewBlock, NewBlockHeader.
// Fire TxEvent for every tx.
// NOTE: if Tendermint crashes before commit, some or all of these events may be published again.
func fireEvents(
	ctx context.Context,
	logger log.Logger,
	eventBus types.BlockEventPublisher,
	block *types.Block,
	blockID types.BlockID,
	abciResponses *tmstate.ABCIResponses,
	validatorUpdates []*types.Validator,
) {
	if err := eventBus.PublishEventNewBlock(ctx, types.EventDataNewBlock{
		Block:               block,
		BlockID:             blockID,
		ResultFinalizeBlock: *abciResponses.FinalizeBlock,
	}); err != nil {
		logger.Error("failed publishing new block", "err", err)
	}

	if err := eventBus.PublishEventNewBlockHeader(ctx, types.EventDataNewBlockHeader{
		Header:              block.Header,
		NumTxs:              int64(len(block.Txs)),
		ResultFinalizeBlock: *abciResponses.FinalizeBlock,
	}); err != nil {
		logger.Error("failed publishing new block header", "err", err)
	}

	if len(block.Evidence) != 0 {
		for _, ev := range block.Evidence {
			if err := eventBus.PublishEventNewEvidence(ctx, types.EventDataNewEvidence{
				Evidence: ev,
				Height:   block.Height,
			}); err != nil {
				logger.Error("failed publishing new evidence", "err", err)
			}
		}
	}

	// sanity check
	if len(abciResponses.FinalizeBlock.Txs) != len(block.Data.Txs) {
		panic(fmt.Sprintf("number of TXs (%d) and ABCI TX responses (%d) do not match",
			len(block.Data.Txs), len(abciResponses.FinalizeBlock.Txs)))
	}

	for i, tx := range block.Data.Txs {
		if err := eventBus.PublishEventTx(ctx, types.EventDataTx{
			TxResult: abci.TxResult{
				Height: block.Height,
				Index:  uint32(i),
				Tx:     tx,
				Result: *(abciResponses.FinalizeBlock.Txs[i]),
			},
		}); err != nil {
			logger.Error("failed publishing event TX", "err", err)
		}
	}

	if len(validatorUpdates) > 0 {
		if err := eventBus.PublishEventValidatorSetUpdates(ctx,
			types.EventDataValidatorSetUpdates{ValidatorUpdates: validatorUpdates}); err != nil {
			logger.Error("failed publishing event", "err", err)
		}
	}
}

//----------------------------------------------------------------------------------------------------
// Execute block without state. TODO: eliminate

// ExecCommitBlock executes and commits a block on the proxyApp without validating or mutating the state.
// It returns the application root hash (result of abci.Commit).
func ExecCommitBlock(
	ctx context.Context,
	be *BlockExecutor,
	appConnConsensus proxy.AppConnConsensus,
	block *types.Block,
	logger log.Logger,
	store Store,
	initialHeight int64,
	s State,
) ([]byte, error) {
	abciResponses, err := execBlockOnProxyApp(ctx, logger, appConnConsensus, block, store, initialHeight)
	if err != nil {
		logger.Error("failed executing block on proxy app", "height", block.Height, "err", err)
		return nil, err
	}

	// the BlockExecutor condition is using for the final block replay process.
	if be != nil {
		abciValUpdates := abciResponses.FinalizeBlock.ValidatorUpdates
		err = validateValidatorUpdates(abciValUpdates, s.ConsensusParams.Validator)
		if err != nil {
			logger.Error("err", err)
			return nil, err
		}
		validatorUpdates, err := types.PB2TM.ValidatorUpdates(abciValUpdates)
		if err != nil {
			logger.Error("err", err)
			return nil, err
		}

		bps, err := block.MakePartSet(types.BlockPartSizeBytes)
		if err != nil {
			return nil, err
		}

		blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: bps.Header()}
		fireEvents(ctx, be.logger, be.eventBus, block, blockID, abciResponses, validatorUpdates)
	}

	// Commit block, get hash back
	res, err := appConnConsensus.Commit(ctx)
	if err != nil {
		logger.Error("client error during proxyAppConn.Commit", "err", res)
		return nil, err
	}

	// ResponseCommit has no error or log, just data
	return res.Data, nil
}

func (blockExec *BlockExecutor) pruneBlocks(retainHeight int64) (uint64, error) {
	base := blockExec.blockStore.Base()
	if retainHeight <= base {
		return 0, nil
	}
	pruned, err := blockExec.blockStore.PruneBlocks(retainHeight)
	if err != nil {
		return 0, fmt.Errorf("failed to prune block store: %w", err)
	}

	err = blockExec.Store().PruneStates(retainHeight)
	if err != nil {
		return 0, fmt.Errorf("failed to prune state store: %w", err)
	}
	return pruned, nil
}
