package state

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	cmn "github.com/tendermint/tendermint/libs/common"
	dbm "github.com/tendermint/tendermint/libs/db"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/types"
)

// setupTestCase does setup common to all test cases.
func setupTestCase(t *testing.T) (func(t *testing.T), dbm.DB, State) {
	config := cfg.ResetTestRoot("state_")
	dbType := dbm.DBBackendType(config.DBBackend)
	stateDB := dbm.NewDB("state", dbType, config.DBDir())
	state, err := LoadStateFromDBOrGenesisFile(stateDB, config.GenesisFile())
	assert.NoError(t, err, "expected no error on LoadStateFromDBOrGenesisFile")

	tearDown := func(t *testing.T) {}

	return tearDown, stateDB, state
}

// TestStateCopy tests the correct copying behaviour of State.
func TestStateCopy(t *testing.T) {
	tearDown, _, state := setupTestCase(t)
	defer tearDown(t)
	// nolint: vetshadow
	assert := assert.New(t)

	stateCopy := state.Copy()

	assert.True(state.Equals(stateCopy),
		fmt.Sprintf("expected state and its copy to be identical.\ngot: %v\nexpected: %v\n",
			stateCopy, state))

	stateCopy.LastBlockHeight++
	assert.False(state.Equals(stateCopy), fmt.Sprintf(`expected states to be different. got same
        %v`, state))
}

//TestMakeGenesisStateNilValidators tests state's consistency when genesis file's validators field is nil.
func TestMakeGenesisStateNilValidators(t *testing.T) {
	doc := types.GenesisDoc{
		ChainID:    "dummy",
		Validators: nil,
	}
	require.Nil(t, doc.ValidateAndComplete())
	state, err := MakeGenesisState(&doc)
	require.Nil(t, err)
	require.Equal(t, 0, len(state.Validators.Validators))
	require.Equal(t, 0, len(state.NextValidators.Validators))
}

// TestStateSaveLoad tests saving and loading State from a db.
func TestStateSaveLoad(t *testing.T) {
	tearDown, stateDB, state := setupTestCase(t)
	defer tearDown(t)
	// nolint: vetshadow
	assert := assert.New(t)

	state.LastBlockHeight++
	SaveState(stateDB, state)

	loadedState := LoadState(stateDB)
	assert.True(state.Equals(loadedState),
		fmt.Sprintf("expected state and its copy to be identical.\ngot: %v\nexpected: %v\n",
			loadedState, state))
}

// TestABCIResponsesSaveLoad tests saving and loading ABCIResponses.
func TestABCIResponsesSaveLoad1(t *testing.T) {
	tearDown, stateDB, state := setupTestCase(t)
	defer tearDown(t)
	// nolint: vetshadow
	assert := assert.New(t)

	state.LastBlockHeight++

	// Build mock responses.
	block := makeBlock(state, 2)
	abciResponses := NewABCIResponses(block)
	abciResponses.DeliverTx[0] = &abci.ResponseDeliverTx{Data: []byte("foo"), Tags: nil}
	abciResponses.DeliverTx[1] = &abci.ResponseDeliverTx{Data: []byte("bar"), Log: "ok", Tags: nil}
	abciResponses.EndBlock = &abci.ResponseEndBlock{ValidatorUpdates: []abci.ValidatorUpdate{
		types.TM2PB.NewValidatorUpdate(ed25519.GenPrivKey().PubKey(), 10),
	}}

	saveABCIResponses(stateDB, block.Height, abciResponses)
	loadedABCIResponses, err := LoadABCIResponses(stateDB, block.Height)
	assert.Nil(err)
	assert.Equal(abciResponses, loadedABCIResponses,
		fmt.Sprintf("ABCIResponses don't match:\ngot:       %v\nexpected: %v\n",
			loadedABCIResponses, abciResponses))
}

// TestResultsSaveLoad tests saving and loading ABCI results.
func TestABCIResponsesSaveLoad2(t *testing.T) {
	tearDown, stateDB, _ := setupTestCase(t)
	defer tearDown(t)
	// nolint: vetshadow
	assert := assert.New(t)

	cases := [...]struct {
		// Height is implied to equal index+2,
		// as block 1 is created from genesis.
		added    []*abci.ResponseDeliverTx
		expected types.ABCIResults
	}{
		0: {
			nil,
			nil,
		},
		1: {
			[]*abci.ResponseDeliverTx{
				{Code: 32, Data: []byte("Hello"), Log: "Huh?"},
			},
			types.ABCIResults{
				{32, []byte("Hello")},
			}},
		2: {
			[]*abci.ResponseDeliverTx{
				{Code: 383},
				{Data: []byte("Gotcha!"),
					Tags: []cmn.KVPair{
						{Key: []byte("a"), Value: []byte("1")},
						{Key: []byte("build"), Value: []byte("stuff")},
					}},
			},
			types.ABCIResults{
				{383, nil},
				{0, []byte("Gotcha!")},
			}},
		3: {
			nil,
			nil,
		},
	}

	// Query all before, this should return error.
	for i := range cases {
		h := int64(i + 1)
		res, err := LoadABCIResponses(stateDB, h)
		assert.Error(err, "%d: %#v", i, res)
	}

	// Add all cases.
	for i, tc := range cases {
		h := int64(i + 1) // last block height, one below what we save
		responses := &ABCIResponses{
			DeliverTx: tc.added,
			EndBlock:  &abci.ResponseEndBlock{},
		}
		saveABCIResponses(stateDB, h, responses)
	}

	// Query all before, should return expected value.
	for i, tc := range cases {
		h := int64(i + 1)
		res, err := LoadABCIResponses(stateDB, h)
		assert.NoError(err, "%d", i)
		assert.Equal(tc.expected.Hash(), res.ResultsHash(), "%d", i)
	}
}

// TestValidatorSimpleSaveLoad tests saving and loading validators.
func TestValidatorSimpleSaveLoad(t *testing.T) {
	tearDown, stateDB, state := setupTestCase(t)
	defer tearDown(t)
	// nolint: vetshadow
	assert := assert.New(t)

	// Can't load anything for height 0.
	v, err := LoadValidators(stateDB, 0)
	assert.IsType(ErrNoValSetForHeight{}, err, "expected err at height 0")

	// Should be able to load for height 1.
	v, err = LoadValidators(stateDB, 1)
	assert.Nil(err, "expected no err at height 1")
	assert.Equal(v.Hash(), state.Validators.Hash(), "expected validator hashes to match")

	// Should be able to load for height 2.
	v, err = LoadValidators(stateDB, 2)
	assert.Nil(err, "expected no err at height 2")
	assert.Equal(v.Hash(), state.NextValidators.Hash(), "expected validator hashes to match")

	// Increment height, save; should be able to load for next & next next height.
	state.LastBlockHeight++
	nextHeight := state.LastBlockHeight + 1
	saveValidatorsInfo(stateDB, nextHeight+1, state.LastHeightValidatorsChanged, state.NextValidators)
	vp0, err := LoadValidators(stateDB, nextHeight+0)
	assert.Nil(err, "expected no err")
	vp1, err := LoadValidators(stateDB, nextHeight+1)
	assert.Nil(err, "expected no err")
	assert.Equal(vp0.Hash(), state.Validators.Hash(), "expected validator hashes to match")
	assert.Equal(vp1.Hash(), state.NextValidators.Hash(), "expected next validator hashes to match")
}

// TestValidatorChangesSaveLoad tests saving and loading a validator set with changes.
func TestOneValidatorChangesSaveLoad(t *testing.T) {
	tearDown, stateDB, state := setupTestCase(t)
	defer tearDown(t)

	// Change vals at these heights.
	changeHeights := []int64{1, 2, 4, 5, 10, 15, 16, 17, 20}
	N := len(changeHeights)

	// Build the validator history by running updateState
	// with the right validator set for each height.
	highestHeight := changeHeights[N-1] + 5
	changeIndex := 0
	_, val := state.Validators.GetByIndex(0)
	power := val.VotingPower
	var err error
	var validatorUpdates []*types.Validator
	for i := int64(1); i < highestHeight; i++ {
		// When we get to a change height, use the next pubkey.
		if changeIndex < len(changeHeights) && i == changeHeights[changeIndex] {
			changeIndex++
			power++
		}
		header, blockID, responses := makeHeaderPartsResponsesValPowerChange(state, i, power)
		validatorUpdates, err = types.PB2TM.ValidatorUpdates(responses.EndBlock.ValidatorUpdates)
		require.NoError(t, err)
		state, err = updateState(state, blockID, &header, responses, validatorUpdates)
		require.NoError(t, err)
		nextHeight := state.LastBlockHeight + 1
		saveValidatorsInfo(stateDB, nextHeight+1, state.LastHeightValidatorsChanged, state.NextValidators)
	}

	// On each height change, increment the power by one.
	testCases := make([]int64, highestHeight)
	changeIndex = 0
	power = val.VotingPower
	for i := int64(1); i < highestHeight+1; i++ {
		// We get to the height after a change height use the next pubkey (note
		// our counter starts at 0 this time).
		if changeIndex < len(changeHeights) && i == changeHeights[changeIndex]+1 {
			changeIndex++
			power++
		}
		testCases[i-1] = power
	}

	for i, power := range testCases {
		v, err := LoadValidators(stateDB, int64(i+1+1)) // +1 because vset changes delayed by 1 block.
		assert.Nil(t, err, fmt.Sprintf("expected no err at height %d", i))
		assert.Equal(t, v.Size(), 1, "validator set size is greater than 1: %d", v.Size())
		_, val := v.GetByIndex(0)

		assert.Equal(t, val.VotingPower, power, fmt.Sprintf(`unexpected powerat
                height %d`, i))
	}
}

func TestStoreLoadValidatorsIncrementsProposerPriority(t *testing.T) {
	const valSetSize = 2
	tearDown, stateDB, state := setupTestCase(t)
	state.Validators = genValSet(valSetSize)
	state.NextValidators = state.Validators.CopyIncrementProposerPriority(1)
	SaveState(stateDB, state)
	defer tearDown(t)

	nextHeight := state.LastBlockHeight + 1

	v0, err := LoadValidators(stateDB, nextHeight)
	assert.Nil(t, err)
	acc0 := v0.Validators[0].ProposerPriority

	v1, err := LoadValidators(stateDB, nextHeight+1)
	assert.Nil(t, err)
	acc1 := v1.Validators[0].ProposerPriority

	assert.NotEqual(t, acc1, acc0, "expected ProposerPriority value to change between heights")
}

// TestValidatorChangesSaveLoad tests saving and loading a validator set with
// changes.
func TestManyValidatorChangesSaveLoad(t *testing.T) {
	const valSetSize = 7
	tearDown, stateDB, state := setupTestCase(t)
	require.Equal(t, int64(0), state.LastBlockHeight)
	state.Validators = genValSet(valSetSize)
	state.NextValidators = state.Validators.CopyIncrementProposerPriority(1)
	SaveState(stateDB, state)
	defer tearDown(t)

	_, valOld := state.Validators.GetByIndex(0)
	var pubkeyOld = valOld.PubKey
	pubkey := ed25519.GenPrivKey().PubKey()
	const height = 1

	// Swap the first validator with a new one (validator set size stays the same).
	header, blockID, responses := makeHeaderPartsResponsesValPubKeyChange(state, height, pubkey)

	// Save state etc.
	var err error
	var validatorUpdates []*types.Validator
	validatorUpdates, err = types.PB2TM.ValidatorUpdates(responses.EndBlock.ValidatorUpdates)
	require.NoError(t, err)
	state, err = updateState(state, blockID, &header, responses, validatorUpdates)
	require.Nil(t, err)
	nextHeight := state.LastBlockHeight + 1
	saveValidatorsInfo(stateDB, nextHeight+1, state.LastHeightValidatorsChanged, state.NextValidators)

	// Load nextheight, it should be the oldpubkey.
	v0, err := LoadValidators(stateDB, nextHeight)
	assert.Nil(t, err)
	assert.Equal(t, valSetSize, v0.Size())
	index, val := v0.GetByAddress(pubkeyOld.Address())
	assert.NotNil(t, val)
	if index < 0 {
		t.Fatal("expected to find old validator")
	}

	// Load nextheight+1, it should be the new pubkey.
	v1, err := LoadValidators(stateDB, nextHeight+1)
	assert.Nil(t, err)
	assert.Equal(t, valSetSize, v1.Size())
	index, val = v1.GetByAddress(pubkey.Address())
	assert.NotNil(t, val)
	if index < 0 {
		t.Fatal("expected to find newly added validator")
	}
}

func genValSet(size int) *types.ValidatorSet {
	vals := make([]*types.Validator, size)
	for i := 0; i < size; i++ {
		vals[i] = types.NewValidator(ed25519.GenPrivKey().PubKey(), 10)
	}
	return types.NewValidatorSet(vals)
}

func TestStateMakeBlock(t *testing.T) {
	tearDown, _, state := setupTestCase(t)
	defer tearDown(t)

	proposerAddress := state.Validators.GetProposer().Address
	stateVersion := state.Version.Consensus
	block := makeBlock(state, 2)

	// test we set some fields
	assert.Equal(t, stateVersion, block.Version)
	assert.Equal(t, proposerAddress, block.ProposerAddress)
}

// TestConsensusParamsChangesSaveLoad tests saving and loading consensus params
// with changes.
func TestConsensusParamsChangesSaveLoad(t *testing.T) {
	tearDown, stateDB, state := setupTestCase(t)
	defer tearDown(t)

	// Change vals at these heights.
	changeHeights := []int64{1, 2, 4, 5, 10, 15, 16, 17, 20}
	N := len(changeHeights)

	// Each valset is just one validator.
	// create list of them.
	params := make([]types.ConsensusParams, N+1)
	params[0] = state.ConsensusParams
	for i := 1; i < N+1; i++ {
		params[i] = *types.DefaultConsensusParams()
		params[i].BlockSize.MaxBytes += int64(i)
	}

	// Build the params history by running updateState
	// with the right params set for each height.
	highestHeight := changeHeights[N-1] + 5
	changeIndex := 0
	cp := params[changeIndex]
	var err error
	var validatorUpdates []*types.Validator
	for i := int64(1); i < highestHeight; i++ {
		// When we get to a change height, use the next params.
		if changeIndex < len(changeHeights) && i == changeHeights[changeIndex] {
			changeIndex++
			cp = params[changeIndex]
		}
		header, blockID, responses := makeHeaderPartsResponsesParams(state, i, cp)
		validatorUpdates, err = types.PB2TM.ValidatorUpdates(responses.EndBlock.ValidatorUpdates)
		require.NoError(t, err)
		state, err = updateState(state, blockID, &header, responses, validatorUpdates)

		require.Nil(t, err)
		nextHeight := state.LastBlockHeight + 1
		saveConsensusParamsInfo(stateDB, nextHeight, state.LastHeightConsensusParamsChanged, state.ConsensusParams)
	}

	// Make all the test cases by using the same params until after the change.
	testCases := make([]paramsChangeTestCase, highestHeight)
	changeIndex = 0
	cp = params[changeIndex]
	for i := int64(1); i < highestHeight+1; i++ {
		// We get to the height after a change height use the next pubkey (note
		// our counter starts at 0 this time).
		if changeIndex < len(changeHeights) && i == changeHeights[changeIndex]+1 {
			changeIndex++
			cp = params[changeIndex]
		}
		testCases[i-1] = paramsChangeTestCase{i, cp}
	}

	for _, testCase := range testCases {
		p, err := LoadConsensusParams(stateDB, testCase.height)
		assert.Nil(t, err, fmt.Sprintf("expected no err at height %d", testCase.height))
		assert.Equal(t, testCase.params, p, fmt.Sprintf(`unexpected consensus params at
                height %d`, testCase.height))
	}
}

func makeParams(blockBytes, blockGas, evidenceAge int64) types.ConsensusParams {
	return types.ConsensusParams{
		BlockSize: types.BlockSizeParams{
			MaxBytes: blockBytes,
			MaxGas:   blockGas,
		},
		Evidence: types.EvidenceParams{
			MaxAge: evidenceAge,
		},
	}
}

func pk() []byte {
	return ed25519.GenPrivKey().PubKey().Bytes()
}

func TestApplyUpdates(t *testing.T) {
	initParams := makeParams(1, 2, 3)

	cases := [...]struct {
		init     types.ConsensusParams
		updates  abci.ConsensusParams
		expected types.ConsensusParams
	}{
		0: {initParams, abci.ConsensusParams{}, initParams},
		1: {initParams, abci.ConsensusParams{}, initParams},
		2: {initParams,
			abci.ConsensusParams{
				BlockSize: &abci.BlockSizeParams{
					MaxBytes: 44,
					MaxGas:   55,
				},
			},
			makeParams(44, 55, 3)},
		3: {initParams,
			abci.ConsensusParams{
				Evidence: &abci.EvidenceParams{
					MaxAge: 66,
				},
			},
			makeParams(1, 2, 66)},
	}

	for i, tc := range cases {
		res := tc.init.Update(&(tc.updates))
		assert.Equal(t, tc.expected, res, "case %d", i)
	}
}

func makeHeaderPartsResponsesValPubKeyChange(state State, height int64,
	pubkey crypto.PubKey) (types.Header, types.BlockID, *ABCIResponses) {

	block := makeBlock(state, state.LastBlockHeight+1)
	abciResponses := &ABCIResponses{
		EndBlock: &abci.ResponseEndBlock{ValidatorUpdates: nil},
	}

	// If the pubkey is new, remove the old and add the new.
	_, val := state.NextValidators.GetByIndex(0)
	if !bytes.Equal(pubkey.Bytes(), val.PubKey.Bytes()) {
		abciResponses.EndBlock = &abci.ResponseEndBlock{
			ValidatorUpdates: []abci.ValidatorUpdate{
				types.TM2PB.NewValidatorUpdate(val.PubKey, 0),
				types.TM2PB.NewValidatorUpdate(pubkey, 10),
			},
		}
	}

	return block.Header, types.BlockID{block.Hash(), types.PartSetHeader{}}, abciResponses
}

func makeHeaderPartsResponsesValPowerChange(state State, height int64,
	power int64) (types.Header, types.BlockID, *ABCIResponses) {

	block := makeBlock(state, state.LastBlockHeight+1)
	abciResponses := &ABCIResponses{
		EndBlock: &abci.ResponseEndBlock{ValidatorUpdates: nil},
	}

	// If the pubkey is new, remove the old and add the new.
	_, val := state.NextValidators.GetByIndex(0)
	if val.VotingPower != power {
		abciResponses.EndBlock = &abci.ResponseEndBlock{
			ValidatorUpdates: []abci.ValidatorUpdate{
				types.TM2PB.NewValidatorUpdate(val.PubKey, power),
			},
		}
	}

	return block.Header, types.BlockID{block.Hash(), types.PartSetHeader{}}, abciResponses
}

func makeHeaderPartsResponsesParams(state State, height int64,
	params types.ConsensusParams) (types.Header, types.BlockID, *ABCIResponses) {

	block := makeBlock(state, state.LastBlockHeight+1)
	abciResponses := &ABCIResponses{
		EndBlock: &abci.ResponseEndBlock{ConsensusParamUpdates: types.TM2PB.ConsensusParams(&params)},
	}
	return block.Header, types.BlockID{block.Hash(), types.PartSetHeader{}}, abciResponses
}

type paramsChangeTestCase struct {
	height int64
	params types.ConsensusParams
}
