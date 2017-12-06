package state

import (
	"errors"
	"fmt"

	fail "github.com/ebuchman/fail-test"
	abci "github.com/tendermint/abci/types"
	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"
)

//--------------------------------------------------
// Execute the block

// ValExecBlock executes the block, but does NOT mutate State.
// + validates the block
// + executes block.Txs on the proxyAppConn
func (s *State) ValExecBlock(txEventPublisher types.TxEventPublisher, proxyAppConn proxy.AppConnConsensus, block *types.Block) (*ABCIResponses, error) {
	// Validate the block.
	if err := s.validateBlock(block); err != nil {
		return nil, ErrInvalidBlock(err)
	}

	// Execute the block txs
	abciResponses, err := execBlockOnProxyApp(txEventPublisher, proxyAppConn, block, s.logger)
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
func execBlockOnProxyApp(txEventPublisher types.TxEventPublisher, proxyAppConn proxy.AppConnConsensus, block *types.Block, logger log.Logger) (*ABCIResponses, error) {
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
			txResult := r.DeliverTx
			if txResult.Code == abci.CodeTypeOK {
				validTxs++
			} else {
				logger.Debug("Invalid tx", "code", txResult.Code, "log", txResult.Log)
				invalidTxs++
			}

			// NOTE: if we count we can access the tx from the block instead of
			// pulling it from the req
			txEventPublisher.PublishEventTx(types.EventDataTx{types.TxResult{
				Height: block.Height,
				Index:  uint32(txIndex),
				Tx:     types.Tx(req.GetDeliverTx().Tx),
				Result: *txResult,
			}})

			abciResponses.DeliverTx[txIndex] = txResult
			txIndex++
		}
	}
	proxyAppConn.SetResponseCallback(proxyCb)

	// Begin block
	_, err := proxyAppConn.BeginBlockSync(abci.RequestBeginBlock{
		block.Hash(),
		types.TM2PB.Header(block.Header),
		nil,
		nil,
	})
	if err != nil {
		logger.Error("Error in proxyAppConn.BeginBlock", "err", err)
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
	abciResponses.EndBlock, err = proxyAppConn.EndBlockSync(abci.RequestEndBlock{block.Height})
	if err != nil {
		logger.Error("Error in proxyAppConn.EndBlock", "err", err)
		return nil, err
	}

	valDiff := abciResponses.EndBlock.Diffs

	logger.Info("Executed block", "height", block.Height, "validTxs", validTxs, "invalidTxs", invalidTxs)
	if len(valDiff) > 0 {
		logger.Info("Update to validator set", "updates", abci.ValidatorsString(valDiff))
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
		// mind the overflow from int64
		if power < 0 {
			return errors.New(cmn.Fmt("Power (%d) overflows int64", v.Power))
		}

		_, val := validators.GetByAddress(address)
		if val == nil {
			// add val
			added := validators.Add(types.NewValidator(pubkey, power))
			if !added {
				return errors.New(cmn.Fmt("Failed to add new validator %X with voting power %d", address, power))
			}
		} else if v.Power == 0 {
			// remove val
			_, removed := validators.Remove(address)
			if !removed {
				return errors.New(cmn.Fmt("Failed to remove validator %X)"))
			}
		} else {
			// update val
			val.VotingPower = power
			updated := validators.Update(val)
			if !updated {
				return errors.New(cmn.Fmt("Failed to update validator %X with voting power %d", address, power))
			}
		}
	}
	return nil
}

// return a bit array of validators that signed the last commit
// NOTE: assumes commits have already been authenticated
/* function is currently unused
func commitBitArrayFromBlock(block *types.Block) *cmn.BitArray {
	signed := cmn.NewBitArray(len(block.LastCommit.Precommits))
	for i, precommit := range block.LastCommit.Precommits {
		if precommit != nil {
			signed.SetIndex(i, true) // val_.LastCommitHeight = block.Height - 1
		}
	}
	return signed
}
*/

//-----------------------------------------------------
// Validate block

// ValidateBlock validates the block against the state.
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
			return errors.New(cmn.Fmt("Invalid block commit size. Expected %v, got %v",
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

// ApplyBlock validates the block against the state, executes it against the app,
// commits it, and saves the block and state. It's the only function that needs to be called
// from outside this package to process and commit an entire block.
func (s *State) ApplyBlock(txEventPublisher types.TxEventPublisher, proxyAppConn proxy.AppConnConsensus,
	block *types.Block, partsHeader types.PartSetHeader, mempool types.Mempool) error {

	abciResponses, err := s.ValExecBlock(txEventPublisher, proxyAppConn, block)
	if err != nil {
		return fmt.Errorf("Exec failed for application: %v", err)
	}

	fail.Fail() // XXX

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

	// save the state and the validators
	s.Save()

	return nil
}

// CommitStateUpdateMempool locks the mempool, runs the ABCI Commit message, and updates the mempool.
// The Mempool must be locked during commit and update because state is typically reset on Commit and old txs must be replayed
// against committed state before new txs are run in the mempool, lest they be invalid.
func (s *State) CommitStateUpdateMempool(proxyAppConn proxy.AppConnConsensus, block *types.Block, mempool types.Mempool) error {
	mempool.Lock()
	defer mempool.Unlock()

	// Commit block, get hash back
	res, err := proxyAppConn.CommitSync()
	if err != nil {
		s.logger.Error("Client error during proxyAppConn.CommitSync", "err", err)
		return err
	}
	if res.IsErr() {
		s.logger.Error("Error in proxyAppConn.CommitSync", "err", res)
		return res
	}
	if res.Log != "" {
		s.logger.Debug("Commit.Log: " + res.Log)
	}

	s.logger.Info("Committed state", "height", block.Height, "txs", block.NumTxs, "hash", res.Data)

	// Set the state's new AppHash
	s.AppHash = res.Data

	// Update mempool.
	return mempool.Update(block.Height, block.Txs)
}

// ExecCommitBlock executes and commits a block on the proxyApp without validating or mutating the state.
// It returns the application root hash (result of abci.Commit).
func ExecCommitBlock(appConnConsensus proxy.AppConnConsensus, block *types.Block, logger log.Logger) ([]byte, error) {
	_, err := execBlockOnProxyApp(types.NopEventBus{}, appConnConsensus, block, logger)
	if err != nil {
		logger.Error("Error executing block on proxy app", "height", block.Height, "err", err)
		return nil, err
	}
	// Commit block, get hash back
	res, err := appConnConsensus.CommitSync()
	if err != nil {
		logger.Error("Client error during proxyAppConn.CommitSync", "err", res)
		return nil, err
	}
	if res.IsErr() {
		logger.Error("Error in proxyAppConn.CommitSync", "err", res)
		return nil, res
	}
	if res.Log != "" {
		logger.Info("Commit.Log: " + res.Log)
	}
	return res.Data, nil
}
