package commands

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/os"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/version"
	db "github.com/tendermint/tm-db"
)

func MakeRollbackStateCommand() *cobra.Command {
	confg := cfg.DefaultConfig()
	return &cobra.Command{
		Use:   "rollback",
		Short: "rollback tendermint state by one height",
		Long: `
A state rollback is performed to recover from an incorrect application state transition,
when Tendermint has persisted an incorrect app hash and is thus unable to make
progress. Rollback overwrites a state at height n with the state at height n - 1.
The application should also roll back to height n - 1. No blocks are removed, so upon
restarting Tendermint the transactions in block n will be re-executed against the
application.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			height, hash, err := RollbackState(confg)
			if err != nil {
				return fmt.Errorf("failed to rollback state: %w", err)
			}

			fmt.Printf("Rolled back state to height %d and hash %X", height, hash)
			return nil
		},
	}
}

// RollbackState takes the state at the current height n and overwrites it with the state
// at height n - 1. Note state here refers to tendermint state not application state.
// Returns the latest state height and app hash alongside an error if there was one.
func RollbackState(config *cfg.Config) (int64, []byte, error) {
	// use the parsed config to load the block and state store
	blocksdb, statedb, err := loadStateAndBlockStore(config)
	if err != nil {
		return -1, nil, err
	}
	// rollback the last state
	return rollbackState(blocksdb, statedb)
}

func loadStateAndBlockStore(cfg *cfg.Config) (db.DB, db.DB, error) {
	dbType := db.DBBackendType(cfg.DBBackend)

	if !os.FileExists(filepath.Join(cfg.DBDir(), "blockstore.db")) {
		return nil, nil, fmt.Errorf("no blockstore found in %v", cfg.DBDir())
	}

	// Get BlockStore
	blockStoreDB := db.NewDB("blockstore", dbType, cfg.DBDir())

	if !os.FileExists(filepath.Join(cfg.DBDir(), "state.db")) {
		return nil, nil, fmt.Errorf("no blockstore found in %v", cfg.DBDir())
	}

	// Get StateStore
	stateDB := db.NewDB("state", dbType, cfg.DBDir())

	return blockStoreDB, stateDB, nil
}

func rollbackState(blockStoreDB, stateDB db.DB) (int64, []byte, error) {
	blockStore := store.NewBlockStore(blockStoreDB)
	invalidState := state.LoadState(stateDB)

	height := blockStore.Height()
	// skip
	if height == invalidState.LastBlockHeight+1 {
		return invalidState.LastBlockHeight, invalidState.AppHash, nil
	}

	if height != invalidState.LastBlockHeight {
		return -1, nil, fmt.Errorf("statestore height (%d) is not one below or equal to blockstore height (%d)",
			invalidState.LastBlockHeight, height)
	}

	// rollback height
	rollbackHeight := invalidState.LastBlockHeight - 1
	rollbackBlock := blockStore.LoadBlockMeta(rollbackHeight)
	if rollbackBlock == nil {
		return -1, nil, fmt.Errorf("block at height %d not found", rollbackHeight)
	}

	// we also need to retrieve the latest block because the app hash and last results hash is only agreed upon in the following block
	latestBlock := blockStore.LoadBlockMeta(invalidState.LastBlockHeight)
	if latestBlock == nil {
		return -1, nil, fmt.Errorf("block at height %d not found", invalidState.LastBlockHeight)
	}

	// previous validators
	previousLastValidatorSet, err := state.LoadValidators(stateDB, rollbackHeight)
	if err != nil {
		return -1, nil, err
	}

	previousParams, err := state.LoadConsensusParams(stateDB, rollbackHeight)
	if err != nil {
		return -1, nil, err
	}

	valChangeHeight := invalidState.LastHeightValidatorsChanged
	// this can only happen if the validator set changed since the last block
	if valChangeHeight > rollbackHeight {
		valChangeHeight = rollbackHeight + 1
	}

	paramsChangeHeight := invalidState.LastHeightConsensusParamsChanged
	// this can only happen if params changed from the last block
	if paramsChangeHeight > rollbackHeight {
		paramsChangeHeight = rollbackHeight + 1
	}

	// build the new state from the old state and the prior block
	rolledBackState := state.State{
		Version: state.Version{
			Consensus: version.Consensus{
				Block: version.BlockProtocol,
				App:   0,
			},
			Software: version.TMCoreSemVer,
		},
		// immutable fields
		ChainID: invalidState.ChainID,

		LastBlockHeight:  rollbackBlock.Header.Height,
		LastBlockTotalTx: rollbackBlock.Header.TotalTxs,
		LastBlockID:      rollbackBlock.BlockID,
		LastBlockTime:    rollbackBlock.Header.Time,

		NextValidators:              invalidState.Validators,
		Validators:                  invalidState.LastValidators,
		LastValidators:              previousLastValidatorSet,
		LastHeightValidatorsChanged: valChangeHeight,

		ConsensusParams:                  previousParams,
		LastHeightConsensusParamsChanged: paramsChangeHeight,

		LastResultsHash: latestBlock.Header.LastResultsHash,
		AppHash:         latestBlock.Header.AppHash,
	}

	// saving the state
	state.SaveState(stateDB, rolledBackState)
	return rolledBackState.LastBlockHeight, rolledBackState.AppHash, nil
}
