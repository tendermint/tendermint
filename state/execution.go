package state

import (
	"errors"
	"fmt"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-events"
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
func (s *State) ExecBlock(evsw *events.EventSwitch, proxyAppConn proxy.AppConn, block *types.Block, blockPartsHeader types.PartSetHeader) error {

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
	err = s.execBlockOnProxyApp(evsw, proxyAppConn, block)
	if err != nil {
		// There was some error in proxyApp
		// TODO Report error and wait for proxyApp to be available.
		return err
	}

	// All good!
	nextValSet.IncrementAccum(1)
	s.LastBlockHeight = block.Height
	s.LastBlockHash = block.Hash()
	s.LastBlockParts = blockPartsHeader
	s.LastBlockTime = block.Time
	s.Validators = nextValSet
	s.LastValidators = valSet

	return nil
}

// Executes block's transactions on proxyAppConn.
// TODO: Generate a bitmap or otherwise store tx validity in state.
func (s *State) execBlockOnProxyApp(evsw *events.EventSwitch, proxyAppConn proxy.AppConn, block *types.Block) error {

	var validTxs, invalidTxs = 0, 0

	// Execute transactions and get hash
	proxyCb := func(req *tmsp.Request, res *tmsp.Response) {
		switch res.Type {
		case tmsp.MessageType_AppendTx:
			// TODO: make use of res.Log
			// TODO: make use of this info
			// Blocks may include invalid txs.
			// reqAppendTx := req.(tmsp.RequestAppendTx)
			if res.Code == tmsp.CodeType_OK {
				validTxs += 1
			} else {
				log.Debug("Invalid tx", "code", res.Code, "log", res.Log)
				invalidTxs += 1
			}
		}
	}
	proxyAppConn.SetResponseCallback(proxyCb)

	// Run next txs in the block and get new AppHash
	for _, tx := range block.Txs {
		proxyAppConn.AppendTxAsync(tx)
		if err := proxyAppConn.Error(); err != nil {
			return err
		}
	}
	hash, logStr, err := proxyAppConn.GetHashSync()
	if err != nil {
		log.Warn("Error computing proxyAppConn hash", "error", err)
		return err
	}
	if logStr != "" {
		log.Debug("GetHash.Log: " + logStr)
	}
	log.Info(Fmt("ExecBlock got %v valid txs and %v invalid txs", validTxs, invalidTxs))

	// Set the state's new AppHash
	s.AppHash = hash

	return nil
}

func (s *State) validateBlock(block *types.Block) error {
	// Basic block validation.
	err := block.ValidateBasic(s.ChainID, s.LastBlockHeight, s.LastBlockHash, s.LastBlockParts, s.LastBlockTime, s.AppHash)
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
// Assumes that lastValSet matches the valset of block.LastValidation
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
	Tx   types.Tx
	Code tmsp.CodeType
}

func (txErr InvalidTxError) Error() string {
	return Fmt("Invalid tx: [%v] code: [%v]", txErr.Tx, txErr.Code)
}
