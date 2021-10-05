package cli

import (
	dbm "github.com/tendermint/tm-db"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/store"
)

// RollbackState takes the state at the current height n and overwrites it with the state
// at height n - 1. Note state here refers to tendermint state not application state.
// Returns the latest state height and app hash alongside an error if there was one.
func RollbackState(config *cfg.Config) (int64, []byte, error) {
	// use the parsed config to load the block and state store
	dbType := dbm.BackendType(config.DBBackend)

	// Get BlockStore
	blockStoreDB, err := dbm.NewDB("blockstore", dbType, config.DBDir())
	if err != nil {
		return -1, nil, err
	}
	blockStore := store.NewBlockStore(blockStoreDB)

	// Get StateStore
	stateDB, err := dbm.NewDB("state", dbType, config.DBDir())
	if err != nil {
		return -1, nil, err
	}
	stateStore := state.NewStore(stateDB)

	// rollback the last state
	return state.Rollback(blockStore, stateStore)
}
