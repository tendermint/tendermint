package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/types"

	abci "github.com/tendermint/abci/types"
	crypto "github.com/tendermint/go-crypto"

	"github.com/tendermint/go-wire/data"
	cmn "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"
	"github.com/tendermint/tmlibs/log"
)

// setupTestCase does setup common to all test cases
func setupTestCase(t *testing.T) (func(t *testing.T), dbm.DB, *State) {
	config := cfg.ResetTestRoot("state_")
	stateDB := dbm.NewDB("state", config.DBBackend, config.DBDir())
	state, err := GetState(stateDB, config.GenesisFile())
	assert.NoError(t, err, "expected no error on GetState")
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
	genDoc, err := state.GenesisDoc()
	assert.Nil(err, "want successful genDoc retrieval")
	pubkeys[0] = genDoc.Validators[0].PubKey
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

var (
	aPrivKey       = crypto.GenPrivKeyEd25519()
	_1stGenesisDoc = &types.GenesisDoc{
		GenesisTime: time.Now().Add(-60 * time.Minute).Round(time.Second),
		ChainID:     "tendermint_state_test",
		AppHash:     data.Bytes{},
		ConsensusParams: &types.ConsensusParams{
			BlockSizeParams: types.BlockSizeParams{
				MaxBytes: 100,
				MaxGas:   2000,
				MaxTxs:   56,
			},

			BlockGossipParams: types.BlockGossipParams{
				BlockPartSizeBytes: 65336,
			},
		},
		Validators: []types.GenesisValidator{
			{PubKey: aPrivKey.PubKey(), Power: 10000, Name: "TendermintFoo"},
		},
	}
	_2ndGenesisDoc = func() *types.GenesisDoc {
		copy := new(types.GenesisDoc)
		*copy = *_1stGenesisDoc
		copy.GenesisTime = time.Now().Round(time.Second)
		return copy
	}()
)

// See Issue https://github.com/tendermint/tendermint/issues/671.
func TestGenesisDocAndChainIDAccessorsAndSetter(t *testing.T) {
	tearDown, dbm, state := setupTestCase(t)
	defer tearDown(t)
	require := require.New(t)

	// Fire up the initial genesisDoc
	_, err := state.GenesisDoc()
	require.Nil(err, "expecting no error on first load of genesisDoc")

	// By contract, state doesn't expose the dbm, however we need to change
	// it to test out that the respective chainID and genesisDoc will be changed
	state.cachedGenesisDoc = nil
	_1stBlob, err := json.Marshal(_1stGenesisDoc)
	require.Nil(err, "expecting no error serializing _1stGenesisDoc")
	dbm.Set(genesisDBKey, _1stBlob)

	retrGenDoc, err := state.GenesisDoc()
	require.Nil(err, "unexpected error")
	require.Equal(retrGenDoc, _1stGenesisDoc, "expecting the newly set-in-Db genesis doc")
	chainID, err := state.ChainID()
	require.Nil(err, "unexpected error")
	require.Equal(chainID, _1stGenesisDoc.ChainID, "expecting the chainIDs to be equal")

	require.NotNil(state.cachedGenesisDoc, "after retrieval expecting a non-nil cachedGenesisDoc")
	// Save should not discard the previous cachedGenesisDoc
	// which was the point of filing https://github.com/tendermint/tendermint/issues/671.
	state.Save()
	require.NotNil(state.cachedGenesisDoc, "even after flush with .Save(), expecting a non-nil cachedGenesisDoc")

	// Now change up the data but ensure
	// that a Save discards the old validator
	_2ndBlob, err := json.Marshal(_2ndGenesisDoc)
	require.Nil(err, "unexpected error")
	dbm.Set(genesisDBKey, _2ndBlob)

	refreshGenDoc, err := state.GenesisDoc()
	require.Nil(err, "unexpected error")
	require.Equal(refreshGenDoc, _1stGenesisDoc, "despite setting the new genesisDoc in DB, it shouldn't affect the one in state")
	state.SetGenesisDoc(_2ndGenesisDoc)

	refreshGenDoc, err = state.GenesisDoc()
	require.Nil(err, "unexpected error")
	require.Equal(refreshGenDoc, _2ndGenesisDoc, "expecting the newly set-in-Db genesis doc to have been reloaded after a .Save()")

	// Test that .Save() should never overwrite the currently set content in the DB
	dbm.Set(genesisDBKey, _1stBlob)
	state.Save()
	require.Equal(dbm.Get(genesisDBKey), _1stBlob, ".Save() should NEVER serialize back the current genesisDoc")

	// ChainID on a nil cachedGenesisDoc should do a DB fetch
	state.SetGenesisDoc(nil)
	dbm.Set(genesisDBKey, _2ndBlob)
	chainID, err = state.ChainID()
	require.Nil(err, "unexpected error")
	require.Equal(chainID, _2ndGenesisDoc.ChainID, "expecting the 2ndGenesisDoc.ChainID")

	// Now test what happens if we cannot find the genesis doc in the DB
	// Checkpoint and discard
	state.Save()
	dbm.Set(genesisDBKey, nil)
	state.SetGenesisDoc(nil)
	gotGenDoc, err := state.GenesisDoc()
	require.NotNil(err, "could not parse out a genesisDoc from the DB")
	require.Nil(gotGenDoc, "since we couldn't parse the genesis doc, expecting a nil genesis doc")

	dbm.Set(genesisDBKey, []byte(`{}`))
	gotGenDoc, err = state.GenesisDoc()
	require.NotNil(err, "despite {}, that's not a valid serialization for a genesisDoc")
	require.Nil(gotGenDoc, "since we couldn't parse the genesis doc, expecting a nil genesis doc")
}
