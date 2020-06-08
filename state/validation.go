package state

import (
	"bytes"
	"errors"
	"fmt"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------
// Validate block

func validateBlock(evidencePool EvidencePool, stateDB dbm.DB, state State, block *types.Block) error {
	// Validate internal consistency.
	if err := block.ValidateBasic(); err != nil {
		return err
	}

	// Validate basic info.
	if block.Version.App != state.Version.Consensus.App ||
		block.Version.Block != state.Version.Consensus.Block {
		return fmt.Errorf("wrong Block.Header.Version. Expected %v, got %v",
			state.Version.Consensus,
			block.Version,
		)
	}
	if block.ChainID != state.ChainID {
		return fmt.Errorf("wrong Block.Header.ChainID. Expected %v, got %v",
			state.ChainID,
			block.ChainID,
		)
	}
	if block.Height != state.LastBlockHeight+1 {
		return fmt.Errorf("wrong Block.Header.Height. Expected %v, got %v",
			state.LastBlockHeight+1,
			block.Height,
		)
	}
	// Validate prev block info.
	if !block.LastBlockID.Equals(state.LastBlockID) {
		return fmt.Errorf("wrong Block.Header.LastBlockID.  Expected %v, got %v",
			state.LastBlockID,
			block.LastBlockID,
		)
	}

	// Validate app info
	if !bytes.Equal(block.AppHash, state.AppHash) {
		return fmt.Errorf("wrong Block.Header.AppHash.  Expected %X, got %v",
			state.AppHash,
			block.AppHash,
		)
	}
	hashCP := types.HashConsensusParams(state.ConsensusParams)
	if !bytes.Equal(block.ConsensusHash, hashCP) {
		return fmt.Errorf("wrong Block.Header.ConsensusHash.  Expected %X, got %v",
			hashCP,
			block.ConsensusHash,
		)
	}
	if !bytes.Equal(block.LastResultsHash, state.LastResultsHash) {
		return fmt.Errorf("wrong Block.Header.LastResultsHash.  Expected %X, got %v",
			state.LastResultsHash,
			block.LastResultsHash,
		)
	}
	if !bytes.Equal(block.ValidatorsHash, state.Validators.Hash()) {
		return fmt.Errorf("wrong Block.Header.ValidatorsHash.  Expected %X, got %v",
			state.Validators.Hash(),
			block.ValidatorsHash,
		)
	}
	if !bytes.Equal(block.NextValidatorsHash, state.NextValidators.Hash()) {
		return fmt.Errorf("wrong Block.Header.NextValidatorsHash.  Expected %X, got %v",
			state.NextValidators.Hash(),
			block.NextValidatorsHash,
		)
	}

	// Validate block LastCommit.
	if block.Height == 1 {
		if len(block.LastCommit.Signatures) != 0 {
			return errors.New("block at height 1 can't have LastCommit signatures")
		}
	} else {
		// LastCommit.Signatures length is checked in VerifyCommit.
		if err := state.LastValidators.VerifyCommit(
			state.ChainID, state.LastBlockID, block.Height-1, block.LastCommit); err != nil {
			return err
		}
	}

	// Validate block Time
	if block.Height > 1 {
		if !block.Time.After(state.LastBlockTime) {
			return fmt.Errorf("block time %v not greater than last block time %v",
				block.Time,
				state.LastBlockTime,
			)
		}
		medianTime := MedianTime(block.LastCommit, state.LastValidators)
		if !block.Time.Equal(medianTime) {
			return fmt.Errorf("invalid block time. Expected %v, got %v",
				medianTime,
				block.Time,
			)
		}
	} else if block.Height == 1 {
		genesisTime := state.LastBlockTime
		if !block.Time.Equal(genesisTime) {
			return fmt.Errorf("block time %v is not equal to genesis time %v",
				block.Time,
				genesisTime,
			)
		}
	}

	// Limit the amount of evidence
	numEvidence := len(block.Evidence.Evidence)
	// MaxNumEvidence is capped at uint16, so conversion is always safe.
	if maxEvidence := int(state.ConsensusParams.Evidence.MaxNum); numEvidence > maxEvidence {
		return types.NewErrEvidenceOverflow(maxEvidence, numEvidence)
	}

	// Validate all evidence.
	for idx, ev := range block.Evidence.Evidence {
		// check that no evidence has been submitted more than once
		for i := idx + 1; i < len(block.Evidence.Evidence); i++ {
			if ev.Equal(block.Evidence.Evidence[i]) {
				return types.NewErrEvidenceInvalid(ev, errors.New("evidence was submitted twice"))
			}
		}
		if evidencePool != nil {
			if evidencePool.IsCommitted(ev) {
				return types.NewErrEvidenceInvalid(ev, errors.New("evidence was already committed"))
			}
			if evidencePool.IsPending(ev) {
				continue
			}
		}

		var header *types.Header
		if _, ok := ev.(*types.LunaticValidatorEvidence); ok {
			header = evidencePool.Header(ev.Height())
			if header == nil {
				return fmt.Errorf("don't have block meta at height #%d", ev.Height())
			}
		}

		if err := VerifyEvidence(stateDB, state, ev, header); err != nil {
			return types.NewErrEvidenceInvalid(ev, err)
		}
	}

	// NOTE: We can't actually verify it's the right proposer because we dont
	// know what round the block was first proposed. So just check that it's
	// a legit address and a known validator.
	if len(block.ProposerAddress) != crypto.AddressSize {
		return fmt.Errorf("expected ProposerAddress size %d, got %d",
			crypto.AddressSize,
			len(block.ProposerAddress),
		)
	}
	if !state.Validators.HasAddress(block.ProposerAddress) {
		return fmt.Errorf("block.Header.ProposerAddress %X is not a validator",
			block.ProposerAddress,
		)
	}

	return nil
}

// VerifyEvidence verifies the evidence fully by checking:
// - it is sufficiently recent (MaxAge)
// - it is from a key who was a validator at the given height
// - it is internally consistent
// - it was properly signed by the alleged equivocator
func VerifyEvidence(stateDB dbm.DB, state State, evidence types.Evidence, committedHeader *types.Header) error {
	var (
		height         = state.LastBlockHeight
		evidenceParams = state.ConsensusParams.Evidence

		ageDuration  = state.LastBlockTime.Sub(evidence.Time())
		ageNumBlocks = height - evidence.Height()
	)

	if ageDuration > evidenceParams.MaxAgeDuration && ageNumBlocks > evidenceParams.MaxAgeNumBlocks {
		return fmt.Errorf(
			"evidence from height %d (created at: %v) is too old; min height is %d and evidence can not be older than %v",
			evidence.Height(),
			evidence.Time(),
			height-evidenceParams.MaxAgeNumBlocks,
			state.LastBlockTime.Add(evidenceParams.MaxAgeDuration),
		)
	}
	if ev, ok := evidence.(*types.LunaticValidatorEvidence); ok {
		if err := ev.VerifyHeader(committedHeader); err != nil {
			return err
		}
	}

	valset, err := LoadValidators(stateDB, evidence.Height())
	if err != nil {
		// TODO: if err is just that we cant find it cuz we pruned, ignore.
		// TODO: if its actually bad evidence, punish peer
		return err
	}

	addr := evidence.Address()
	var val *types.Validator

	// For PhantomValidatorEvidence, check evidence.Address was not part of the
	// validator set at height evidence.Height, but was a validator before OR
	// after.
	if phve, ok := evidence.(*types.PhantomValidatorEvidence); ok {
		_, val = valset.GetByAddress(addr)
		if val != nil {
			return fmt.Errorf("address %X was a validator at height %d", addr, evidence.Height())
		}

		// check if last height validator was in the validator set is within
		// MaxAgeNumBlocks.
		if ageNumBlocks > 0 && phve.LastHeightValidatorWasInSet <= ageNumBlocks {
			return fmt.Errorf("last time validator was in the set at height %d, min: %d",
				phve.LastHeightValidatorWasInSet, ageNumBlocks+1)
		}

		valset, err := LoadValidators(stateDB, phve.LastHeightValidatorWasInSet)
		if err != nil {
			// TODO: if err is just that we cant find it cuz we pruned, ignore.
			// TODO: if its actually bad evidence, punish peer
			return err
		}
		_, val = valset.GetByAddress(addr)
		if val == nil {
			return fmt.Errorf("phantom validator %X not found", addr)
		}
	} else {
		// For all other types, expect evidence.Address to be a validator at height
		// evidence.Height.
		_, val = valset.GetByAddress(addr)
		if val == nil {
			return fmt.Errorf("address %X was not a validator at height %d", addr, evidence.Height())
		}
	}

	if err := evidence.Verify(state.ChainID, val.PubKey); err != nil {
		return err
	}

	return nil
}
