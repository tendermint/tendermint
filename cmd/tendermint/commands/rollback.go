package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/version"
)

var RollbackStateCmd = &cobra.Command{
	Use:   "rollback",
	Short: "rollback tendermint state by one height",
	Long: `
rollbacking state should be performed in the event of a non-deterministic app hash where 
Tendermint has persisted an incorrect app hash and is thus stuck and unable to make 
progress. By default this will rollback the very last height. Note: this does not remove 
any blocks from the blockstore, rather alter the state store such that Tendermint can 
replay blocks from the new starting point.
`,
	Example: `
tendermint rollback
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// use the parsed config to load the block and state store
		bs, ss, err := loadStateAndBlockStore(config)
		if err != nil {
			return err
		}

		// rollback the last state
		return rollbackState(bs, ss)
	},
}

// rollbackState takes a state at height n and overwrites it with the state
// at height n - 1. Note state here refers to tendermint state not application state.
func rollbackState(bs state.BlockStore, ss state.Store) error {
	invalidState, err := ss.Load()
	if err != nil {
		return err
	}

	rollbackHeight := invalidState.LastBlockHeight
	rollbackBlock := bs.LoadBlockMeta(rollbackHeight)
	if rollbackBlock == nil {
		return fmt.Errorf("block at height %d not found", rollbackHeight)
	}

	previousValidatorSet, err := ss.LoadValidators(rollbackHeight - 1)
	if err != nil {
		return err
	}

	previousParams, err := ss.LoadConsensusParams(rollbackHeight)
	if err != nil {
		return err
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
	newState := state.State{
		Version: state.Version{
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
		return err
	}

	fmt.Printf("Rolled back state to height %d and hash %v", rollbackHeight, rollbackBlock.Header.AppHash)
	return nil
}
