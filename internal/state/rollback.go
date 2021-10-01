package state

import (
	"errors"
	"fmt"

	"github.com/tendermint/tendermint/version"
)

// Rollback takes a state at height n and overwrites it with the state
// at height n - 1. Note state here refers to tendermint state not application state.
func Rollback(bs BlockStore, ss Store) (int64, []byte, error) {
	invalidState, err := ss.Load()
	if err != nil {
		return -1, []byte{}, err
	}
	if invalidState.IsEmpty() {
		return -1, []byte{}, errors.New("no state found")
	}

	rollbackHeight := invalidState.LastBlockHeight
	rollbackBlock := bs.LoadBlockMeta(rollbackHeight)
	if rollbackBlock == nil {
		return -1, []byte{}, fmt.Errorf("block at height %d not found", rollbackHeight)
	}

	previousValidatorSet, err := ss.LoadValidators(rollbackHeight - 1)
	if err != nil {
		return -1, []byte{}, err
	}

	previousParams, err := ss.LoadConsensusParams(rollbackHeight)
	if err != nil {
		return -1, []byte{}, err
	}

	valChangeHeight := invalidState.LastHeightValidatorsChanged
	// this can only happen if the validator set changed since the last block
	if valChangeHeight > rollbackHeight {
		valChangeHeight = rollbackHeight
	}

	paramsChangeHeight := invalidState.LastHeightConsensusParamsChanged
	// this can only happen if params changed from the last block
	if paramsChangeHeight > rollbackHeight {
		paramsChangeHeight = rollbackHeight
	}

	// build the new state from the old state and the prior block
	newState := State{
		Version: Version{
			Consensus: version.Consensus{
				Block: version.BlockProtocol,
				App:   previousParams.Version.AppVersion,
			},
			Software: version.TMVersion,
		},
		// immutable fields
		ChainID:       invalidState.ChainID,
		InitialHeight: invalidState.InitialHeight,

		LastBlockHeight: invalidState.LastBlockHeight - 1,
		LastBlockID:     rollbackBlock.Header.LastBlockID,
		LastBlockTime:   rollbackBlock.Header.Time,

		NextValidators:              invalidState.Validators,
		Validators:                  invalidState.LastValidators,
		LastValidators:              previousValidatorSet,
		LastHeightValidatorsChanged: valChangeHeight,

		ConsensusParams:                  previousParams,
		LastHeightConsensusParamsChanged: paramsChangeHeight,

		LastResultsHash: rollbackBlock.Header.LastResultsHash,
		AppHash:         rollbackBlock.Header.AppHash,
	}

	// persist the new state. This overrides the invalid one. NOTE: this will also
	// persist the validator set and consensus params over the existing structures,
	// but both should be the same
	if err := ss.Save(newState); err != nil {
		return -1, []byte{}, err
	}

	return newState.LastBlockHeight, newState.AppHash, nil
}
