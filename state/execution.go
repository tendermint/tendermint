package state

import (
	"bytes"
	"errors"
	"fmt"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
	tmsp "github.com/tendermint/tmsp/types"
)

// Execute the block to mutate State.
// Also, execute txs on the proxyAppCtx and validate apphash
// Rolls back before executing transactions.
// Rolls back if invalid, but never commits.
func (s *State) ExecBlock(proxyAppCtx proxy.AppContext, block *types.Block, blockPartsHeader types.PartSetHeader) error {

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

	// First, rollback.
	proxyAppCtx.RollbackSync()

	// Execute, or rollback. (Does not commit)
	err = s.execBlockOnProxyApp(proxyAppCtx, block)
	if err != nil {
		proxyAppCtx.RollbackSync()
		return err
	}

	// All good!
	nextValSet.IncrementAccum(1)
	s.Validators = nextValSet
	s.LastValidators = valSet
	s.LastAppHash = block.AppHash
	s.LastBlockHeight = block.Height
	s.LastBlockHash = block.Hash()
	s.LastBlockParts = blockPartsHeader
	s.LastBlockTime = block.Time

	return nil
}

// Commits block on proxyAppCtx.
func (s *State) Commit(proxyAppCtx proxy.AppContext) error {
	err := proxyAppCtx.CommitSync()
	return err
}

// Executes transactions on proxyAppCtx.
func (s *State) execBlockOnProxyApp(proxyAppCtx proxy.AppContext, block *types.Block) error {
	// Execute transactions and get hash
	var invalidTxErr error
	proxyCb := func(req tmsp.Request, res tmsp.Response) {
		switch res := res.(type) {
		case tmsp.ResponseAppendTx:
			reqAppendTx := req.(tmsp.RequestAppendTx)
			if res.RetCode != tmsp.RetCodeOK {
				if invalidTxErr == nil {
					invalidTxErr = InvalidTxError{reqAppendTx.TxBytes, res.RetCode}
				}
			}
		case tmsp.ResponseEvent:
			s.evc.FireEvent(types.EventStringApp(), types.EventDataApp{res.Key, res.Data})
		}
	}
	proxyAppCtx.SetResponseCallback(proxyCb)
	for _, tx := range block.Data.Txs {
		proxyAppCtx.AppendTxAsync(tx)
		if err := proxyAppCtx.Error(); err != nil {
			return err
		}
	}
	hash, err := proxyAppCtx.GetHashSync()
	if err != nil {
		log.Warn("Error computing proxyAppCtx hash", "error", err)
		return err
	}
	if invalidTxErr != nil {
		log.Warn("Invalid transaction in block")
		return invalidTxErr
	}

	// Check that appHash matches
	if !bytes.Equal(block.AppHash, hash) {
		log.Warn(Fmt("App hash in proposal was %X, computed %X instead", block.AppHash, hash))
		return InvalidAppHashError{block.AppHash, hash}
	}

	return nil
}

func (s *State) validateBlock(block *types.Block) error {
	// Basic block validation.
	err := block.ValidateBasic(s.ChainID, s.LastBlockHeight, s.LastBlockHash, s.LastBlockParts, s.LastBlockTime)
	if err != nil {
		return err
	}

	// Validate block LastValidation.
	if block.Height == 1 {
		if len(block.LastValidation.Precommits) != 0 {
			return errors.New("Block at height 1 (first block) should have no LastValidation precommits")
		}
	} else {
		if len(block.LastValidation.Precommits) != s.LastValidators.Size() {
			return fmt.Errorf("Invalid block validation size. Expected %v, got %v",
				s.LastValidators.Size(), len(block.LastValidation.Precommits))
		}
		err := s.LastValidators.VerifyValidation(
			s.ChainID, s.LastBlockHash, s.LastBlockParts, block.Height-1, block.LastValidation)
		if err != nil {
			return err
		}
	}

	return nil
}

// Updates the LastCommitHeight of the validators in valSet, in place.
// Assumes that lastValSet matches the valset of block.LastValidators
// CONTRACT: lastValSet is not mutated.
func updateValidatorsWithBlock(lastValSet *types.ValidatorSet, valSet *types.ValidatorSet, block *types.Block) {

	for i, precommit := range block.LastValidation.Precommits {
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
	Tx types.Tx
	tmsp.RetCode
}

func (txErr InvalidTxError) Error() string {
	return Fmt("Invalid tx: [%v] code: [%v]", txErr.Tx, txErr.RetCode)
}

type InvalidAppHashError struct {
	Expected []byte
	Got      []byte
}

func (hashErr InvalidAppHashError) Error() string {
	return Fmt("Invalid hash: [%X] got: [%X]", hashErr.Expected, hashErr.Got)
}
