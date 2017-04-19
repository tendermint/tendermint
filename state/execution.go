package state

import (
	"errors"
	"fmt"

	fail "github.com/ebuchman/fail-test"
	abci "github.com/tendermint/abci/types"
	. "github.com/tendermint/go-common"
	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/state/txindex"
	"github.com/tendermint/tendermint/types"
)

//--------------------------------------------------
// Execute the block

// ValExecBlock executes the block, but does NOT mutate State.
// + validates the block
// + executes block.Txs on the proxyAppConn
func (s *State) ValExecBlock(eventCache types.Fireable, proxyAppConn proxy.AppConnConsensus, block *types.Block) (*ABCIResponses, error) {
	// Validate the block.
	if err := s.validateBlock(block); err != nil {
		return nil, ErrInvalidBlock(err)
	}

	// Execute the block txs
	abciResponses, err := execBlockOnProxyApp(eventCache, proxyAppConn, block)
	if err != nil {
		// There was some error in proxyApp
		// TODO Report error and wait for proxyApp to be available.
		return nil, ErrProxyAppConn(err)
	}

	return abciResponses, nil
}

// Executes block's transactions on proxyAppConn.
// Returns a list of transaction results and updates to the validator set
// TODO: Generate a bitmap or otherwise store tx validity in state.
func execBlockOnProxyApp(eventCache types.Fireable, proxyAppConn proxy.AppConnConsensus, block *types.Block) (*ABCIResponses, error) {
	var validTxs, invalidTxs = 0, 0

	txIndex := 0
	abciResponses := NewABCIResponses(block)

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

			abciResponses.DeliverTx[txIndex] = txResult
			txIndex++

			// NOTE: if we count we can access the tx from the block instead of
			// pulling it from the req
			event := types.EventDataTx{
				Height: block.Height,
				Tx:     types.Tx(req.GetDeliverTx().Tx),
				Data:   txResult.Data,
				Code:   txResult.Code,
				Log:    txResult.Log,
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
		return nil, err
	}

	// Run txs of block
	for _, tx := range block.Txs {
		proxyAppConn.DeliverTxAsync(tx)
		if err := proxyAppConn.Error(); err != nil {
			return nil, err
		}
	}

	// End block
	abciResponses.EndBlock, err = proxyAppConn.EndBlockSync(uint64(block.Height))
	if err != nil {
		log.Warn("Error in proxyAppConn.EndBlock", "error", err)
		return nil, err
	}

	valDiff := abciResponses.EndBlock.Diffs

	log.Info("Executed block", "height", block.Height, "valid txs", validTxs, "invalid txs", invalidTxs)
	if len(valDiff) > 0 {
		log.Info("Update to validator set", "updates", abci.ValidatorsString(valDiff))
	}

	return abciResponses, nil
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
// ApplyBlock validates & executes the block, updates state w/ ABCI responses,
// then commits and updates the mempool atomically, then saves state.
// Transaction results are optionally indexed.

// Validate, execute, and commit block against app, save block and state
func (s *State) ApplyBlock(eventCache types.Fireable, proxyAppConn proxy.AppConnConsensus,
	block *types.Block, partsHeader types.PartSetHeader, mempool types.Mempool) error {

	abciResponses, err := s.ValExecBlock(eventCache, proxyAppConn, block)
	if err != nil {
		return fmt.Errorf("Exec failed for application: %v", err)
	}

	fail.Fail() // XXX

	// index txs. This could run in the background
	s.indexTxs(abciResponses)

	// save the results before we commit
	s.SaveABCIResponses(abciResponses)

	fail.Fail() // XXX

	// now update the block and validators
	s.SetBlockAndValidators(block.Header, partsHeader, abciResponses)

	// lock mempool, commit state, update mempoool
	err = s.CommitStateUpdateMempool(proxyAppConn, block, mempool)
	if err != nil {
		return fmt.Errorf("Commit failed for application: %v", err)
	}

	fail.Fail() // XXX

	// save the state
	s.Save()

	return nil
}

// mempool must be locked during commit and update
// because state is typically reset on Commit and old txs must be replayed
// against committed state before new txs are run in the mempool, lest they be invalid
func (s *State) CommitStateUpdateMempool(proxyAppConn proxy.AppConnConsensus, block *types.Block, mempool types.Mempool) error {
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

	log.Info("Committed state", "hash", res.Data)
	// Set the state's new AppHash
	s.AppHash = res.Data

	// Update mempool.
	mempool.Update(block.Height, block.Txs)

	return nil
}

func (s *State) indexTxs(abciResponses *ABCIResponses) {
	// save the tx results using the TxIndexer
	// NOTE: these may be overwriting, but the values should be the same.
	batch := txindex.NewBatch(len(abciResponses.DeliverTx))
	for i, d := range abciResponses.DeliverTx {
		tx := abciResponses.txs[i]
		batch.Add(types.TxResult{
			Height: uint64(abciResponses.Height),
			Index:  uint32(i),
			Tx:     tx,
			Result: *d,
		})
	}
	s.TxIndexer.AddBatch(batch)
}

// Exec and commit a block on the proxyApp without validating or mutating the state
// Returns the application root hash (result of abci.Commit)
func ExecCommitBlock(appConnConsensus proxy.AppConnConsensus, block *types.Block) ([]byte, error) {
	var eventCache types.Fireable // nil
	_, err := execBlockOnProxyApp(eventCache, appConnConsensus, block)
	if err != nil {
		log.Warn("Error executing block on proxy app", "height", block.Height, "err", err)
		return nil, err
	}
	// Commit block, get hash back
	res := appConnConsensus.CommitSync()
	if res.IsErr() {
		log.Warn("Error in proxyAppConn.CommitSync", "error", res)
		return nil, res
	}
	if res.Log != "" {
		log.Info("Commit.Log: " + res.Log)
	}
	return res.Data, nil
}
