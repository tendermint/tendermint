package state

import (
	"bytes"
	"errors"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tendermint/events"
	"github.com/tendermint/tendermint/types"
)

// NOTE: If an error occurs during block execution, state will be left
// at an invalid state.  Copy the state before calling ExecBlock!
func ExecBlock(s *State, block *types.Block, blockPartsHeader types.PartSetHeader) error {
	err := execBlock(s, block, blockPartsHeader)
	if err != nil {
		return err
	}
	// State.Hash should match block.StateHash
	stateHash := s.Hash()
	if !bytes.Equal(stateHash, block.StateHash) {
		return errors.New(Fmt("Invalid state hash. Expected %X, got %X",
			stateHash, block.StateHash))
	}
	return nil
}

// executes transactions of a block, does not check block.StateHash
// NOTE: If an error occurs during block execution, state will be left
// at an invalid state.  Copy the state before calling execBlock!
func execBlock(s *State, block *types.Block, blockPartsHeader types.PartSetHeader) error {
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
			return errors.New(Fmt("Invalid block validation size. Expected %v, got %v",
				s.LastValidators.Size(), len(block.LastValidation.Precommits)))
		}
		err := s.LastValidators.VerifyValidation(
			s.ChainID, s.LastBlockHash, s.LastBlockParts, block.Height-1, block.LastValidation)
		if err != nil {
			return err
		}
	}

	// Update Validator.LastCommitHeight as necessary.
	for i, precommit := range block.LastValidation.Precommits {
		if precommit == nil {
			continue
		}
		_, val := s.LastValidators.GetByIndex(i)
		if val == nil {
			PanicCrisis(Fmt("Failed to fetch validator at index %v", i))
		}
		if _, val_ := s.Validators.GetByAddress(val.Address); val_ != nil {
			val_.LastCommitHeight = block.Height - 1
			updated := s.Validators.Update(val_)
			if !updated {
				PanicCrisis("Failed to update validator LastCommitHeight")
			}
		} else {
			PanicCrisis("Could not find validator")
		}
	}

	// Remember LastValidators
	s.LastValidators = s.Validators.Copy()

	// Execute each tx
	for _, tx := range block.Data.Txs {
		err := ExecTx(s, tx, s.evc)
		if err != nil {
			return InvalidTxError{tx, err}
		}
	}

	// Increment validator AccumPowers
	s.Validators.IncrementAccum(1)
	s.LastBlockHeight = block.Height
	s.LastBlockHash = block.Hash()
	s.LastBlockParts = blockPartsHeader
	s.LastBlockTime = block.Time
	return nil
}

// If the tx is invalid, an error will be returned.
// Unlike ExecBlock(), state will not be altered.
func ExecTx(s *State, tx types.Tx, evc events.Fireable) (err error) {

	// TODO: do something with fees
	//fees := int64(0)
	//_s := blockCache.State() // hack to access validators and block height

	// XXX Query ledger application
	return nil

}

//-----------------------------------------------------------------------------

type InvalidTxError struct {
	Tx     types.Tx
	Reason error
}

func (txErr InvalidTxError) Error() string {
	return Fmt("Invalid tx: [%v] reason: [%v]", txErr.Tx, txErr.Reason)
}
