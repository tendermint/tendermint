package state

import (
	"testing"

	dbm "github.com/tendermint/go-db"
	"github.com/tendermint/tendermint/config/tendermint_test"
)

func TestStateCopyEquals(t *testing.T) {
	config := tendermint_test.ResetConfig("state_")
	// Get State db
	stateDB := dbm.NewDB("state", config.GetString("db_backend"), config.GetString("db_dir"))
	state := GetState(config, stateDB)

	stateCopy := state.Copy()

	if !state.Equals(stateCopy) {
		t.Fatal("expected state and its copy to be identical. got %v\n expected %v\n", stateCopy, state)
	}

	stateCopy.LastBlockHeight += 1

	if state.Equals(stateCopy) {
		t.Fatal("expected states to be different. got same %v", state)
	}
}

func TestStateSaveLoad(t *testing.T) {
	config := tendermint_test.ResetConfig("state_")
	// Get State db
	stateDB := dbm.NewDB("state", config.GetString("db_backend"), config.GetString("db_dir"))
	state := GetState(config, stateDB)

	state.LastBlockHeight += 1
	state.Save()

	loadedState := LoadState(stateDB)
	if !state.Equals(loadedState) {
		t.Fatal("expected state and its copy to be identical. got %v\n expected %v\n", loadedState, state)
	}
}
