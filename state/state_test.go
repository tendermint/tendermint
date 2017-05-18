package state

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	abci "github.com/tendermint/abci/types"
	crypto "github.com/tendermint/go-crypto"
	cfg "github.com/tendermint/tendermint/config"
	dbm "github.com/tendermint/tmlibs/db"
	"github.com/tendermint/tmlibs/log"
)

func TestStateCopyEquals(t *testing.T) {
	config := cfg.ResetTestRoot("state_")

	// Get State db
	stateDB := dbm.NewDB("state", config.DBBackend, config.DBDir())
	state := GetState(stateDB, config.GenesisFile())
	state.SetLogger(log.TestingLogger())

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
	config := cfg.ResetTestRoot("state_")
	// Get State db
	stateDB := dbm.NewDB("state", config.DBBackend, config.DBDir())
	state := GetState(stateDB, config.GenesisFile())
	state.SetLogger(log.TestingLogger())

	state.LastBlockHeight += 1
	state.Save()

	loadedState := LoadState(stateDB)
	if !state.Equals(loadedState) {
		t.Fatal("expected state and its copy to be identical. got %v\n expected %v\n", loadedState, state)
	}
}

func TestABCIResponsesSaveLoad(t *testing.T) {
	assert := assert.New(t)

	config := cfg.ResetTestRoot("state_")
	stateDB := dbm.NewDB("state", config.DBBackend, config.DBDir())
	state := GetState(stateDB, config.GenesisFile())
	state.SetLogger(log.TestingLogger())

	state.LastBlockHeight += 1

	// build mock responses
	block := makeBlock(2, state)
	abciResponses := NewABCIResponses(block)
	abciResponses.DeliverTx[0] = &abci.ResponseDeliverTx{Data: []byte("foo")}
	abciResponses.DeliverTx[1] = &abci.ResponseDeliverTx{Data: []byte("bar"), Log: "ok"}
	abciResponses.EndBlock = abci.ResponseEndBlock{Diffs: []*abci.Validator{
		{
			PubKey: crypto.GenPrivKeyEd25519().PubKey().Bytes(),
			Power:  10,
		},
	}}
	abciResponses.txs = nil

	state.SaveABCIResponses(abciResponses)
	abciResponses2 := state.LoadABCIResponses()
	assert.Equal(abciResponses, abciResponses2, fmt.Sprintf("ABCIResponses don't match: Got %v, Expected %v", abciResponses2, abciResponses))
}
