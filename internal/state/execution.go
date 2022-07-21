package state

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	abciclient "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/libs/log"
	tmtypes "github.com/tendermint/tendermint/proto/tendermint/types"
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
		eventBus:   eventBus,
		store:      stateStore,
		appClient:  appClient,
		mempool:    pool,
		evpool:     evpool,
		logger:     logger,
		metrics:    metrics,
		cache:      make(map[string]struct{}),
		blockStore: blockStore,
	}
}

func (blockExec *BlockExecutor) Store() Store {
	return blockExec.store
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
	state State,
	lastExtCommit *types.ExtendedCommit,
	proposerAddr []byte,
) (*types.Block, error) {

	maxBytes := state.ConsensusParams.Block.MaxBytes
	maxGas := state.ConsensusParams.Block.MaxGas

	evidence, evSize := blockExec.evpool.PendingEvidence(state.ConsensusParams.Evidence.MaxBytes)

	// Fetch a limited amount of valid txs
	maxDataBytes := types.MaxDataBytes(maxBytes, evSize, state.Validators.Size())

	txs := blockExec.mempool.ReapMaxBytesMaxGas(maxDataBytes, maxGas)
	commit := lastExtCommit.ToCommit()
	block := state.MakeBlock(height, txs, commit, evidence, proposerAddr)
	rpp, err := blockExec.appClient.PrepareProposal(
		ctx,
		&abci.RequestPrepareProposal{
			MaxTxBytes:         maxDataBytes,
			Txs:                block.Txs.ToSliceOfBytes(),
			LocalLastCommit:    buildExtendedCommitInfo(lastExtCommit, blockExec.store, state.InitialHeight, state.ConsensusParams.ABCI),
			Misbehavior:        block.Evidence.ToABCI(),
			Height:             block.Height,
			Time:               block.Time,
			NextValidatorsHash: block.NextValidatorsHash,
			ProposerAddress:    block.ProposerAddress,
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
		return nil, err
	}
	txrSet := types.NewTxRecordSet(rpp.TxRecords)

	if err := txrSet.Validate(maxDataBytes, block.Txs); err != nil {
		return nil, err
	}

	for _, rtx := range txrSet.RemovedTxs() {
		if err := blockExec.mempool.RemoveTxByKey(rtx.Key()); err != nil {
			blockExec.logger.Debug("error removing transaction from the mempool", "error", err, "tx hash", rtx.Hash())
		}
	}
	itxs := txrSet.IncludedTxs()
	return state.MakeBlock(height, itxs, commit, evidence, proposerAddr), nil
}

func (blockExec *BlockExecutor) ProcessProposal(
	ctx context.Context,
	block *types.Block,
	state State,
) (bool, error) {
	resp, err := blockExec.appClient.ProcessProposal(ctx, &abci.RequestProcessProposal{
		Hash:               block.Header.Hash(),
		Height:             block.Header.Height,
		Time:               block.Header.Time,
		Txs:                block.Data.Txs.ToSliceOfBytes(),
		ProposedLastCommit: buildLastCommitInfo(block, blockExec.store, state.InitialHeight),
		Misbehavior:        block.Evidence.ToABCI(),
		ProposerAddress:    block.ProposerAddress,
		NextValidatorsHash: block.NextValidatorsHash,
	})
	if err != nil {
		return false, ErrInvalidBlock(err)
	}
	if resp.IsStatusUnknown() {
		panic(fmt.Sprintf("ProcessProposal responded with status %s", resp.Status.String()))
	}

	return resp.IsAccepted(), nil
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
	blockID types.BlockID, block *types.Block) (State, error) {
	// validate the block if we haven't already
	if err := blockExec.ValidateBlock(ctx, state, block); err != nil {
		return state, ErrInvalidBlock(err)
	}
	startTime := time.Now().UnixNano()
	fBlockRes, err := blockExec.appClient.FinalizeBlock(
		ctx,
		&abci.RequestFinalizeBlock{
			Hash:               block.Hash(),
			Height:             block.Header.Height,
			Time:               block.Header.Time,
			Txs:                block.Txs.ToSliceOfBytes(),
			DecidedLastCommit:  buildLastCommitInfo(block, blockExec.store, state.InitialHeight),
			Misbehavior:        block.Evidence.ToABCI(),
			ProposerAddress:    block.ProposerAddress,
			NextValidatorsHash: block.NextValidatorsHash,
		},
	)
	endTime := time.Now().UnixNano()
	blockExec.metrics.BlockProcessingTime.Observe(float64(endTime-startTime) / 1000000)
	if err != nil {
		return state, ErrProxyAppConn(err)
	}

	blockExec.logger.Info(
		"finalized block",
		"height", block.Height,
		"num_txs_res", len(fBlockRes.TxResults),
		"num_val_updates", len(fBlockRes.ValidatorUpdates),
		"block_app_hash", fmt.Sprintf("%X", fBlockRes.AppHash),
	)

	// Save the results before we commit.
	err = blockExec.store.SaveFinalizeBlockResponses(block.Height, fBlockRes)
	if err != nil && !errors.Is(err, ErrNoFinalizeBlockResponsesForHeight{block.Height}) {
		// It is correct to have an empty ResponseFinalizeBlock for ApplyBlock,
		// but not for saving it to the state store
		return state, err
	}

	// validate the validator updates and convert to tendermint types
	err = validateValidatorUpdates(fBlockRes.ValidatorUpdates, state.ConsensusParams.Validator)
	if err != nil {
		return state, fmt.Errorf("error in validator updates: %w", err)
	}

	validatorUpdates, err := types.PB2TM.ValidatorUpdates(fBlockRes.ValidatorUpdates)
	if err != nil {
		return state, err
	}
	if len(validatorUpdates) > 0 {
		blockExec.logger.Debug("updates to validators", "updates", types.ValidatorListString(validatorUpdates))
		blockExec.metrics.ValidatorSetUpdates.Add(1)
	}
	if fBlockRes.ConsensusParamUpdates != nil {
		blockExec.metrics.ConsensusParamUpdates.Add(1)
	}

	// Update the state with the block and responses.
	rs, err := abci.MarshalTxResults(fBlockRes.TxResults)
	if err != nil {
		return state, fmt.Errorf("marshaling TxResults: %w", err)
	}
	h := merkle.HashFromByteSlices(rs)
	state, err = state.Update(blockID, &block.Header, h, fBlockRes.ConsensusParamUpdates, validatorUpdates)
	if err != nil {
		return state, fmt.Errorf("commit failed for application: %w", err)
	}

	// Lock mempool, commit app state, update mempoool.
	retainHeight, err := blockExec.Commit(ctx, state, block, fBlockRes.TxResults)
	if err != nil {
		return state, fmt.Errorf("commit failed for application: %w", err)
	}

	// Update evpool with the latest state.
	blockExec.evpool.Update(ctx, state, block.Evidence)

	// Update the app hash and save the state.
	state.AppHash = fBlockRes.AppHash
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
	fireEvents(blockExec.logger, blockExec.eventBus, block, blockID, fBlockRes, validatorUpdates)

	return state, nil
}

func (blockExec *BlockExecutor) ExtendVote(ctx context.Context, vote *types.Vote) ([]byte, error) {
	resp, err := blockExec.appClient.ExtendVote(ctx, &abci.RequestExtendVote{
		Hash:   vote.BlockID.Hash,
		Height: vote.Height,
	})
	if err != nil {
		panic(fmt.Errorf("ExtendVote call failed: %w", err))
	}
	return resp.VoteExtension, nil
}

func (blockExec *BlockExecutor) VerifyVoteExtension(ctx context.Context, vote *types.Vote) error {
	resp, err := blockExec.appClient.VerifyVoteExtension(ctx, &abci.RequestVerifyVoteExtension{
		Hash:             vote.BlockID.Hash,
		ValidatorAddress: vote.ValidatorAddress,
		Height:           vote.Height,
		VoteExtension:    vote.Extension,
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

func buildLastCommitInfo(block *types.Block, store Store, initialHeight int64) abci.CommitInfo {
	if block.Height == initialHeight {
		// there is no last commit for the initial height.
		// return an empty value.
		return abci.CommitInfo{}
	}

	lastValSet, err := store.LoadValidators(block.Height - 1)
	if err != nil {
		panic(fmt.Errorf("failed to load validator set at height %d: %w", block.Height-1, err))
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
			SignedLastBlock: commitSig.BlockIDFlag != types.BlockIDFlagAbsent,
		}
	}

	return abci.CommitInfo{
		Round: block.LastCommit.Round,
		Votes: votes,
	}
}

// buildExtendedCommitInfo populates an ABCI extended commit from the
// corresponding Tendermint extended commit ec, using the stored validator set
// from ec.  It requires ec to include the original precommit votes along with
// the vote extensions from the last commit.
//
// For heights below the initial height, for which we do not have the required
// data, it returns an empty record.
//
// Assumes that the commit signatures are sorted according to validator index.
func buildExtendedCommitInfo(ec *types.ExtendedCommit, store Store, initialHeight int64, ap types.ABCIParams) abci.ExtendedCommitInfo {
	if ec.Height < initialHeight {
		// There are no extended commits for heights below the initial height.
		return abci.ExtendedCommitInfo{}
	}

	valSet, err := store.LoadValidators(ec.Height)
	if err != nil {
		panic(fmt.Errorf("failed to load validator set at height %d, initial height %d: %w", ec.Height, initialHeight, err))
	}

	var (
		ecSize    = ec.Size()
		valSetLen = len(valSet.Validators)
	)

	// Ensure that the size of the validator set in the extended commit matches
	// the size of the validator set in the state store.
	if ecSize != valSetLen {
		panic(fmt.Errorf(
			"extended commit size (%d) does not match validator set length (%d) at height %d\n\n%v\n\n%v",
			ecSize, valSetLen, ec.Height, ec.ExtendedSignatures, valSet.Validators,
		))
	}

	votes := make([]abci.ExtendedVoteInfo, ecSize)
	for i, val := range valSet.Validators {
		ecs := ec.ExtendedSignatures[i]

		// Absent signatures have empty validator addresses, but otherwise we
		// expect the validator addresses to be the same.
		if ecs.BlockIDFlag != types.BlockIDFlagAbsent && !bytes.Equal(ecs.ValidatorAddress, val.Address) {
			panic(fmt.Errorf("validator address of extended commit signature in position %d (%s) does not match the corresponding validator's at height %d (%s)",
				i, ecs.ValidatorAddress, ec.Height, val.Address,
			))
		}

		var ext []byte
		// Check if vote extensions were enabled during the commit's height: ec.Height.
		// ec is the commit from the previous height, so if extensions were enabled
		// during that height, we ensure they are present and deliver the data to
		// the proposer. If they were not enabled during this previous height, we
		// will not deliver extension data.
		if ap.VoteExtensionsEnabled(ec.Height) && ecs.BlockIDFlag == types.BlockIDFlagCommit {
			if err := ecs.EnsureExtension(); err != nil {
				panic(fmt.Errorf("commit at height %d received with missing vote extensions data", ec.Height))
			}
			ext = ecs.Extension
		}

		votes[i] = abci.ExtendedVoteInfo{
			Validator:       types.TM2PB.Validator(val),
			SignedLastBlock: ecs.BlockIDFlag != types.BlockIDFlagAbsent,
			VoteExtension:   ext,
		}
	}

	return abci.ExtendedCommitInfo{
		Round: ec.Round,
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

// Update returns a copy of state with the fields set using the arguments passed in.
func (state State) Update(
	blockID types.BlockID,
	header *types.Header,
	resultsHash []byte,
	consensusParamUpdates *tmtypes.ConsensusParams,
	validatorUpdates []*types.Validator,
) (State, error) {

	// Copy the valset so we can apply changes from FinalizeBlock
	// and update s.LastValidators and s.Validators.
	nValSet := state.NextValidators.Copy()

	// Update the validator set with the latest responses to FinalizeBlock.
	lastHeightValsChanged := state.LastHeightValidatorsChanged
	if len(validatorUpdates) > 0 {
		err := nValSet.UpdateWithChangeSet(validatorUpdates)
		if err != nil {
			return state, fmt.Errorf("changing validator set: %w", err)
		}
		// Change results from this height but only applies to the next next height.
		lastHeightValsChanged = header.Height + 1 + 1
	}

	// Update validator proposer priority and set state variables.
	nValSet.IncrementProposerPriority(1)

	// Update the params with the latest responses to FinalizeBlock.
	nextParams := state.ConsensusParams
	lastHeightParamsChanged := state.LastHeightConsensusParamsChanged
	if consensusParamUpdates != nil {
		// NOTE: must not mutate state.ConsensusParams
		nextParams = state.ConsensusParams.UpdateConsensusParams(consensusParamUpdates)
		err := nextParams.ValidateConsensusParams()
		if err != nil {
			return state, fmt.Errorf("updating consensus params: %w", err)
		}

		err = state.ConsensusParams.ValidateUpdate(consensusParamUpdates, header.Height)
		if err != nil {
			return state, fmt.Errorf("updating consensus params: %w", err)
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
		LastResultsHash:                  resultsHash,
		AppHash:                          nil,
	}, nil
}

// Fire NewBlock, NewBlockHeader.
// Fire TxEvent for every tx.
// NOTE: if Tendermint crashes before commit, some or all of these events may be published again.
func fireEvents(
	logger log.Logger,
	eventBus types.BlockEventPublisher,
	block *types.Block,
	blockID types.BlockID,
	finalizeBlockResponse *abci.ResponseFinalizeBlock,
	validatorUpdates []*types.Validator,
) {
	if err := eventBus.PublishEventNewBlock(types.EventDataNewBlock{
		Block:               block,
		BlockID:             blockID,
		ResultFinalizeBlock: *finalizeBlockResponse,
	}); err != nil {
		logger.Error("failed publishing new block", "err", err)
	}

	if err := eventBus.PublishEventNewBlockHeader(types.EventDataNewBlockHeader{
		Header:              block.Header,
		NumTxs:              int64(len(block.Txs)),
		ResultFinalizeBlock: *finalizeBlockResponse,
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
	if len(finalizeBlockResponse.TxResults) != len(block.Data.Txs) {
		panic(fmt.Sprintf("number of TXs (%d) and ABCI TX responses (%d) do not match",
			len(block.Data.Txs), len(finalizeBlockResponse.TxResults)))
	}

	for i, tx := range block.Data.Txs {
		if err := eventBus.PublishEventTx(types.EventDataTx{
			TxResult: abci.TxResult{
				Height: block.Height,
				Index:  uint32(i),
				Tx:     tx,
				Result: *(finalizeBlockResponse.TxResults[i]),
			},
		}); err != nil {
			logger.Error("failed publishing event TX", "err", err)
		}
	}

	if len(finalizeBlockResponse.ValidatorUpdates) > 0 {
		if err := eventBus.PublishEventValidatorSetUpdates(
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
	appConn abciclient.Client,
	block *types.Block,
	logger log.Logger,
	store Store,
	initialHeight int64,
	s State,
) ([]byte, error) {
	finalizeBlockResponse, err := appConn.FinalizeBlock(
		ctx,
		&abci.RequestFinalizeBlock{
			Hash:              block.Hash(),
			Height:            block.Height,
			Time:              block.Time,
			Txs:               block.Txs.ToSliceOfBytes(),
			DecidedLastCommit: buildLastCommitInfo(block, store, initialHeight),
			Misbehavior:       block.Evidence.ToABCI(),
		},
	)

	if err != nil {
		logger.Error("executing block", "err", err)
		return nil, err
	}
	logger.Info("executed block", "height", block.Height)

	// the BlockExecutor condition is using for the final block replay process.
	if be != nil {
		err = validateValidatorUpdates(finalizeBlockResponse.ValidatorUpdates, s.ConsensusParams.Validator)
		if err != nil {
			logger.Error("validating validator updates", "err", err)
			return nil, err
		}
		validatorUpdates, err := types.PB2TM.ValidatorUpdates(finalizeBlockResponse.ValidatorUpdates)
		if err != nil {
			logger.Error("converting validator updates to native types", "err", err)
			return nil, err
		}

		bps, err := block.MakePartSet(types.BlockPartSizeBytes)
		if err != nil {
			return nil, err
		}

		blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: bps.Header()}
		fireEvents(be.logger, be.eventBus, block, blockID, finalizeBlockResponse, validatorUpdates)
	}

	// Commit block
	_, err = appConn.Commit(ctx)
	if err != nil {
		logger.Error("client error during proxyAppConn.Commit", "err", err)
		return nil, err
	}

	// ResponseCommit has no error or log
	return finalizeBlockResponse.AppHash, nil
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
