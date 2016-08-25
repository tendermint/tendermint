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
func (s *State) ExecBlock(eventCache events.Fireable, proxyAppConn proxy.AppConnConsensus, block *types.Block, blockPartsHeader types.PartSetHeader) error {

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
func (s *State) execBlockOnProxyApp(eventCache events.Fireable, proxyAppConn proxy.AppConnConsensus, block *types.Block) error {

	var validTxs, invalidTxs = 0, 0

	// Execute transactions and get hash
	proxyCb := func(req *tmsp.Request, res *tmsp.Response) {
		switch r := res.Value.(type) {
		case *tmsp.Response_AppendTx:
			// TODO: make use of res.Log
			// TODO: make use of this info
			// Blocks may include invalid txs.
			// reqAppendTx := req.(tmsp.RequestAppendTx)
			if r.AppendTx.Code == tmsp.CodeType_OK {
				validTxs += 1
			} else {
				log.Debug("Invalid tx", "code", r.AppendTx.Code, "log", r.AppendTx.Log)
				invalidTxs += 1
			}
			// NOTE: if we count we can access the tx from the block instead of
			// pulling it from the req
			eventCache.FireEvent(types.EventStringTx(req.GetAppendTx().Tx), res)
		}
	}
	proxyAppConn.SetResponseCallback(proxyCb)

	// TODO: BeginBlock

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
	log.Info("TODO: Do something with changedValidators", changedValidators)

	log.Info(Fmt("ExecBlock got %v valid txs and %v invalid txs", validTxs, invalidTxs))
	return nil
}

func (s *State) validateBlock(block *types.Block) error {
	// Basic block validation.
	err := block.ValidateBasic(s.ChainID, s.LastBlockHeight, s.LastBlockHash, s.LastBlockParts, s.LastBlockTime, s.AppHash)
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
			s.ChainID, s.LastBlockHash, s.LastBlockParts, block.Height-1, block.LastCommit)
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
