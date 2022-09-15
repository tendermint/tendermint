package state

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	abciclient "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/mempool"
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
	appClient abciclient.Client

	// events
	eventBus types.BlockEventPublisher

	// manage the mempool lock during commit
	// and update both with block results after commit.
	mempool mempool.Mempool
	evpool  EvidencePool

	logger  log.Logger
	metrics *Metrics

	appHashSize int

	// cache the verification results over a single height
	cache map[string]struct{}
}

// NewBlockExecutor returns a new BlockExecutor with the passed-in EventBus.
func NewBlockExecutor(
	stateStore Store,
	logger log.Logger,
	appClient abciclient.Client,
	pool mempool.Mempool,
	evpool EvidencePool,
	blockStore BlockStore,
	eventBus *eventbus.EventBus,
	metrics *Metrics,
) *BlockExecutor {
	return &BlockExecutor{
		eventBus:    eventBus,
		store:       stateStore,
		appClient:   appClient,
		mempool:     pool,
		evpool:      evpool,
		logger:      logger,
		metrics:     metrics,
		cache:       make(map[string]struct{}),
		blockStore:  blockStore,
		appHashSize: 32, // TODO change on constant
	}
}

func (blockExec *BlockExecutor) Store() Store {
	return blockExec.store
}

// CreateProposalBlock calls state.MakeBlock with evidence from the evpool
// and txs from the mempool. The max bytes must be big enough to fit the commit.
// Up to 1/10th of the block space is allocated for maximum sized evidence.
// The rest is given to txs, up to the max gas.
//
// Contract: application will not return more bytes than are sent over the wire.
func (blockExec *BlockExecutor) CreateProposalBlock(
	ctx context.Context,
	height int64,
	state State,
	commit *types.Commit,
	proposerProTxHash []byte,
	proposedAppVersion uint64,
) (*types.Block, CurrentRoundState, error) {
	maxBytes := state.ConsensusParams.Block.MaxBytes
	maxGas := state.ConsensusParams.Block.MaxGas

	evidence, evSize := blockExec.evpool.PendingEvidence(state.ConsensusParams.Evidence.MaxBytes)

	// Fetch a limited amount of valid txs
	maxDataBytes := types.MaxDataBytes(maxBytes, crypto.BLS12381, evSize, state.Validators.Size())

	// Pass proposed app version only if it's higher than current network app version
	if proposedAppVersion <= state.Version.Consensus.App {
		proposedAppVersion = 0
	}

	txs := blockExec.mempool.ReapMaxBytesMaxGas(maxDataBytes, maxGas)
	block := state.MakeBlock(height, txs, commit, evidence, proposerProTxHash, proposedAppVersion)

	localLastCommit := buildLastCommitInfo(block, state.InitialHeight)
	version := block.Version.ToProto()
	rpp, err := blockExec.appClient.PrepareProposal(
		ctx,
		&abci.RequestPrepareProposal{
			MaxTxBytes:         maxDataBytes,
			Txs:                block.Txs.ToSliceOfBytes(),
			LocalLastCommit:    abci.ExtendedCommitInfo(localLastCommit),
			Misbehavior:        block.Evidence.ToABCI(),
			Height:             block.Height,
			Time:               block.Time,
			NextValidatorsHash: block.NextValidatorsHash,

			// Dash's fields
			CoreChainLockedHeight: block.CoreChainLockedHeight,
			ProposerProTxHash:     block.ProposerProTxHash,
			ProposedAppVersion:    block.ProposedAppVersion,
			Version:               &version,
		},
	)
	if err != nil {
		// The App MUST ensure that only valid (and hence 'processable') transactions
		// enter the mempool. Hence, at this point, we can't have any non-processable
		// transaction causing an error.
		//
		// Also, the App can simply skip any transaction that could cause any kind of trouble.
		// Either way, we cannot recover in a meaningful way, unless we skip proposing
		// this block, repair what caused the error and try again. Hence, we return an
		// error for now (the production code calling this function is expected to panic).
		return nil, CurrentRoundState{}, err
	}

	if err := rpp.Validate(); err != nil {
		return nil, CurrentRoundState{}, fmt.Errorf("PrepareProposal responded with invalid response: %w", err)
	}

	txrSet := types.NewTxRecordSet(rpp.TxRecords)

	if err := txrSet.Validate(maxDataBytes, block.Txs); err != nil {
		return nil, CurrentRoundState{}, err
	}

	for _, rtx := range txrSet.RemovedTxs() {
		if err := blockExec.mempool.RemoveTxByKey(rtx.Key()); err != nil {
			blockExec.logger.Debug("error removing transaction from the mempool", "error", err, "tx hash", rtx.Hash())
		}
	}
	itxs := txrSet.IncludedTxs()

	// TODO: validate rpp.TxResults

	block = state.MakeBlock(
		height,
		itxs,
		commit,
		evidence,
		proposerProTxHash,
		proposedAppVersion,
	)

	// update some round state data
	stateChanges, err := state.NewStateChangeset(ctx, rpp)
	if err != nil {
		return nil, CurrentRoundState{}, err
	}
	err = stateChanges.UpdateBlock(block)
	if err != nil {
		return nil, CurrentRoundState{}, err
	}

	return block, stateChanges, nil
}

// ProcessProposal sends the proposal to ABCI App and verifies the response
func (blockExec *BlockExecutor) ProcessProposal(
	ctx context.Context,
	block *types.Block,
	state State,
	verify bool,
) (CurrentRoundState, error) {
	version := block.Version.ToProto()
	resp, err := blockExec.appClient.ProcessProposal(ctx, &abci.RequestProcessProposal{
		Hash:               block.Header.Hash(),
		Height:             block.Header.Height,
		Time:               block.Header.Time,
		Txs:                block.Data.Txs.ToSliceOfBytes(),
		ProposedLastCommit: buildLastCommitInfo(block, state.InitialHeight),
		Misbehavior:        block.Evidence.ToABCI(),
		NextValidatorsHash: block.NextValidatorsHash,

		// Dash's fields
		ProposerProTxHash:     block.ProposerProTxHash,
		CoreChainLockedHeight: block.CoreChainLockedHeight,
		ProposedAppVersion:    block.ProposedAppVersion,
		Version:               &version,
	})
	if err != nil {
		return CurrentRoundState{}, err
	}
	if resp.IsStatusUnknown() {
		return CurrentRoundState{}, fmt.Errorf("ProcessProposal responded with status %s", resp.Status.String())
	}
	if err := resp.Validate(); err != nil {
		return CurrentRoundState{}, fmt.Errorf("ProcessProposal responded with invalid response: %w", err)
	}
	accepted := resp.IsAccepted()
	if !accepted {
		return CurrentRoundState{}, ErrBlockRejected
	}

	// update some round state data
	stateChanges, err := state.NewStateChangeset(ctx, resp)
	if err != nil {
		return stateChanges, err
	}
	// we force the abci app to return only 32 byte app hashes (set to 20 temporarily)
	if resp.AppHash != nil && len(resp.AppHash) != blockExec.appHashSize {
		blockExec.logger.Error(
			"Client returned invalid app hash size", "bytesLength", len(resp.AppHash),
		)
		return stateChanges, errors.New("invalid App Hash size")
	}

	if verify {
		// Here we check if the ProcessProposal response matches
		// block received from proposer, eg. if `uncommittedState`
		// fields are the same as `block` fields
		err = blockExec.ValidateBlockWithRoundState(ctx, state, stateChanges, block)
		if err != nil {
			return stateChanges, ErrInvalidBlock{err}
		}
	}

	return stateChanges, nil
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

func (blockExec *BlockExecutor) ValidateBlockWithRoundState(
	ctx context.Context,
	state State,
	uncommittedState CurrentRoundState,
	block *types.Block,
) error {
	err := blockExec.ValidateBlock(ctx, state, block)
	if err != nil {
		return err
	}

	// Validate app info
	if uncommittedState.AppHash != nil && !bytes.Equal(block.AppHash, uncommittedState.AppHash) {
		return fmt.Errorf("wrong Block.Header.AppHash. Expected %X, got %X",
			uncommittedState.AppHash,
			block.AppHash,
		)
	}
	if uncommittedState.ResultsHash != nil && !bytes.Equal(block.ResultsHash, uncommittedState.ResultsHash) {
		return fmt.Errorf("wrong Block.Header.ResultsHash.  Expected %X, got %X",
			uncommittedState.ResultsHash,
			block.ResultsHash,
		)
	}

	if block.Height > state.InitialHeight {
		if err := state.LastValidators.VerifyCommit(
			state.ChainID, state.LastBlockID, state.LastStateID, block.Height-1, block.LastCommit); err != nil {
			return fmt.Errorf("error validating block: %w", err)
		}
	}
	if !bytes.Equal(block.NextValidatorsHash, uncommittedState.NextValidators.Hash()) {
		return fmt.Errorf("wrong Block.Header.NextValidatorsHash. Expected %X, got %v",
			uncommittedState.NextValidators.Hash(),
			block.NextValidatorsHash,
		)
	}

	return validateCoreChainLock(block, state)
}

// ValidateBlockChainLock validates the given block chain lock against the given state.
// If the block is invalid, it returns an error.
// Validation does not mutate state, but does require historical information from the stateDB,
// ie. to verify evidence from a validator at an old height.
func (blockExec *BlockExecutor) ValidateBlockChainLock(ctx context.Context, state State, block *types.Block) error {
	return validateBlockChainLock(ctx, blockExec.appClient, state, block)
}

// ValidateBlockTime validates the given block time against the given state.
// If the block is invalid, it returns an error.
// Validation does not mutate state, but does require historical information from the stateDB,
// ie. to verify evidence from a validator at an old height.
func (blockExec *BlockExecutor) ValidateBlockTime(
	allowedTimeWindow time.Duration,
	state State,
	block *types.Block,
) error {
	return validateBlockTime(allowedTimeWindow, state, block)
}

// FinalizeBlock validates the block against the state, fires the relevant events,
// calls FinalizeBlock ABCI endpoint, and saves the new state and responses.
// It returns the new state.
//
// It takes a blockID to avoid recomputing the parts hash.
//
// CONTRACT: The block was already delivered to the ABCI app using either PrepareProposal or ProcessProposal.
// See also ApplyBlock() to deliver proposal and finalize it in one step.

func (blockExec *BlockExecutor) FinalizeBlock(
	ctx context.Context,
	state State,
	uncommittedState CurrentRoundState,
	blockID types.BlockID,
	block *types.Block,
) (State, error) {
	// validate the block if we haven't already
	if err := blockExec.ValidateBlockWithRoundState(ctx, state, uncommittedState, block); err != nil {
		return state, ErrInvalidBlock{err}
	}
	startTime := time.Now().UnixNano()
	finalizeBlockResponse, err := execBlockWithoutState(ctx, blockExec.appClient, block, blockExec.logger, blockExec.store, -1, state, uncommittedState)
	if err != nil {
		return state, ErrInvalidBlock{err}
	}
	endTime := time.Now().UnixNano()
	blockExec.metrics.BlockProcessingTime.Observe(float64(endTime-startTime) / 1000000)
	if err != nil {
		return state, ErrProxyAppConn(err)
	}

	// Save the results before we commit.
	// TODO: upgrade for Same-Block-Execution. Right now, it just saves Finalize response, while we need
	// to find a way to save Prepare/ProcessProposal AND FinalizeBlock responses, as we don't have details like validators
	// in FinalizeResponse.
	abciResponses := tmstate.ABCIResponses{
		ProcessProposal: &uncommittedState.response,
		FinalizeBlock:   finalizeBlockResponse,
	}
	if err := blockExec.store.SaveABCIResponses(block.Height, abciResponses); err != nil {
		return state, err
	}

	stateUpdates, err := PrepareStateUpdates(ctx, block.Header, state, uncommittedState)
	if err != nil {
		return State{}, err
	}
	state, err = state.Update(ctx, blockID, &block.Header, stateUpdates...)
	if err != nil {
		return state, fmt.Errorf("commit failed for application: %w", err)
	}

	// Lock mempool, commit app state, update mempoool.
	retainHeight, err := blockExec.Commit(ctx, state, block, uncommittedState.TxResults)
	if err != nil {
		return state, fmt.Errorf("commit failed for application: %w", err)
	}

	// Update evpool with the latest state.
	blockExec.evpool.Update(ctx, state, block.Evidence)

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
	fireEvents(
		blockExec.logger,
		blockExec.eventBus,
		block,
		blockID,
		uncommittedState.TxResults,
		finalizeBlockResponse,
		&uncommittedState.response,
		state.Validators)

	return state, nil
}

// ApplyBlock validates the block against the state, executes it against the app using ProcessProposal ABCI request,
// fires the relevant events, finalizes with FinalizeBlock, and saves the new state and responses.
// It returns the new state.
// It's the only function that needs to be called
// from outside this package to process and commit an entire block.
// It takes a blockID to avoid recomputing the parts hash.
func (blockExec *BlockExecutor) ApplyBlock(
	ctx context.Context,
	state State,
	blockID types.BlockID, block *types.Block,
) (State, error) {
	uncommittedState, err := blockExec.ProcessProposal(ctx, block, state, true)
	if err != nil {
		return state, err
	}

	return blockExec.FinalizeBlock(ctx, state, uncommittedState, blockID, block)
}

func (blockExec *BlockExecutor) ExtendVote(ctx context.Context, vote *types.Vote) ([]*abci.ExtendVoteExtension, error) {
	resp, err := blockExec.appClient.ExtendVote(ctx, &abci.RequestExtendVote{
		Hash:   vote.BlockID.Hash,
		Height: vote.Height,
	})
	if err != nil {
		panic(fmt.Errorf("ExtendVote call failed: %w", err))
	}
	return resp.VoteExtensions, nil
}

func (blockExec *BlockExecutor) VerifyVoteExtension(ctx context.Context, vote *types.Vote) error {
	var extensions []*abci.ExtendVoteExtension
	if vote.VoteExtensions != nil {
		extensions = vote.VoteExtensions.ToExtendProto()
	}

	resp, err := blockExec.appClient.VerifyVoteExtension(ctx, &abci.RequestVerifyVoteExtension{
		Hash:               vote.BlockID.Hash,
		Height:             vote.Height,
		ValidatorProTxHash: vote.ValidatorProTxHash,
		VoteExtensions:     extensions,
	})
	if err != nil {
		panic(fmt.Errorf("VerifyVoteExtension call failed: %w", err))
	}

	if !resp.IsOK() {
		return errors.New("invalid vote extension")
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
	txResults []*abci.ExecTxResult,
) (int64, error) {
	blockExec.mempool.Lock()
	defer blockExec.mempool.Unlock()

	// while mempool is Locked, flush to ensure all async requests have completed
	// in the ABCI app before Commit.
	err := blockExec.mempool.FlushAppConn(ctx)
	if err != nil {
		blockExec.logger.Error("client error during mempool.FlushAppConn", "err", err)
		return 0, err
	}

	// Commit block, get hash back
	res, err := blockExec.appClient.Commit(ctx)
	if err != nil {
		blockExec.logger.Error("client error during proxyAppConn.Commit", "err", err)
		return 0, err
	}

	// ResponseCommit has no error code - just data
	blockExec.logger.Info(
		"committed state",
		"height", block.Height,
		"core_height", block.CoreChainLockedHeight,
		"num_txs", len(block.Txs),
		"block_app_hash", fmt.Sprintf("%X", block.AppHash),
	)

	// Update mempool.
	err = blockExec.mempool.Update(
		ctx,
		block.Height,
		block.Txs,
		txResults,
		TxPreCheckForState(state),
		TxPostCheckForState(state),
		state.ConsensusParams.ABCI.RecheckTx,
	)

	return res.RetainHeight, err
}

func buildLastCommitInfo(block *types.Block, initialHeight int64) abci.CommitInfo {
	if block.Height == initialHeight {
		// there is no last commit for the initial height.
		// return an empty value.
		return abci.CommitInfo{}
	}
	return abci.CommitInfo{
		Round:                   block.LastCommit.Round,
		QuorumHash:              block.LastCommit.QuorumHash,
		BlockSignature:          block.LastCommit.ThresholdBlockSignature,
		StateSignature:          block.LastCommit.ThresholdStateSignature,
		ThresholdVoteExtensions: types.ThresholdExtensionSignToProto(block.LastCommit.ThresholdVoteExtensions),
	}
}

// Update returns a copy of state with the fields set using the arguments passed in.
func (state State) Update(
	ctx context.Context,
	blockID types.BlockID,
	header *types.Header,
	stateUpdates ...UpdateFunc,
) (State, error) {

	nextVersion := state.Version

	// NOTE: the AppHash and the VoteExtension has not been populated.
	// It will be filled on state.Save.
	newState := State{
		Version:                          nextVersion,
		ChainID:                          state.ChainID,
		InitialHeight:                    state.InitialHeight,
		LastBlockHeight:                  header.Height,
		LastBlockID:                      blockID,
		LastStateID:                      types.StateID{Height: header.Height, AppHash: header.AppHash},
		LastBlockTime:                    header.Time,
		LastCoreChainLockedBlockHeight:   state.LastCoreChainLockedBlockHeight,
		Validators:                       state.Validators.Copy(),
		LastValidators:                   state.Validators.Copy(),
		LastHeightValidatorsChanged:      state.LastHeightValidatorsChanged,
		ConsensusParams:                  state.ConsensusParams,
		LastHeightConsensusParamsChanged: state.LastHeightConsensusParamsChanged,
		LastResultsHash:                  nil,
		AppHash:                          nil,
	}
	var err error
	newState, err = executeStateUpdates(ctx, newState, stateUpdates...)
	if err != nil {
		return State{}, err
	}
	return newState, nil
}

// SetAppHashSize ...
func (blockExec *BlockExecutor) SetAppHashSize(size int) {
	blockExec.appHashSize = size
}

// Fire NewBlock, NewBlockHeader.
// Fire TxEvent for every tx.
// NOTE: if Tendermint crashes before commit, some or all of these events may be published again.
func fireEvents(
	logger log.Logger,
	eventBus types.BlockEventPublisher,
	block *types.Block,
	blockID types.BlockID,
	txResults []*abci.ExecTxResult,
	finalizeBlockResponse *abci.ResponseFinalizeBlock,
	processProposalResponse *abci.ResponseProcessProposal,
	validatorSetUpdate *types.ValidatorSet,
) {
	if err := eventBus.PublishEventNewBlock(types.EventDataNewBlock{
		Block:               block,
		BlockID:             blockID,
		ResultFinalizeBlock: *finalizeBlockResponse,
	}); err != nil {
		logger.Error("failed publishing new block", "err", err)
	}

	if err := eventBus.PublishEventNewBlockHeader(types.EventDataNewBlockHeader{
		Header:                block.Header,
		NumTxs:                int64(len(block.Txs)),
		ResultProcessProposal: *processProposalResponse,
		ResultFinalizeBlock:   *finalizeBlockResponse,
	}); err != nil {
		logger.Error("failed publishing new block header", "err", err)
	}

	if len(block.Evidence) != 0 {
		for _, ev := range block.Evidence {
			if err := eventBus.PublishEventNewEvidence(types.EventDataNewEvidence{
				Evidence: ev,
				Height:   block.Height,
			}); err != nil {
				logger.Error("failed publishing new evidence", "err", err)
			}
		}
	}

	// sanity check
	if len(txResults) != len(block.Data.Txs) {
		panic(fmt.Sprintf("number of TXs (%d) and ABCI TX responses (%d) do not match",
			len(block.Data.Txs), len(txResults)))
	}

	for i, tx := range block.Data.Txs {
		if err := eventBus.PublishEventTx(types.EventDataTx{
			TxResult: abci.TxResult{
				Height: block.Height,
				Index:  uint32(i),
				Tx:     tx,
				Result: *(txResults[i]),
			},
		}); err != nil {
			logger.Error("failed publishing event TX", "err", err)
		}
	}

	if validatorSetUpdate != nil {
		if err := eventBus.PublishEventValidatorSetUpdates(
			types.EventDataValidatorSetUpdate{
				ValidatorSetUpdates: validatorSetUpdate.Validators,
				ThresholdPublicKey:  validatorSetUpdate.ThresholdPublicKey,
				QuorumHash:          validatorSetUpdate.QuorumHash,
			}); err != nil {
			logger.Error("failed publishing event validator-set update", "err", err)
		}
	}
}

func execBlock(
	ctx context.Context,
	appConn abciclient.Client,
	block *types.Block,
	logger log.Logger,
	store Store,
	initialHeight int64,
	s State,
) (*abci.ResponseFinalizeBlock, error) {
	version := block.Header.Version.ToProto()

	blockHash := block.Hash()
	txs := block.Txs.ToSliceOfBytes()
	lastCommit := buildLastCommitInfo(block, initialHeight)
	evidence := block.Evidence.ToABCI()

	var err error
	responseFinalizeBlock, err := appConn.FinalizeBlock(
		ctx,
		&abci.RequestFinalizeBlock{
			Hash:              blockHash,
			Height:            block.Height,
			Time:              block.Time,
			Txs:               txs,
			DecidedLastCommit: lastCommit,
			Misbehavior:       evidence,

			// Dash's fields
			CoreChainLockedHeight: block.CoreChainLockedHeight,
			ProposerProTxHash:     block.ProposerProTxHash,
			ProposedAppVersion:    block.ProposedAppVersion,
			Version:               &version,
			AppHash:               block.AppHash.Copy(),
		},
	)
	if err != nil {
		logger.Error("executing block", "err", err)
		return responseFinalizeBlock, err
	}
	logger.Info("executed block", "height", block.Height)

	return responseFinalizeBlock, nil
}

//----------------------------------------------------------------------------------------------------
// Execute block without state. TODO: eliminate
// ExecReplayedCommitBlock executes and commits a block on the proxyApp without validating or mutating the state.
// It returns the application root hash (apphash - result of abci.Commit).
//
// CONTRACT: Block should already be delivered to the app with PrepareProposal or ProcessProposal
func ExecReplayedCommitBlock(
	ctx context.Context,
	be *BlockExecutor,
	appConn abciclient.Client,
	block *types.Block,
	logger log.Logger,
	store Store,
	initialHeight int64,
	s State,
	ucState CurrentRoundState,
) (*abci.ResponseFinalizeBlock, error) {
	respFinalizeBlock, err := execBlockWithoutState(ctx, appConn, block, logger, store, initialHeight, s, ucState)
	if err != nil {
		return nil, err
	}

	// Commit block, get hash back
	res, err := appConn.Commit(ctx)
	if err != nil {
		logger.Error("client error during proxyAppConn.Commit", "err", res)
		return respFinalizeBlock, err
	}

	// the BlockExecutor condition is using for the final block replay process.
	if be != nil {
		vsetUpdate := ucState.NextValidators

		bps, err := block.MakePartSet(types.BlockPartSizeBytes)
		if err != nil {
			return respFinalizeBlock, err
		}

		blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: bps.Header()}
		fireEvents(
			be.logger,
			be.eventBus,
			block,
			blockID,
			ucState.TxResults,
			respFinalizeBlock,
			&ucState.response,
			vsetUpdate,
		)
	}

	return respFinalizeBlock, nil
}
func execBlockWithoutState(
	ctx context.Context,
	appConn abciclient.Client,
	block *types.Block,
	logger log.Logger,
	store Store,
	initialHeight int64,
	s State,
	ucState CurrentRoundState,
) (*abci.ResponseFinalizeBlock, error) {
	respFinalizeBlock, err := execBlock(ctx, appConn, block, logger, store, initialHeight, s)
	if err != nil {
		logger.Error("executing block", "err", err)
		return respFinalizeBlock, err
	}

	// ResponseCommit has no error or log, just data
	return respFinalizeBlock, nil
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

func validatePubKey(pk crypto.PubKey) error {
	v, ok := pk.(crypto.Validator)
	if !ok {
		return nil
	}
	if err := v.Validate(); err != nil {
		return err
	}
	return nil
}
