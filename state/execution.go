package state

import (
	"errors"
	"fmt"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
	tmsp "github.com/tendermint/tmsp/types"
)

// Validate block
func (s *State) ValidateBlock(block *types.Block) error {
	return s.validateBlock(block)
}

// Execute the block to mutate State.
// Validates block and then executes Data.Txs in the block.
func (s *State) ExecBlock(eventCache types.Fireable, proxyAppConn proxy.AppConnConsensus, block *types.Block, blockPartsHeader types.PartSetHeader) error {

	// Validate the block.
	err := s.validateBlock(block)
	if err != nil {
		return err
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
		return err
	}

	// All good!
	// Update validator accums and set state variables
	nextValSet.IncrementAccum(1)
	s.SetBlockAndValidators(block.Header, blockPartsHeader, valSet, nextValSet)

	return nil
}

// Executes block's transactions on proxyAppConn.
// TODO: Generate a bitmap or otherwise store tx validity in state.
func (s *State) execBlockOnProxyApp(eventCache types.Fireable, proxyAppConn proxy.AppConnConsensus, block *types.Block) error {

	var validTxs, invalidTxs = 0, 0

	// Execute transactions and get hash
	proxyCb := func(req *tmsp.Request, res *tmsp.Response) {
		switch r := res.Value.(type) {
		case *tmsp.Response_AppendTx:
			// TODO: make use of res.Log
			// TODO: make use of this info
			// Blocks may include invalid txs.
			// reqAppendTx := req.(tmsp.RequestAppendTx)
			txError := ""
			apTx := r.AppendTx
			if apTx.Code == tmsp.CodeType_OK {
				validTxs += 1
			} else {
				log.Debug("Invalid tx", "code", r.AppendTx.Code, "log", r.AppendTx.Log)
				invalidTxs += 1
				txError = apTx.Code.String()
			}
			// NOTE: if we count we can access the tx from the block instead of
			// pulling it from the req
			event := types.EventDataTx{
				Tx:     req.GetAppendTx().Tx,
				Result: apTx.Data,
				Code:   apTx.Code,
				Log:    apTx.Log,
				Error:  txError,
			}
			types.FireEventTx(eventCache, event)
		}
	}
	proxyAppConn.SetResponseCallback(proxyCb)

	// Begin block
	err := proxyAppConn.BeginBlockSync(types.TM2PB.Header(block.Header))
	if err != nil {
		log.Warn("Error in proxyAppConn.BeginBlock", "error", err)
		return err
	}

	// Run txs of block
	for _, tx := range block.Txs {
		proxyAppConn.AppendTxAsync(tx)
		if err := proxyAppConn.Error(); err != nil {
			return err
		}
	}

	// End block
	changedValidators, err := proxyAppConn.EndBlockSync(uint64(block.Height))
	if err != nil {
		log.Warn("Error in proxyAppConn.EndBlock", "error", err)
		return err
	}
	// TODO: Do something with changedValidators
	log.Debug("TODO: Do something with changedValidators", "changedValidators", changedValidators)

	log.Info(Fmt("ExecBlock got %v valid txs and %v invalid txs", validTxs, invalidTxs))
	return nil
}

func (s *State) validateBlock(block *types.Block) error {
	// Basic block validation.
	err := block.ValidateBasic(s.ChainID, s.LastBlockHeight, s.LastBlockID, s.LastBlockTime, s.AppHash)
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
			return fmt.Errorf("Invalid block commit size. Expected %v, got %v",
				s.LastValidators.Size(), len(block.LastCommit.Precommits))
		}
		err := s.LastValidators.VerifyCommit(
			s.ChainID, s.LastBlockID, block.Height-1, block.LastCommit)
		if err != nil {
			return err
		}
	}

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

//-----------------------------------------------------------------------------

type InvalidTxError struct {
	Tx   types.Tx
	Code tmsp.CodeType
}

func (txErr InvalidTxError) Error() string {
	return Fmt("Invalid tx: [%v] code: [%v]", txErr.Tx, txErr.Code)
}

//-----------------------------------------------------------------------------

// mempool must be locked during commit and update
// because state is typically reset on Commit and old txs must be replayed
// against committed state before new txs are run in the mempool, lest they be invalid
func (s *State) CommitStateUpdateMempool(proxyAppConn proxy.AppConnConsensus, block *types.Block, mempool Mempool) error {
	mempool.Lock()
	defer mempool.Unlock()

	// flush out any CheckTx that have already started
	// cs.proxyAppConn.FlushSync() // ?! XXX

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

// Execute and commit block against app, save block and state
func (s *State) ApplyBlock(eventCache events.Fireable, proxyAppConn proxy.AppConnConsensus,
	block *types.Block, partsHeader types.PartSetHeader, mempool Mempool) {

	// Run the block on the State:
	// + update validator sets
	// + run txs on the proxyAppConn
	err := s.ExecBlock(eventCache, proxyAppConn, block, partsHeader)
	if err != nil {
		// TODO: handle this gracefully.
		PanicQ(Fmt("Exec failed for application: %v", err))
	}

	// lock mempool, commit state, update mempoool
	err = s.CommitStateUpdateMempool(proxyAppConn, block, mempool)
	if err != nil {
		// TODO: handle this gracefully.
		PanicQ(Fmt("Commit failed for application: %v", err))
	}
}

// Replay all blocks after blockHeight and ensure the result matches the current state.
// XXX: blockStore must guarantee to have blocks for height <= blockStore.Height()
func (s *State) ReplayBlocks(header *types.Header, partsHeader types.PartSetHeader,
	appConnConsensus proxy.AppConnConsensus, blockStore proxy.BlockStore) error {

	// fresh state to work on
	stateCopy := s.Copy()

	// reset to this height (do nothing if its 0)
	var blockHeight int
	if header != nil {
		blockHeight = header.Height
		// TODO: put validators in iavl tree so we can set the state with an older validator set
		lastVals, nextVals := stateCopy.GetValidators()
		stateCopy.SetBlockAndValidators(header, partsHeader, lastVals, nextVals)
		stateCopy.AppHash = header.AppHash
	}

	// run the transactions
	var eventCache events.Fireable // nil

	// replay all blocks starting with blockHeight+1
	for i := blockHeight + 1; i <= blockStore.Height(); i++ {
		blockMeta := blockStore.LoadBlockMeta(i)
		block := blockStore.LoadBlock(i)
		panicOnNilBlock(i, blockStore.Height(), block, blockMeta) // XXX

		stateCopy.ApplyBlock(eventCache, appConnConsensus, block, blockMeta.PartsHeader, mockMempool{})
	}

	// The computed state and the previously set state should be identical
	if !s.Equals(stateCopy) {
		return fmt.Errorf("State after replay does not match saved state. Got ----\n%v\nExpected ----\n%v\n", stateCopy, s)
	}
	return nil
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

//------------------------------------------------
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
