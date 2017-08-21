package state

import (
	"testing"

	"github.com/stretchr/testify/assert"

	abci "github.com/tendermint/abci/types"
	crypto "github.com/tendermint/go-crypto"
	cmn "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"
	"github.com/tendermint/tmlibs/log"

	cfg "github.com/tendermint/tendermint/config"
)

func TestStateCopyEquals(t *testing.T) {
	assert := assert.New(t)
	config := cfg.ResetTestRoot("state_")

	// Get State db
	stateDB := dbm.NewDB("state", config.DBBackend, config.DBDir())
	state := GetState(stateDB, config.GenesisFile())
	state.SetLogger(log.TestingLogger())

	stateCopy := state.Copy()

	assert.True(state.Equals(stateCopy), cmn.Fmt("expected state and its copy to be identical. got %v\n expected %v\n", stateCopy, state))
	stateCopy.LastBlockHeight += 1
	assert.False(state.Equals(stateCopy), cmn.Fmt("expected states to be different. got same %v", state))
}

func TestStateSaveLoad(t *testing.T) {
	assert := assert.New(t)
	config := cfg.ResetTestRoot("state_")
	// Get State db
	stateDB := dbm.NewDB("state", config.DBBackend, config.DBDir())
	state := GetState(stateDB, config.GenesisFile())
	state.SetLogger(log.TestingLogger())

	state.LastBlockHeight += 1
	state.Save()

	loadedState := LoadState(stateDB)
	assert.True(state.Equals(loadedState), cmn.Fmt("expected state and its copy to be identical. got %v\n expected %v\n", loadedState, state))
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
	assert.Equal(abciResponses, abciResponses2, cmn.Fmt("ABCIResponses don't match: Got %v, Expected %v", abciResponses2, abciResponses))
}

func TestValidatorsSaveLoad(t *testing.T) {
	assert := assert.New(t)
	config := cfg.ResetTestRoot("state_")
	// Get State db
	stateDB := dbm.NewDB("state", config.DBBackend, config.DBDir())
	state := GetState(stateDB, config.GenesisFile())
	state.SetLogger(log.TestingLogger())

	// cant load anything for height 0
	v, err := state.LoadValidators(0)
	assert.NotNil(err, "expected err at height 0")

	// should be able to load for height 1
	v, err = state.LoadValidators(1)
	assert.Nil(err, "expected no err at height 1")
	assert.Equal(v.Hash(), state.Validators.Hash(), "expected validator hashes to match")

	// increment height, save; should be able to load for next height
	state.LastBlockHeight += 1
	state.SaveValidators()
	v, err = state.LoadValidators(state.LastBlockHeight + 1)
	assert.Nil(err, "expected no err")
	assert.Equal(v.Hash(), state.Validators.Hash(), "expected validator hashes to match")

	// increment height, save; should be able to load for next height
	state.LastBlockHeight += 10
	state.SaveValidators()
	v, err = state.LoadValidators(state.LastBlockHeight + 1)
	assert.Nil(err, "expected no err")
	assert.Equal(v.Hash(), state.Validators.Hash(), "expected validator hashes to match")

	// should be able to load for next next height
	_, err = state.LoadValidators(state.LastBlockHeight + 2)
	assert.NotNil(err, "expected err")
}
