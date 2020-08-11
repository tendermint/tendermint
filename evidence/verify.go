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
		
		blockMeta = blockStore.LoadBlockMeta(evidence.Height())
		committedHeader = &blockMeta.Header
	)
	
	if committedHeader.Time != evidence.Time() {
		return fmt.Errorf("evidence time (%v) is different to the time of the header we have for the same height (%v)",
			evidence.Time(),
			committedHeader.Time,
		)
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
	if ev, ok := evidence.(*types.LunaticValidatorEvidence); ok {
		if err := ev.VerifyHeader(committedHeader); err != nil {
			return err
		}
	}

	valset, err := stateDB.LoadValidators(evidence.Height())
	if err != nil {
		// TODO: if err is just that we cant find it cuz we pruned, ignore.
		// TODO: if its actually bad evidence, punish peer
		return err
	}

	addr := evidence.Address()
	var val *types.Validator

	if ae, ok := evidence.(*types.AmnesiaEvidence); ok {
		// check the validator set against the polc to make sure that a majority of valid votes was reached
		if !ae.Polc.IsAbsent() {
			err = ae.Polc.ValidateVotes(valset, state.ChainID)
			if err != nil {
				return fmt.Errorf("amnesia evidence contains invalid polc, err: %w", err)
			}
		}
	}

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