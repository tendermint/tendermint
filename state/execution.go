package state

import (
	"bytes"
	"errors"

	"github.com/ebuchman/fail-test"

	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
	tmsp "github.com/tendermint/tmsp/types"
)

//--------------------------------------------------
// Execute the block

// Execute the block to mutate State.
// Validates block and then executes Data.Txs in the block.
func (s *State) ExecBlock(eventCache types.Fireable, proxyAppConn proxy.AppConnConsensus, block *types.Block, blockPartsHeader types.PartSetHeader) error {

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
	log.Debug("TODO: Do something with changedValidators", "changedValidators", changedValidators)

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
			return errors.New(Fmt("Invalid block commit size. Expected %v, got %v",
				s.LastValidators.Size(), len(block.LastCommit.Precommits)))
		}
		err := s.LastValidators.VerifyCommit(
			s.ChainID, s.LastBlockID, block.Height-1, block.LastCommit)
		if err != nil {
			return err
		}
	}

	return nil
}

//-----------------------------------------------------------------------------
// ApplyBlock executes the block, then commits and updates the mempool atomically

// Execute and commit block against app, save block and state
func (s *State) ApplyBlock(eventCache types.Fireable, proxyAppConn proxy.AppConnConsensus,
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
// Handshake with app to sync to latest state of core by replaying blocks

// TODO: Should we move blockchain/store.go to its own package?
type BlockStore interface {
	Height() int
	LoadBlock(height int) *types.Block
}

type Handshaker struct {
	config cfg.Config
	state  *State
	store  BlockStore

	nBlocks int // number of blocks applied to the state
}

func NewHandshaker(config cfg.Config, state *State, store BlockStore) *Handshaker {
	return &Handshaker{config, state, store, 0}
}

// TODO: retry the handshake once if it fails the first time
// ... let Info take an argument determining its behaviour
func (h *Handshaker) Handshake(proxyApp proxy.AppConns) error {
	// handshake is done via info request on the query conn
	res, tmspInfo, blockInfo, configInfo := proxyApp.Query().InfoSync()
	if res.IsErr() {
		return errors.New(Fmt("Error calling Info. Code: %v; Data: %X; Log: %s", res.Code, res.Data, res.Log))
	}

	if blockInfo == nil {
		log.Warn("blockInfo is nil, aborting handshake")
		return nil
	}

	log.Notice("TMSP Handshake", "height", blockInfo.BlockHeight, "block_hash", blockInfo.BlockHash, "app_hash", blockInfo.AppHash)

	blockHeight := int(blockInfo.BlockHeight) // safe, should be an int32
	blockHash := blockInfo.BlockHash
	appHash := blockInfo.AppHash

	if tmspInfo != nil {
		// TODO: check tmsp version (or do this in the tmspcli?)
		_ = tmspInfo
	}

	// last block (nil if we starting from 0)
	var header *types.Header
	var partsHeader types.PartSetHeader

	// replay all blocks after blockHeight
	// if blockHeight == 0, we will replay everything
	if blockHeight != 0 {
		block := h.store.LoadBlock(blockHeight)
		if block == nil {
			return ErrUnknownBlock{blockHeight}
		}

		// check block hash
		if !bytes.Equal(block.Hash(), blockHash) {
			return ErrBlockHashMismatch{block.Hash(), blockHash, blockHeight}
		}

		// NOTE: app hash should be in the next block ...
		// check app hash
		/*if !bytes.Equal(block.Header.AppHash, appHash) {
			return fmt.Errorf("Handshake error. App hash at height %d does not match. Got %X, expected %X", blockHeight, appHash, block.Header.AppHash)
		}*/

		header = block.Header
		partsHeader = block.MakePartSet(h.config.GetInt("block_part_size")).Header()
	}

	if configInfo != nil {
		// TODO: set config info
		_ = configInfo
	}

	// replay blocks up to the latest in the blockstore
	err := h.ReplayBlocks(appHash, header, partsHeader, proxyApp.Consensus())
	if err != nil {
		return errors.New(Fmt("Error on replay: %v", err))
	}

	// TODO: (on restart) replay mempool

	return nil
}

// Replay all blocks after blockHeight and ensure the result matches the current state.
func (h *Handshaker) ReplayBlocks(appHash []byte, header *types.Header, partsHeader types.PartSetHeader,
	appConnConsensus proxy.AppConnConsensus) error {

	// NOTE/TODO: tendermint may crash after the app commits
	// but before it can save the new state root.
	// it should save all eg. valset changes before calling Commit.
	// then, if tm state is behind app state, the only thing missing can be app hash

	// get a fresh state and reset to the apps latest
	stateCopy := h.state.Copy()

	// TODO: put validators in iavl tree so we can set the state with an older validator set
	lastVals, nextVals := stateCopy.GetValidators()
	if header == nil {
		stateCopy.LastBlockHeight = 0
		stateCopy.LastBlockID = types.BlockID{}
		// stateCopy.LastBlockTime = ... doesnt matter
		stateCopy.Validators = nextVals
		stateCopy.LastValidators = lastVals
	} else {
		stateCopy.SetBlockAndValidators(header, partsHeader, lastVals, nextVals)
	}
	stateCopy.Stale = false
	stateCopy.AppHash = appHash

	appBlockHeight := stateCopy.LastBlockHeight
	coreBlockHeight := h.store.Height()
	if coreBlockHeight < appBlockHeight {
		// if the app is ahead, there's nothing we can do
		return ErrAppBlockHeightTooHigh{coreBlockHeight, appBlockHeight}

	} else if coreBlockHeight == appBlockHeight {
		// if we crashed between Commit and SaveState,
		// the state's app hash is stale.
		// otherwise we're synced
		if h.state.Stale {
			h.state.Stale = false
			h.state.AppHash = appHash
		}
		return checkState(h.state, stateCopy)

	} else if h.state.LastBlockHeight == appBlockHeight {
		// core is ahead of app but core's state height is at apps height
		// this happens if we crashed after saving the block,
		// but before committing it. We should be 1 ahead
		if coreBlockHeight != appBlockHeight+1 {
			PanicSanity(Fmt("core.state.height == app.height but core.height (%d) > app.height+1 (%d)", coreBlockHeight, appBlockHeight+1))
		}

		// check that the blocks last apphash is the states apphash
		block := h.store.LoadBlock(coreBlockHeight)
		if !bytes.Equal(block.Header.AppHash, appHash) {
			return ErrLastStateMismatch{coreBlockHeight, block.Header.AppHash, appHash}
		}

		// replay the block against the actual tendermint state (not the copy)
		return h.loadApplyBlock(coreBlockHeight, h.state, appConnConsensus)

	} else {
		// either we're caught up or there's blocks to replay
		// replay all blocks starting with appBlockHeight+1
		for i := appBlockHeight + 1; i <= coreBlockHeight; i++ {
			h.loadApplyBlock(i, stateCopy, appConnConsensus)
		}
		return checkState(h.state, stateCopy)
	}
}

func checkState(s, stateCopy *State) error {
	// The computed state and the previously set state should be identical
	if !s.Equals(stateCopy) {
		return ErrStateMismatch{stateCopy, s}
	}
	return nil
}

func (h *Handshaker) loadApplyBlock(blockIndex int, state *State, appConnConsensus proxy.AppConnConsensus) error {
	h.nBlocks += 1
	block := h.store.LoadBlock(blockIndex)
	panicOnNilBlock(blockIndex, h.store.Height(), block) // XXX
	var eventCache types.Fireable                        // nil
	return state.ApplyBlock(eventCache, appConnConsensus, block, block.MakePartSet(h.config.GetInt("block_part_size")).Header(), mockMempool{})
}

func panicOnNilBlock(height, bsHeight int, block *types.Block) {
	if block == nil {
		// Sanity?
		PanicCrisis(Fmt(`
block is nil for height <= blockStore.Height() (%d <= %d).
Block: %v,
`, height, bsHeight, block))

	}
}
