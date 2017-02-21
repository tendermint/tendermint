package state

import (
	"errors"

	"github.com/ebuchman/fail-test"

	abci "github.com/tendermint/abci/types"
	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
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
	block *types.Block, partsHeader types.PartSetHeader, mempool types.Mempool) error {

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

// Apply and commit a block, but without all the state validation.
// Returns the application root hash (result of abci.Commit)
func ApplyBlock(appConnConsensus proxy.AppConnConsensus, block *types.Block) ([]byte, error) {
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
