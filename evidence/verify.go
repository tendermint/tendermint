package evidence

import (
	"fmt"

	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

// VerifyEvidence verifies the evidence fully by checking:
// - it is sufficiently recent (MaxAge)
// - it is from a key who was a validator at the given height
// - it is internally consistent
// - it was properly signed by the alleged equivocator
func VerifyEvidence(evidence types.Evidence, state sm.State, stateDB StateStore, blockStore BlockStore) error {
	var (
		height         = state.LastBlockHeight
		evidenceParams = state.ConsensusParams.Evidence

		ageDuration  = state.LastBlockTime.Sub(evidence.Time())
		ageNumBlocks = height - evidence.Height()

		header *types.Header
	)

	// if the evidence is from the current height - this means the evidence is fresh from the consensus
	// and we won't have it in the block store. We thus check that the time isn't before the previous block
	if evidence.Height() == height+1 {
		if evidence.Time().Before(state.LastBlockTime) {
			return fmt.Errorf("evidence is from an earlier time than the previous block: %v < %v",
				evidence.Time(),
				state.LastBlockTime)
		}
	} else {
		// try to retrieve header from blockstore
		blockMeta := blockStore.LoadBlockMeta(evidence.Height())
		header = &blockMeta.Header
		if header == nil {
			return fmt.Errorf("don't have header at height #%d", evidence.Height())
		}
		if header.Time != evidence.Time() {
			return fmt.Errorf("evidence time (%v) is different to the time of the header we have for the same height (%v)",
				evidence.Time(),
				header.Time,
			)
		}
	}

	if ageDuration > evidenceParams.MaxAgeDuration && ageNumBlocks > evidenceParams.MaxAgeNumBlocks {
		return fmt.Errorf(
			"evidence from height %d (created at: %v) is too old; min height is %d and evidence can not be older than %v",
			evidence.Height(),
			evidence.Time(),
			height-evidenceParams.MaxAgeNumBlocks,
			state.LastBlockTime.Add(evidenceParams.MaxAgeDuration),
		)
	}

	valset, err := stateDB.LoadValidators(evidence.Height())
	if err != nil {
		return err
	}

	if ae, ok := evidence.(*types.AmnesiaEvidence); ok {
		// check the validator set against the polc to make sure that a majority of valid votes was reached
		if !ae.Polc.IsAbsent() {
			err = ae.Polc.ValidateVotes(valset, state.ChainID)
			if err != nil {
				return fmt.Errorf("amnesia evidence contains invalid polc, err: %w", err)
			}
		}
	}

	addr := evidence.Address()
	var val *types.Validator

	// For all other types, expect evidence.Address to be a validator at height
	// evidence.Height.
	_, val = valset.GetByAddress(addr)
	if val == nil {
		return fmt.Errorf("address %X was not a validator at height %d", addr, evidence.Height())
	}

	if err := evidence.Verify(state.ChainID, val.PubKey); err != nil {
		return err
	}

	return nil
}
