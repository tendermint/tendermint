// Copyright 2016 Tendermint. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package state

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	abci "github.com/tendermint/abci/types"

	crypto "github.com/tendermint/go-crypto"

	cmn "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"
	"github.com/tendermint/tmlibs/log"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/types"
)

// setupTestCase does setup common to all test cases
func setupTestCase(t *testing.T) (func(t *testing.T), dbm.DB, *State) {
	config := cfg.ResetTestRoot("state_")
	stateDB := dbm.NewDB("state", config.DBBackend, config.DBDir())
	state := GetState(stateDB, config.GenesisFile())
	state.SetLogger(log.TestingLogger())

	tearDown := func(t *testing.T) {}

	return tearDown, stateDB, state
}

func TestStateCopy(t *testing.T) {
	tearDown, _, state := setupTestCase(t)
	defer tearDown(t)
	assert := assert.New(t)

	stateCopy := state.Copy()

	assert.True(state.Equals(stateCopy),
		cmn.Fmt("expected state and its copy to be identical. got %v\n expected %v\n", stateCopy, state))
	stateCopy.LastBlockHeight++
	assert.False(state.Equals(stateCopy), cmn.Fmt("expected states to be different. got same %v", state))
}

func TestStateSaveLoad(t *testing.T) {
	tearDown, stateDB, state := setupTestCase(t)
	defer tearDown(t)
	assert := assert.New(t)

	state.LastBlockHeight++
	state.Save()

	loadedState := LoadState(stateDB)
	assert.True(state.Equals(loadedState),
		cmn.Fmt("expected state and its copy to be identical. got %v\n expected %v\n", loadedState, state))
}

func TestABCIResponsesSaveLoad(t *testing.T) {
	tearDown, _, state := setupTestCase(t)
	defer tearDown(t)
	assert := assert.New(t)

	state.LastBlockHeight++

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
	assert.Equal(abciResponses, abciResponses2,
		cmn.Fmt("ABCIResponses don't match: Got %v, Expected %v", abciResponses2, abciResponses))
}

func TestValidatorSimpleSaveLoad(t *testing.T) {
	tearDown, _, state := setupTestCase(t)
	defer tearDown(t)
	assert := assert.New(t)

	// cant load anything for height 0
	v, err := state.LoadValidators(0)
	assert.IsType(ErrNoValSetForHeight{}, err, "expected err at height 0")

	// should be able to load for height 1
	v, err = state.LoadValidators(1)
	assert.Nil(err, "expected no err at height 1")
	assert.Equal(v.Hash(), state.Validators.Hash(), "expected validator hashes to match")

	// increment height, save; should be able to load for next height
	state.LastBlockHeight++
	state.saveValidatorsInfo()
	v, err = state.LoadValidators(state.LastBlockHeight + 1)
	assert.Nil(err, "expected no err")
	assert.Equal(v.Hash(), state.Validators.Hash(), "expected validator hashes to match")

	// increment height, save; should be able to load for next height
	state.LastBlockHeight += 10
	state.saveValidatorsInfo()
	v, err = state.LoadValidators(state.LastBlockHeight + 1)
	assert.Nil(err, "expected no err")
	assert.Equal(v.Hash(), state.Validators.Hash(), "expected validator hashes to match")

	// should be able to load for next next height
	_, err = state.LoadValidators(state.LastBlockHeight + 2)
	assert.IsType(ErrNoValSetForHeight{}, err, "expected err at unknown height")
}

func TestValidatorChangesSaveLoad(t *testing.T) {
	tearDown, _, state := setupTestCase(t)
	defer tearDown(t)
	assert := assert.New(t)

	// change vals at these heights
	changeHeights := []int{1, 2, 4, 5, 10, 15, 16, 17, 20}
	N := len(changeHeights)

	// each valset is just one validator.
	// create list of them
	pubkeys := make([]crypto.PubKey, N+1)
	pubkeys[0] = state.GenesisDoc.Validators[0].PubKey
	for i := 1; i < N+1; i++ {
		pubkeys[i] = crypto.GenPrivKeyEd25519().PubKey()
	}

	// build the validator history by running SetBlockAndValidators
	// with the right validator set for each height
	highestHeight := changeHeights[N-1] + 5
	changeIndex := 0
	pubkey := pubkeys[changeIndex]
	for i := 1; i < highestHeight; i++ {
		// when we get to a change height,
		// use the next pubkey
		if changeIndex < len(changeHeights) && i == changeHeights[changeIndex] {
			changeIndex++
			pubkey = pubkeys[changeIndex]
		}
		header, parts, responses := makeHeaderPartsResponses(state, i, pubkey)
		state.SetBlockAndValidators(header, parts, responses)
		state.saveValidatorsInfo()
	}

	// make all the test cases by using the same validator until after the change
	testCases := make([]valChangeTestCase, highestHeight)
	changeIndex = 0
	pubkey = pubkeys[changeIndex]
	for i := 1; i < highestHeight+1; i++ {
		// we we get to the height after a change height
		// use the next pubkey (note our counter starts at 0 this time)
		if changeIndex < len(changeHeights) && i == changeHeights[changeIndex]+1 {
			changeIndex++
			pubkey = pubkeys[changeIndex]
		}
		testCases[i-1] = valChangeTestCase{i, pubkey}
	}

	for _, testCase := range testCases {
		v, err := state.LoadValidators(testCase.height)
		assert.Nil(err, fmt.Sprintf("expected no err at height %d", testCase.height))
		assert.Equal(v.Size(), 1, "validator set size is greater than 1: %d", v.Size())
		addr, _ := v.GetByIndex(0)

		assert.Equal(addr, testCase.vals.Address(), fmt.Sprintf("unexpected pubkey at height %d", testCase.height))
	}
}

func makeHeaderPartsResponses(state *State, height int,
	pubkey crypto.PubKey) (*types.Header, types.PartSetHeader, *ABCIResponses) {

	block := makeBlock(height, state)
	_, val := state.Validators.GetByIndex(0)
	abciResponses := &ABCIResponses{
		Height: height,
	}

	// if the pubkey is new, remove the old and add the new
	if !bytes.Equal(pubkey.Bytes(), val.PubKey.Bytes()) {
		abciResponses.EndBlock = abci.ResponseEndBlock{
			Diffs: []*abci.Validator{
				{val.PubKey.Bytes(), 0},
				{pubkey.Bytes(), 10},
			},
		}
	}

	return block.Header, types.PartSetHeader{}, abciResponses
}

type valChangeTestCase struct {
	height int
	vals   crypto.PubKey
}
