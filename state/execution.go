package state

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/ebuchman/fail-test"

	abci "github.com/tendermint/abci/types"
	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
)

// ExecBlock executes the block to mutate State.
// + validates the block
// + executes block.Txs on the proxyAppConn
// + updates validator sets
// + returns block.Txs results
func (s *State) ExecBlock(eventCache types.Fireable, proxyAppConn proxy.AppConnConsensus, block *types.Block, blockPartsHeader types.PartSetHeader) ([]*types.TxResult, error) {
	// Validate the block.
	if err := s.validateBlock(block); err != nil {
		return nil, ErrInvalidBlock(err)
	}

	// compute bitarray of validators that signed
	signed := commitBitArrayFromBlock(block)
	_ = signed // TODO send on begin block

	// copy the valset
	valSet := s.Validators.Copy()
	nextValSet := valSet.Copy()

	// Execute the block txs
	txResults, changedValidators, err := execBlockOnProxyApp(eventCache, proxyAppConn, block)
	if err != nil {
		// There was some error in proxyApp
		// TODO Report error and wait for proxyApp to be available.
		return nil, ErrProxyAppConn(err)
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

	return txResults, nil
}

// Executes block's transactions on proxyAppConn.
// Returns a list of transaction results and updates to the validator set
// TODO: Generate a bitmap or otherwise store tx validity in state.
func execBlockOnProxyApp(eventCache types.Fireable, proxyAppConn proxy.AppConnConsensus, block *types.Block) ([]*types.TxResult, []*abci.Validator, error) {

	var validTxs, invalidTxs = 0, 0

	var i = 0
	txResults := make([]*types.TxResult, len(block.Txs))

	// Execute transactions and get hash
	proxyCb := func(req *abci.Request, res *abci.Response) {
		switch r := res.Value.(type) {
		case *abci.Response_DeliverTx:
			// TODO: make use of res.Log
			// TODO: make use of this info
			// Blocks may include invalid txs.
			// reqDeliverTx := req.(abci.RequestDeliverTx)
			txError := ""
			txResult := r.DeliverTx
			if txResult.Code == abci.CodeType_OK {
				validTxs++
			} else {
				log.Debug("Invalid tx", "code", txResult.Code, "log", txResult.Log)
				invalidTxs++
				txError = txResult.Code.String()
			}

			tx := req.GetDeliverTx().Tx

			txResults[i] = &types.TxResult{tx, block.Height, *txResult}
			i++

			// NOTE: if we count we can access the tx from the block instead of
			// pulling it from the req
			event := types.EventDataTx{
				Tx:    tx,
				Data:  txResult.Data,
				Code:  txResult.Code,
				Log:   txResult.Log,
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
		return nil, nil, err
	}

	fail.Fail() // XXX

	// Run txs of block
	for _, tx := range block.Txs {
		fail.FailRand(len(block.Txs)) // XXX
		proxyAppConn.DeliverTxAsync(tx)
		if err := proxyAppConn.Error(); err != nil {
			return nil, nil, err
		}
	}

	fail.Fail() // XXX

	// End block
	respEndBlock, err := proxyAppConn.EndBlockSync(uint64(block.Height))
	if err != nil {
		log.Warn("Error in proxyAppConn.EndBlock", "error", err)
		return nil, nil, err
	}

	fail.Fail() // XXX

	log.Info("Executed block", "height", block.Height, "valid txs", validTxs, "invalid txs", invalidTxs)
	if len(respEndBlock.Diffs) > 0 {
		log.Info("Update to validator set", "updates", abci.ValidatorsString(respEndBlock.Diffs))
	}
	return txResults, respEndBlock.Diffs, nil
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

// ApplyBlock executes the block, then commits and updates the mempool
// atomically, then saves transaction results into `BlockStore`.
func (s *State) ApplyBlock(eventCache types.Fireable, proxyAppConn proxy.AppConnConsensus,
	block *types.Block, partsHeader types.PartSetHeader, mempool Mempool, blockStore BlockStore) error {

	txResults, err := s.ExecBlock(eventCache, proxyAppConn, block, partsHeader)
	if err != nil {
		return fmt.Errorf("Exec failed for application: %v", err)
	}

	// lock mempool, commit state, update mempoool
	err = s.CommitStateUpdateMempool(proxyAppConn, block, mempool)
	if err != nil {
		return fmt.Errorf("Commit failed for application: %v", err)
	}

	for _, r := range txResults {
		blockStore.SaveTxResult(r.Tx.Hash(), r)
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

type MockMempool struct{}

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
	SaveTxResult(hash []byte, txResul *types.TxResult)
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
		// FIXME should we resave tx results here while replaying blocks?
		return h.state.ApplyBlock(eventCache, appConnConsensus, block, blockMeta.PartsHeader, MockMempool{}, h.store)
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
			_, _, err := execBlockOnProxyApp(eventCache, appConnConsensus, block)
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
