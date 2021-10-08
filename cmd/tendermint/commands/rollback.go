package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	dbm "github.com/tendermint/tm-db"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
)

var RollbackStateCmd = &cobra.Command{
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
		height, hash, err := RollbackState(config)
		if err != nil {
			return fmt.Errorf("failed to rollback state: %w", err)
		}

		fmt.Printf("Rolled back state to height %d and hash %v", height, hash)
		return nil
	},
}

// RollbackState takes the state at the current height n and overwrites it with the state
// at height n - 1. Note state here refers to tendermint state not application state.
// Returns the latest state height and app hash alongside an error if there was one.
func RollbackState(config *cfg.Config) (int64, []byte, error) {
	// use the parsed config to load the block and state store
	blockStore, stateStore, err := loadStateAndBlockStore(config)
	if err != nil {
		return -1, nil, err
	}

	// rollback the last state
	return state.Rollback(blockStore, stateStore)
}

func loadStateAndBlockStore(config *cfg.Config) (*store.BlockStore, state.Store, error) {
	dbType := dbm.BackendType(config.DBBackend)

	// Get BlockStore
	blockStoreDB, err := dbm.NewDB("blockstore", dbType, config.DBDir())
	if err != nil {
		return nil, nil, err
	}
	blockStore := store.NewBlockStore(blockStoreDB)

	// Get StateStore
	stateDB, err := dbm.NewDB("state", dbType, config.DBDir())
	if err != nil {
		return nil, nil, err
	}
	stateStore := state.NewStore(stateDB)

	return blockStore, stateStore, nil
}
