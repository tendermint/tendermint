package state

import (
	"bytes"
	"errors"

	"github.com/ebuchman/fail-test"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-events"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
	tmsp "github.com/tendermint/tmsp/types"
)

//--------------------------------------------------
// Execute the block

type (
	ErrInvalidBlock error
	ErrProxyAppConn error
)

// Execute the block to mutate State.
// Validates block and then executes Data.Txs in the block.
func (s *State) ExecBlock(eventCache events.Fireable, proxyAppConn proxy.AppConnConsensus, block *types.Block, blockPartsHeader types.PartSetHeader) error {

	// Validate the block.
	err := s.validateBlock(block)
	if err != nil {
		return ErrInvalidBlock(err)
	}

	// Update the validator set
	valSet := s.Validators.Copy()
	// Update valSet with signatures from block.
	updateValidatorsWithBlock(s.LastValidators, valSet, block)
	// TODO: Update the validator set (e.g. block.Data.ValidatorUpdates?)
	nextValSet := valSet.Copy()

	// Execute the block txs
	err = s.execBlockOnProxyApp(eventCache, proxyAppConn, block)
	if err != nil {
		// There was some error in proxyApp
		// TODO Report error and wait for proxyApp to be available.
		return ErrProxyAppConn(err)
	}

	// All good!
	// Update validator accums and set state variables
	nextValSet.IncrementAccum(1)
	s.SetBlockAndValidators(block.Header, blockPartsHeader, valSet, nextValSet)

	// save state with updated height/blockhash/validators
	// but stale apphash, in case we fail between Commit and Save
	s.Save()

	return nil
}

// Executes block's transactions on proxyAppConn.
// TODO: Generate a bitmap or otherwise store tx validity in state.
func (s *State) execBlockOnProxyApp(eventCache events.Fireable, proxyAppConn proxy.AppConnConsensus, block *types.Block) error {

	var validTxs, invalidTxs = 0, 0

	// Execute transactions and get hash
	proxyCb := func(req *tmsp.Request, res *tmsp.Response) {
		switch r := res.Value.(type) {
		case *tmsp.Response_AppendTx:
			// TODO: make use of res.Log
			// TODO: make use of this info
			// Blocks may include invalid txs.
			// reqAppendTx := req.(tmsp.RequestAppendTx)
			if r.AppendTx.Code == tmsp.CodeType_OK {
				validTxs += 1
			} else {
				log.Debug("Invalid tx", "code", r.AppendTx.Code, "log", r.AppendTx.Log)
				invalidTxs += 1
			}
			// NOTE: if we count we can access the tx from the block instead of
			// pulling it from the req
			if eventCache != nil {
				eventCache.FireEvent(types.EventStringTx(req.GetAppendTx().Tx), res)
			}
		}
	}
	proxyAppConn.SetResponseCallback(proxyCb)

	// Begin block
	err := proxyAppConn.BeginBlockSync(block.Hash(), types.TM2PB.Header(block.Header))
	if err != nil {
		log.Warn("Error in proxyAppConn.BeginBlock", "error", err)
		return err
	}

	fail.Fail() // XXX

	// Run txs of block
	for _, tx := range block.Txs {
		fail.FailRand(len(block.Txs)) // XXX
		proxyAppConn.AppendTxAsync(tx)
		if err := proxyAppConn.Error(); err != nil {
			return err
		}
	}

	fail.Fail() // XXX

	// End block
	changedValidators, err := proxyAppConn.EndBlockSync(uint64(block.Height))
	if err != nil {
		log.Warn("Error in proxyAppConn.EndBlock", "error", err)
		return err
	}

	fail.Fail() // XXX

	// TODO: Do something with changedValidators
	log.Info("TODO: Do something with changedValidators", changedValidators)

	log.Info(Fmt("ExecBlock got %v valid txs and %v invalid txs", validTxs, invalidTxs))
	return nil
}

// Updates the LastCommitHeight of the validators in valSet, in place.
// Assumes that lastValSet matches the valset of block.LastCommit
// CONTRACT: lastValSet is not mutated.
func updateValidatorsWithBlock(lastValSet *types.ValidatorSet, valSet *types.ValidatorSet, block *types.Block) {

	for i, precommit := range block.LastCommit.Precommits {
		if precommit == nil {
			continue
		}
		_, val := lastValSet.GetByIndex(i)
		if val == nil {
			PanicCrisis(Fmt("Failed to fetch validator at index %v", i))
		}
		if _, val_ := valSet.GetByAddress(val.Address); val_ != nil {
			val_.LastCommitHeight = block.Height - 1
			updated := valSet.Update(val_)
			if !updated {
				PanicCrisis("Failed to update validator LastCommitHeight")
			}
		} else {
			// XXX This is not an error if validator was removed.
			// But, we don't mutate validators yet so go ahead and panic.
			PanicCrisis("Could not find validator")
		}
	}

}

//-----------------------------------------------------
// Validate block

func (s *State) ValidateBlock(block *types.Block) error {
	return s.validateBlock(block)
}

func (s *State) validateBlock(block *types.Block) error {
	// Basic block validation.
	err := block.ValidateBasic(s.ChainID, s.LastBlockHeight, s.LastBlockHash, s.LastBlockParts, s.LastBlockTime, s.AppHash)
	if err != nil {
		return err
	}

	// Validate block LastCommit.
	if block.Height == 1 {
		if len(block.LastCommit.Precommits) != 0 {
			return errors.New("Block at height 1 (first block) should have no LastCommit precommits")
		}
	} else {
		if len(block.LastCommit.Precommits) != s.LastValidators.Size() {
			return errors.New(Fmt("Invalid block commit size. Expected %v, got %v",
				s.LastValidators.Size(), len(block.LastCommit.Precommits)))
		}
		err := s.LastValidators.VerifyCommit(
			s.ChainID, s.LastBlockHash, s.LastBlockParts, block.Height-1, block.LastCommit)
		if err != nil {
			return err
		}
	}

	return nil
}

//-----------------------------------------------------------------------------
// ApplyBlock executes the block, then commits and updates the mempool atomically

// Execute and commit block against app, save block and state
func (s *State) ApplyBlock(eventCache events.Fireable, proxyAppConn proxy.AppConnConsensus,
	block *types.Block, partsHeader types.PartSetHeader, mempool Mempool) error {

	// Run the block on the State:
	// + update validator sets
	// + run txs on the proxyAppConn
	err := s.ExecBlock(eventCache, proxyAppConn, block, partsHeader)
	if err != nil {
		return errors.New(Fmt("Exec failed for application: %v", err))
	}

	// lock mempool, commit state, update mempoool
	err = s.CommitStateUpdateMempool(proxyAppConn, block, mempool)
	if err != nil {
		return errors.New(Fmt("Commit failed for application: %v", err))
	}
	return nil
}

// mempool must be locked during commit and update
// because state is typically reset on Commit and old txs must be replayed
// against committed state before new txs are run in the mempool, lest they be invalid
func (s *State) CommitStateUpdateMempool(proxyAppConn proxy.AppConnConsensus, block *types.Block, mempool Mempool) error {
	mempool.Lock()
	defer mempool.Unlock()

	// Commit block, get hash back
	res := proxyAppConn.CommitSync()
	if res.IsErr() {
		log.Warn("Error in proxyAppConn.CommitSync", "error", res)
		return res
	}
	if res.Log != "" {
		log.Debug("Commit.Log: " + res.Log)
	}

	// Set the state's new AppHash
	s.AppHash = res.Data

	// Update mempool.
	mempool.Update(block.Height, block.Txs)

	return nil
}

// Updates to the mempool need to be synchronized with committing a block
// so apps can reset their transient state on Commit
type Mempool interface {
	Lock()
	Unlock()
	Update(height int, txs []types.Tx)
}

type mockMempool struct {
}

func (m mockMempool) Lock()                             {}
func (m mockMempool) Unlock()                           {}
func (m mockMempool) Update(height int, txs []types.Tx) {}

//----------------------------------------------------------------
// Replay blocks to sync app to latest state of core

type ErrReplay error

type ErrAppBlockHeightTooHigh struct {
	coreHeight int
	appHeight  int
}

func (e ErrAppBlockHeightTooHigh) Error() string {
	return Fmt("App block height (%d) is higher than core (%d)", e.appHeight, e.coreHeight)
}

type ErrLastStateMismatch struct {
	height int
	core   []byte
	app    []byte
}

func (e ErrLastStateMismatch) Error() string {
	return Fmt("Latest tendermint block (%d) LastAppHash (%X) does not match app's AppHash (%X)", e.height, e.core, e.app)
}

type ErrStateMismatch struct {
	got      *State
	expected *State
}

func (e ErrStateMismatch) Error() string {
	return Fmt("State after replay does not match saved state. Got ----\n%v\nExpected ----\n%v\n", e.got, e.expected)
}

// Replay all blocks after blockHeight and ensure the result matches the current state.
// XXX: blockStore must guarantee to have blocks for height <= blockStore.Height()
func (s *State) ReplayBlocks(appHash []byte, header *types.Header, partsHeader types.PartSetHeader,
	appConnConsensus proxy.AppConnConsensus, blockStore proxy.BlockStore) error {

	// NOTE/TODO: tendermint may crash after the app commits
	// but before it can save the new state root.
	// it should save all eg. valset changes before calling Commit.
	// then, if tm state is behind app state, the only thing missing can be app hash

	// get a fresh state and reset to the apps latest
	stateCopy := s.Copy()
	if header != nil {
		// TODO: put validators in iavl tree so we can set the state with an older validator set
		lastVals, nextVals := stateCopy.GetValidators()
		stateCopy.SetBlockAndValidators(header, partsHeader, lastVals, nextVals)
		stateCopy.Stale = false
		stateCopy.AppHash = appHash
	}

	appBlockHeight := stateCopy.LastBlockHeight
	coreBlockHeight := blockStore.Height()
	if coreBlockHeight < appBlockHeight {
		// if the app is ahead, there's nothing we can do
		return ErrAppBlockHeightTooHigh{coreBlockHeight, appBlockHeight}

	} else if coreBlockHeight == appBlockHeight {
		// if we crashed between Commit and SaveState,
		// the state's app hash is stale.
		// otherwise we're synced
		if s.Stale {
			s.Stale = false
			s.AppHash = appHash
		}
		return checkState(s, stateCopy)

	} else if s.LastBlockHeight == appBlockHeight {
		// core is ahead of app but core's state height is at apps height
		// this happens if we crashed after saving the block,
		// but before committing it. We should be 1 ahead
		if coreBlockHeight != appBlockHeight+1 {
			PanicSanity(Fmt("core.state.height == app.height but core.height (%d) > app.height+1 (%d)", coreBlockHeight, appBlockHeight+1))
		}

		// check that the blocks last apphash is the states apphash
		blockMeta := blockStore.LoadBlockMeta(coreBlockHeight)
		if !bytes.Equal(blockMeta.Header.AppHash, appHash) {
			return ErrLastStateMismatch{coreBlockHeight, blockMeta.Header.AppHash, appHash}
		}

		// replay the block against the actual tendermint state (not the copy)
		return loadApplyBlock(coreBlockHeight, s, blockStore, appConnConsensus)

	} else {
		// either we're caught up or there's blocks to replay
		// replay all blocks starting with appBlockHeight+1
		for i := appBlockHeight + 1; i <= coreBlockHeight; i++ {
			loadApplyBlock(i, stateCopy, blockStore, appConnConsensus)
		}
		return checkState(s, stateCopy)
	}
}

func checkState(s, stateCopy *State) error {
	// The computed state and the previously set state should be identical
	if !s.Equals(stateCopy) {
		return ErrStateMismatch{stateCopy, s}
	}
	return nil
}

func loadApplyBlock(blockIndex int, s *State, blockStore proxy.BlockStore, appConnConsensus proxy.AppConnConsensus) error {
	blockMeta := blockStore.LoadBlockMeta(blockIndex)
	block := blockStore.LoadBlock(blockIndex)
	panicOnNilBlock(blockIndex, blockStore.Height(), block, blockMeta) // XXX

	var eventCache events.Fireable // nil
	return s.ApplyBlock(eventCache, appConnConsensus, block, blockMeta.PartsHeader, mockMempool{})
}

func panicOnNilBlock(height, bsHeight int, block *types.Block, blockMeta *types.BlockMeta) {
	if block == nil || blockMeta == nil {
		// Sanity?
		PanicCrisis(Fmt(`
block/blockMeta is nil for height <= blockStore.Height() (%d <= %d).
Block: %v,
BlockMeta: %v
`, height, bsHeight, block, blockMeta))

	}
}
