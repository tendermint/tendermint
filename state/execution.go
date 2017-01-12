package state

import (
	"bytes"
	"errors"

	"github.com/ebuchman/fail-test"

	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
	abci "github.com/tendermint/abci/types"
)

//--------------------------------------------------
// Execute the block

// Execute the block to mutate State.
// Validates block and then executes Data.Txs in the block.
func (s *State) ExecBlock(eventCache types.Fireable, proxyAppConn proxy.AppConnConsensus, block *types.Block, blockPartsHeader types.PartSetHeader) error {

	// Validate the block.
	if err := s.validateBlock(block); err != nil {
		return ErrInvalidBlock(err)
	}

	// compute bitarray of validators that signed
	signed := commitBitArrayFromBlock(block)
	_ = signed // TODO send on begin block

	// copy the valset
	valSet := s.Validators.Copy()
	nextValSet := valSet.Copy()

	// Execute the block txs
	changedValidators, err := execBlockOnProxyApp(eventCache, proxyAppConn, block)
	if err != nil {
		// There was some error in proxyApp
		// TODO Report error and wait for proxyApp to be available.
		return ErrProxyAppConn(err)
	}

	// update the validator set
	err = updateValidators(nextValSet, changedValidators)
	if err != nil {
		log.Warn("Error changing validator set", "error", err)
		// TODO: err or carry on?
	}

	// All good!
	// Update validator accums and set state variables
	nextValSet.IncrementAccum(1)
	s.SetBlockAndValidators(block.Header, blockPartsHeader, valSet, nextValSet)

	// save state with updated height/blockhash/validators
	// but stale apphash, in case we fail between Commit and Save
	s.SaveIntermediate()

	fail.Fail() // XXX

	return nil
}

// Executes block's transactions on proxyAppConn.
// Returns a list of updates to the validator set
// TODO: Generate a bitmap or otherwise store tx validity in state.
func execBlockOnProxyApp(eventCache types.Fireable, proxyAppConn proxy.AppConnConsensus, block *types.Block) ([]*abci.Validator, error) {

	var validTxs, invalidTxs = 0, 0

	// Execute transactions and get hash
	proxyCb := func(req *abci.Request, res *abci.Response) {
		switch r := res.Value.(type) {
		case *abci.Response_DeliverTx:
			// TODO: make use of res.Log
			// TODO: make use of this info
			// Blocks may include invalid txs.
			// reqDeliverTx := req.(abci.RequestDeliverTx)
			txError := ""
			apTx := r.DeliverTx
			if apTx.Code == abci.CodeType_OK {
				validTxs += 1
			} else {
				log.Debug("Invalid tx", "code", r.DeliverTx.Code, "log", r.DeliverTx.Log)
				invalidTxs += 1
				txError = apTx.Code.String()
			}
			// NOTE: if we count we can access the tx from the block instead of
			// pulling it from the req
			event := types.EventDataTx{
				Tx:    req.GetDeliverTx().Tx,
				Data:  apTx.Data,
				Code:  apTx.Code,
				Log:   apTx.Log,
				Error: txError,
			}
			types.FireEventTx(eventCache, event)
		}
	}
	proxyAppConn.SetResponseCallback(proxyCb)

	// Begin block
	err := proxyAppConn.BeginBlockSync(block.Hash(), types.TM2PB.Header(block.Header))
	if err != nil {
		log.Warn("Error in proxyAppConn.BeginBlock", "error", err)
		return nil, err
	}

	fail.Fail() // XXX

	// Run txs of block
	for _, tx := range block.Txs {
		fail.FailRand(len(block.Txs)) // XXX
		proxyAppConn.DeliverTxAsync(tx)
		if err := proxyAppConn.Error(); err != nil {
			return nil, err
		}
	}

	fail.Fail() // XXX

	// End block
	respEndBlock, err := proxyAppConn.EndBlockSync(uint64(block.Height))
	if err != nil {
		log.Warn("Error in proxyAppConn.EndBlock", "error", err)
		return nil, err
	}

	fail.Fail() // XXX

	log.Info("Executed block", "height", block.Height, "valid txs", validTxs, "invalid txs", invalidTxs)
	if len(respEndBlock.Diffs) > 0 {
		log.Info("Update to validator set", "updates", abci.ValidatorsString(respEndBlock.Diffs))
	}
	return respEndBlock.Diffs, nil
}

func updateValidators(validators *types.ValidatorSet, changedValidators []*abci.Validator) error {
	// TODO: prevent change of 1/3+ at once

	for _, v := range changedValidators {
		pubkey, err := crypto.PubKeyFromBytes(v.PubKey) // NOTE: expects go-wire encoded pubkey
		if err != nil {
			return err
		}

		address := pubkey.Address()
		power := int64(v.Power)
		// mind the overflow from uint64
		if power < 0 {
			return errors.New(Fmt("Power (%d) overflows int64", v.Power))
		}

		_, val := validators.GetByAddress(address)
		if val == nil {
			// add val
			added := validators.Add(types.NewValidator(pubkey, power))
			if !added {
				return errors.New(Fmt("Failed to add new validator %X with voting power %d", address, power))
			}
		} else if v.Power == 0 {
			// remove val
			_, removed := validators.Remove(address)
			if !removed {
				return errors.New(Fmt("Failed to remove validator %X)"))
			}
		} else {
			// update val
			val.VotingPower = power
			updated := validators.Update(val)
			if !updated {
				return errors.New(Fmt("Failed to update validator %X with voting power %d", address, power))
			}
		}
	}
	return nil
}

// return a bit array of validators that signed the last commit
// NOTE: assumes commits have already been authenticated
func commitBitArrayFromBlock(block *types.Block) *BitArray {
	signed := NewBitArray(len(block.LastCommit.Precommits))
	for i, precommit := range block.LastCommit.Precommits {
		if precommit != nil {
			signed.SetIndex(i, true) // val_.LastCommitHeight = block.Height - 1
		}
	}
	return signed
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

type MockMempool struct {
}

func (m MockMempool) Lock()                             {}
func (m MockMempool) Unlock()                           {}
func (m MockMempool) Update(height int, txs []types.Tx) {}

//----------------------------------------------------------------
// Handshake with app to sync to latest state of core by replaying blocks

// TODO: Should we move blockchain/store.go to its own package?
type BlockStore interface {
	Height() int
	LoadBlock(height int) *types.Block
	LoadBlockMeta(height int) *types.BlockMeta
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

// TODO: retry the handshake/replay if it fails ?
func (h *Handshaker) Handshake(proxyApp proxy.AppConns) error {
	// handshake is done via info request on the query conn
	res, err := proxyApp.Query().InfoSync()
	if err != nil {
		return errors.New(Fmt("Error calling Info: %v", err))
	}

	blockHeight := int(res.LastBlockHeight) // XXX: beware overflow
	appHash := res.LastBlockAppHash

	log.Notice("ABCI Handshake", "appHeight", blockHeight, "appHash", appHash)

	// TODO: check version

	// replay blocks up to the latest in the blockstore
	err = h.ReplayBlocks(appHash, blockHeight, proxyApp.Consensus())
	if err != nil {
		return errors.New(Fmt("Error on replay: %v", err))
	}

	// Save the state
	h.state.Save()

	// TODO: (on restart) replay mempool

	return nil
}

// Replay all blocks after blockHeight and ensure the result matches the current state.
func (h *Handshaker) ReplayBlocks(appHash []byte, appBlockHeight int, appConnConsensus proxy.AppConnConsensus) error {

	storeBlockHeight := h.store.Height()
	stateBlockHeight := h.state.LastBlockHeight
	log.Notice("ABCI Replay Blocks", "appHeight", appBlockHeight, "storeHeight", storeBlockHeight, "stateHeight", stateBlockHeight)

	if storeBlockHeight == 0 {
		return nil
	} else if storeBlockHeight < appBlockHeight {
		// if the app is ahead, there's nothing we can do
		return ErrAppBlockHeightTooHigh{storeBlockHeight, appBlockHeight}

	} else if storeBlockHeight == appBlockHeight {
		// We ran Commit, but if we crashed before state.Save(),
		// load the intermediate state and update the state.AppHash.
		// NOTE: If ABCI allowed rollbacks, we could just replay the
		// block even though it's been committed
		stateAppHash := h.state.AppHash
		lastBlockAppHash := h.store.LoadBlock(storeBlockHeight).AppHash

		if bytes.Equal(stateAppHash, appHash) {
			// we're all synced up
			log.Debug("ABCI RelpayBlocks: Already synced")
		} else if bytes.Equal(stateAppHash, lastBlockAppHash) {
			// we crashed after commit and before saving state,
			// so load the intermediate state and update the hash
			h.state.LoadIntermediate()
			h.state.AppHash = appHash
			log.Debug("ABCI RelpayBlocks: Loaded intermediate state and updated state.AppHash")
		} else {
			PanicSanity(Fmt("Unexpected state.AppHash: state.AppHash %X; app.AppHash %X, lastBlock.AppHash %X", stateAppHash, appHash, lastBlockAppHash))

		}
		return nil

	} else if storeBlockHeight == appBlockHeight+1 &&
		storeBlockHeight == stateBlockHeight+1 {
		// We crashed after saving the block
		// but before Commit (both the state and app are behind),
		// so just replay the block

		// check that the lastBlock.AppHash matches the state apphash
		block := h.store.LoadBlock(storeBlockHeight)
		if !bytes.Equal(block.Header.AppHash, appHash) {
			return ErrLastStateMismatch{storeBlockHeight, block.Header.AppHash, appHash}
		}

		blockMeta := h.store.LoadBlockMeta(storeBlockHeight)

		h.nBlocks += 1
		var eventCache types.Fireable // nil

		// replay the latest block
		return h.state.ApplyBlock(eventCache, appConnConsensus, block, blockMeta.PartsHeader, MockMempool{})
	} else if storeBlockHeight != stateBlockHeight {
		// unless we failed before committing or saving state (previous 2 case),
		// the store and state should be at the same height!
		PanicSanity(Fmt("Expected storeHeight (%d) and stateHeight (%d) to match.", storeBlockHeight, stateBlockHeight))
	} else {
		// store is more than one ahead,
		// so app wants to replay many blocks

		// replay all blocks starting with appBlockHeight+1
		var eventCache types.Fireable // nil

		// TODO: use stateBlockHeight instead and let the consensus state
		// do the replay

		var appHash []byte
		for i := appBlockHeight + 1; i <= storeBlockHeight; i++ {
			h.nBlocks += 1
			block := h.store.LoadBlock(i)
			_, err := execBlockOnProxyApp(eventCache, appConnConsensus, block)
			if err != nil {
				log.Warn("Error executing block on proxy app", "height", i, "err", err)
				return err
			}
			// Commit block, get hash back
			res := appConnConsensus.CommitSync()
			if res.IsErr() {
				log.Warn("Error in proxyAppConn.CommitSync", "error", res)
				return res
			}
			if res.Log != "" {
				log.Info("Commit.Log: " + res.Log)
			}
			appHash = res.Data
		}
		if !bytes.Equal(h.state.AppHash, appHash) {
			return errors.New(Fmt("Tendermint state.AppHash does not match AppHash after replay. Got %X, expected %X", appHash, h.state.AppHash))
		}
		return nil
	}
	return nil
}
