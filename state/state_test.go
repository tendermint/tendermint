package state_test

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/ed25519"
	cmn "github.com/tendermint/tendermint/libs/common"
	sm "github.com/tendermint/tendermint/state"
	dbm "github.com/tendermint/tm-db"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/types"
)

// setupTestCase does setup common to all test cases.
func setupTestCase(t *testing.T) (func(t *testing.T), dbm.DB, sm.State) {
	config := cfg.ResetTestRoot("state_")
	dbType := dbm.DBBackendType(config.DBBackend)
	stateDB := dbm.NewDB("state", dbType, config.DBDir())
	state, err := sm.LoadStateFromDBOrGenesisFile(stateDB, config.GenesisFile())
	assert.NoError(t, err, "expected no error on LoadStateFromDBOrGenesisFile")

	tearDown := func(t *testing.T) { os.RemoveAll(config.RootDir) }

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
	state, err := sm.MakeGenesisState(&doc)
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
	sm.SaveState(stateDB, state)

	loadedState := sm.LoadState(stateDB)
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
	abciResponses := sm.NewABCIResponses(block)
	abciResponses.DeliverTx[0] = &abci.ResponseDeliverTx{Data: []byte("foo"), Events: nil}
	abciResponses.DeliverTx[1] = &abci.ResponseDeliverTx{Data: []byte("bar"), Log: "ok", Events: nil}
	abciResponses.EndBlock = &abci.ResponseEndBlock{ValidatorUpdates: []abci.ValidatorUpdate{
		types.TM2PB.NewValidatorUpdate(ed25519.GenPrivKey().PubKey(), 10),
	}}

	sm.SaveABCIResponses(stateDB, block.Height, abciResponses)
	loadedABCIResponses, err := sm.LoadABCIResponses(stateDB, block.Height)
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
				{Code: 32, Data: []byte("Hello")},
			}},
		2: {
			[]*abci.ResponseDeliverTx{
				{Code: 383},
				{
					Data: []byte("Gotcha!"),
					Events: []abci.Event{
						{Type: "type1", Attributes: []cmn.KVPair{{Key: []byte("a"), Value: []byte("1")}}},
						{Type: "type2", Attributes: []cmn.KVPair{{Key: []byte("build"), Value: []byte("stuff")}}},
					},
				},
			},
			types.ABCIResults{
				{Code: 383, Data: nil},
				{Code: 0, Data: []byte("Gotcha!")},
			}},
		3: {
			nil,
			nil,
		},
	}

	// Query all before, this should return error.
	for i := range cases {
		h := int64(i + 1)
		res, err := sm.LoadABCIResponses(stateDB, h)
		assert.Error(err, "%d: %#v", i, res)
	}

	// Add all cases.
	for i, tc := range cases {
		h := int64(i + 1) // last block height, one below what we save
		responses := &sm.ABCIResponses{
			DeliverTx: tc.added,
			EndBlock:  &abci.ResponseEndBlock{},
		}
		sm.SaveABCIResponses(stateDB, h, responses)
	}

	// Query all before, should return expected value.
	for i, tc := range cases {
		h := int64(i + 1)
		res, err := sm.LoadABCIResponses(stateDB, h)
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
	_, err := sm.LoadValidators(stateDB, 0)
	assert.IsType(sm.ErrNoValSetForHeight{}, err, "expected err at height 0")

	// Should be able to load for height 1.
	v, err := sm.LoadValidators(stateDB, 1)
	assert.Nil(err, "expected no err at height 1")
	assert.Equal(v.Hash(), state.Validators.Hash(), "expected validator hashes to match")

	// Should be able to load for height 2.
	v, err = sm.LoadValidators(stateDB, 2)
	assert.Nil(err, "expected no err at height 2")
	assert.Equal(v.Hash(), state.NextValidators.Hash(), "expected validator hashes to match")

	// Increment height, save; should be able to load for next & next next height.
	state.LastBlockHeight++
	nextHeight := state.LastBlockHeight + 1
	sm.SaveValidatorsInfo(stateDB, nextHeight+1, state.LastHeightValidatorsChanged, state.NextValidators)
	vp0, err := sm.LoadValidators(stateDB, nextHeight+0)
	assert.Nil(err, "expected no err")
	vp1, err := sm.LoadValidators(stateDB, nextHeight+1)
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
		header, blockID, responses := makeHeaderPartsResponsesValPowerChange(state, power)
		validatorUpdates, err = types.PB2TM.ValidatorUpdates(responses.EndBlock.ValidatorUpdates)
		require.NoError(t, err)
		state, err = sm.UpdateState(state, blockID, &header, responses, validatorUpdates)
		require.NoError(t, err)
		nextHeight := state.LastBlockHeight + 1
		sm.SaveValidatorsInfo(stateDB, nextHeight+1, state.LastHeightValidatorsChanged, state.NextValidators)
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
		v, err := sm.LoadValidators(stateDB, int64(i+1+1)) // +1 because vset changes delayed by 1 block.
		assert.Nil(t, err, fmt.Sprintf("expected no err at height %d", i))
		assert.Equal(t, v.Size(), 1, "validator set size is greater than 1: %d", v.Size())
		_, val := v.GetByIndex(0)

		assert.Equal(t, val.VotingPower, power, fmt.Sprintf(`unexpected powerat
                height %d`, i))
	}
}

func TestProposerFrequency(t *testing.T) {

	// some explicit test cases
	testCases := []struct {
		powers []int64
	}{
		// 2 vals
		{[]int64{1, 1}},
		{[]int64{1, 2}},
		{[]int64{1, 100}},
		{[]int64{5, 5}},
		{[]int64{5, 100}},
		{[]int64{50, 50}},
		{[]int64{50, 100}},
		{[]int64{1, 1000}},

		// 3 vals
		{[]int64{1, 1, 1}},
		{[]int64{1, 2, 3}},
		{[]int64{1, 2, 3}},
		{[]int64{1, 1, 10}},
		{[]int64{1, 1, 100}},
		{[]int64{1, 10, 100}},
		{[]int64{1, 1, 1000}},
		{[]int64{1, 10, 1000}},
		{[]int64{1, 100, 1000}},

		// 4 vals
		{[]int64{1, 1, 1, 1}},
		{[]int64{1, 2, 3, 4}},
		{[]int64{1, 1, 1, 10}},
		{[]int64{1, 1, 1, 100}},
		{[]int64{1, 1, 1, 1000}},
		{[]int64{1, 1, 10, 100}},
		{[]int64{1, 1, 10, 1000}},
		{[]int64{1, 1, 100, 1000}},
		{[]int64{1, 10, 100, 1000}},
	}

	for caseNum, testCase := range testCases {
		// run each case 5 times to sample different
		// initial priorities
		for i := 0; i < 5; i++ {
			valSet := genValSetWithPowers(testCase.powers)
			testProposerFreq(t, caseNum, valSet)
		}
	}

	// some random test cases with up to 100 validators
	maxVals := 100
	maxPower := 1000
	nTestCases := 5
	for i := 0; i < nTestCases; i++ {
		N := cmn.RandInt()%maxVals + 1
		vals := make([]*types.Validator, N)
		totalVotePower := int64(0)
		for j := 0; j < N; j++ {
			// make sure votePower > 0
			votePower := int64(cmn.RandInt()%maxPower) + 1
			totalVotePower += votePower
			privVal := types.NewMockPV()
			pubKey := privVal.GetPubKey()
			val := types.NewValidator(pubKey, votePower)
			val.ProposerPriority = cmn.RandInt64()
			vals[j] = val
		}
		valSet := types.NewValidatorSet(vals)
		valSet.RescalePriorities(totalVotePower)
		testProposerFreq(t, i, valSet)
	}
}

// new val set with given powers and random initial priorities
func genValSetWithPowers(powers []int64) *types.ValidatorSet {
	size := len(powers)
	vals := make([]*types.Validator, size)
	totalVotePower := int64(0)
	for i := 0; i < size; i++ {
		totalVotePower += powers[i]
		val := types.NewValidator(ed25519.GenPrivKey().PubKey(), powers[i])
		val.ProposerPriority = cmn.RandInt64()
		vals[i] = val
	}
	valSet := types.NewValidatorSet(vals)
	valSet.RescalePriorities(totalVotePower)
	return valSet
}

// test a proposer appears as frequently as expected
func testProposerFreq(t *testing.T, caseNum int, valSet *types.ValidatorSet) {
	N := valSet.Size()
	totalPower := valSet.TotalVotingPower()

	// run the proposer selection and track frequencies
	runMult := 1
	runs := int(totalPower) * runMult
	freqs := make([]int, N)
	for i := 0; i < runs; i++ {
		prop := valSet.GetProposer()
		idx, _ := valSet.GetByAddress(prop.Address)
		freqs[idx] += 1
		valSet.IncrementProposerPriority(1)
	}

	// assert frequencies match expected (max off by 1)
	for i, freq := range freqs {
		_, val := valSet.GetByIndex(i)
		expectFreq := int(val.VotingPower) * runMult
		gotFreq := freq
		abs := int(math.Abs(float64(expectFreq - gotFreq)))

		// max bound on expected vs seen freq was proven
		// to be 1 for the 2 validator case in
		// https://github.com/cwgoes/tm-proposer-idris
		// and inferred to generalize to N-1
		bound := N - 1
		require.True(t, abs <= bound, fmt.Sprintf("Case %d val %d (%d): got %d, expected %d", caseNum, i, N, gotFreq, expectFreq))
	}
}

// TestProposerPriorityDoesNotGetResetToZero assert that we preserve accum when calling updateState
// see https://github.com/tendermint/tendermint/issues/2718
func TestProposerPriorityDoesNotGetResetToZero(t *testing.T) {
	tearDown, _, state := setupTestCase(t)
	defer tearDown(t)
	val1VotingPower := int64(10)
	val1PubKey := ed25519.GenPrivKey().PubKey()
	val1 := &types.Validator{Address: val1PubKey.Address(), PubKey: val1PubKey, VotingPower: val1VotingPower}

	state.Validators = types.NewValidatorSet([]*types.Validator{val1})
	state.NextValidators = state.Validators

	// NewValidatorSet calls IncrementProposerPriority but uses on a copy of val1
	assert.EqualValues(t, 0, val1.ProposerPriority)

	block := makeBlock(state, state.LastBlockHeight+1)
	blockID := types.BlockID{Hash: block.Hash(), PartsHeader: block.MakePartSet(testPartSize).Header()}
	abciResponses := &sm.ABCIResponses{
		EndBlock: &abci.ResponseEndBlock{ValidatorUpdates: nil},
	}
	validatorUpdates, err := types.PB2TM.ValidatorUpdates(abciResponses.EndBlock.ValidatorUpdates)
	require.NoError(t, err)
	updatedState, err := sm.UpdateState(state, blockID, &block.Header, abciResponses, validatorUpdates)
	assert.NoError(t, err)
	curTotal := val1VotingPower
	// one increment step and one validator: 0 + power - total_power == 0
	assert.Equal(t, 0+val1VotingPower-curTotal, updatedState.NextValidators.Validators[0].ProposerPriority)

	// add a validator
	val2PubKey := ed25519.GenPrivKey().PubKey()
	val2VotingPower := int64(100)
	updateAddVal := abci.ValidatorUpdate{PubKey: types.TM2PB.PubKey(val2PubKey), Power: val2VotingPower}
	validatorUpdates, err = types.PB2TM.ValidatorUpdates([]abci.ValidatorUpdate{updateAddVal})
	assert.NoError(t, err)
	updatedState2, err := sm.UpdateState(updatedState, blockID, &block.Header, abciResponses, validatorUpdates)
	assert.NoError(t, err)

	require.Equal(t, len(updatedState2.NextValidators.Validators), 2)
	_, updatedVal1 := updatedState2.NextValidators.GetByAddress(val1PubKey.Address())
	_, addedVal2 := updatedState2.NextValidators.GetByAddress(val2PubKey.Address())

	// adding a validator should not lead to a ProposerPriority equal to zero (unless the combination of averaging and
	// incrementing would cause so; which is not the case here)
	// Steps from adding new validator:
	// 0 - val1 prio is 0, TVP after add:
	wantVal1Prio := int64(0)
	totalPowerAfter := val1VotingPower + val2VotingPower
	// 1. Add - Val2 should be initially added with (-123) =>
	wantVal2Prio := -(totalPowerAfter + (totalPowerAfter >> 3))
	// 2. Scale - noop
	// 3. Center - with avg, resulting val2:-61, val1:62
	avg := big.NewInt(0).Add(big.NewInt(wantVal1Prio), big.NewInt(wantVal2Prio))
	avg.Div(avg, big.NewInt(2))
	wantVal2Prio -= avg.Int64() // -61
	wantVal1Prio -= avg.Int64() // 62

	// 4. Steps from IncrementProposerPriority
	wantVal1Prio += val1VotingPower // 72
	wantVal2Prio += val2VotingPower // 39
	wantVal1Prio -= totalPowerAfter // -38 as val1 is proposer

	assert.Equal(t, wantVal1Prio, updatedVal1.ProposerPriority)
	assert.Equal(t, wantVal2Prio, addedVal2.ProposerPriority)

	// Updating a validator does not reset the ProposerPriority to zero:
	// 1. Add - Val2 VotingPower change to 1 =>
	updatedVotingPowVal2 := int64(1)
	updateVal := abci.ValidatorUpdate{PubKey: types.TM2PB.PubKey(val2PubKey), Power: updatedVotingPowVal2}
	validatorUpdates, err = types.PB2TM.ValidatorUpdates([]abci.ValidatorUpdate{updateVal})
	assert.NoError(t, err)

	// this will cause the diff of priorities (77)
	// to be larger than threshold == 2*totalVotingPower (22):
	updatedState3, err := sm.UpdateState(updatedState2, blockID, &block.Header, abciResponses, validatorUpdates)
	assert.NoError(t, err)

	require.Equal(t, len(updatedState3.NextValidators.Validators), 2)
	_, prevVal1 := updatedState3.Validators.GetByAddress(val1PubKey.Address())
	_, prevVal2 := updatedState3.Validators.GetByAddress(val2PubKey.Address())
	_, updatedVal1 = updatedState3.NextValidators.GetByAddress(val1PubKey.Address())
	_, updatedVal2 := updatedState3.NextValidators.GetByAddress(val2PubKey.Address())

	// 2. Scale
	// old prios: v1(10):-38, v2(1):39
	wantVal1Prio = prevVal1.ProposerPriority
	wantVal2Prio = prevVal2.ProposerPriority
	// scale to diffMax = 22 = 2 * tvp, diff=39-(-38)=77
	// new totalPower
	totalPower := updatedVal1.VotingPower + updatedVal2.VotingPower
	dist := wantVal2Prio - wantVal1Prio
	// ratio := (dist + 2*totalPower - 1) / 2*totalPower = 98/22 = 4
	ratio := (dist + 2*totalPower - 1) / (2 * totalPower)
	// v1(10):-38/4, v2(1):39/4
	wantVal1Prio /= ratio // -9
	wantVal2Prio /= ratio // 9

	// 3. Center - noop
	// 4. IncrementProposerPriority() ->
	// v1(10):-9+10, v2(1):9+1 -> v2 proposer so subsract tvp(11)
	// v1(10):1, v2(1):-1
	wantVal2Prio += updatedVal2.VotingPower // 10 -> prop
	wantVal1Prio += updatedVal1.VotingPower // 1
	wantVal2Prio -= totalPower              // -1

	assert.Equal(t, wantVal2Prio, updatedVal2.ProposerPriority)
	assert.Equal(t, wantVal1Prio, updatedVal1.ProposerPriority)
}

func TestProposerPriorityProposerAlternates(t *testing.T) {
	// Regression test that would fail if the inner workings of
	// IncrementProposerPriority change.
	// Additionally, make sure that same power validators alternate if both
	// have the same voting power (and the 2nd was added later).
	tearDown, _, state := setupTestCase(t)
	defer tearDown(t)
	val1VotingPower := int64(10)
	val1PubKey := ed25519.GenPrivKey().PubKey()
	val1 := &types.Validator{Address: val1PubKey.Address(), PubKey: val1PubKey, VotingPower: val1VotingPower}

	// reset state validators to above validator
	state.Validators = types.NewValidatorSet([]*types.Validator{val1})
	state.NextValidators = state.Validators
	// we only have one validator:
	assert.Equal(t, val1PubKey.Address(), state.Validators.Proposer.Address)

	block := makeBlock(state, state.LastBlockHeight+1)
	blockID := types.BlockID{Hash: block.Hash(), PartsHeader: block.MakePartSet(testPartSize).Header()}
	// no updates:
	abciResponses := &sm.ABCIResponses{
		EndBlock: &abci.ResponseEndBlock{ValidatorUpdates: nil},
	}
	validatorUpdates, err := types.PB2TM.ValidatorUpdates(abciResponses.EndBlock.ValidatorUpdates)
	require.NoError(t, err)

	updatedState, err := sm.UpdateState(state, blockID, &block.Header, abciResponses, validatorUpdates)
	assert.NoError(t, err)

	// 0 + 10 (initial prio) - 10 (avg) - 10 (mostest - total) = -10
	totalPower := val1VotingPower
	wantVal1Prio := 0 + val1VotingPower - totalPower
	assert.Equal(t, wantVal1Prio, updatedState.NextValidators.Validators[0].ProposerPriority)
	assert.Equal(t, val1PubKey.Address(), updatedState.NextValidators.Proposer.Address)

	// add a validator with the same voting power as the first
	val2PubKey := ed25519.GenPrivKey().PubKey()
	updateAddVal := abci.ValidatorUpdate{PubKey: types.TM2PB.PubKey(val2PubKey), Power: val1VotingPower}
	validatorUpdates, err = types.PB2TM.ValidatorUpdates([]abci.ValidatorUpdate{updateAddVal})
	assert.NoError(t, err)

	updatedState2, err := sm.UpdateState(updatedState, blockID, &block.Header, abciResponses, validatorUpdates)
	assert.NoError(t, err)

	require.Equal(t, len(updatedState2.NextValidators.Validators), 2)
	assert.Equal(t, updatedState2.Validators, updatedState.NextValidators)

	// val1 will still be proposer as val2 just got added:
	assert.Equal(t, val1PubKey.Address(), updatedState.NextValidators.Proposer.Address)
	assert.Equal(t, updatedState2.Validators.Proposer.Address, updatedState2.NextValidators.Proposer.Address)
	assert.Equal(t, updatedState2.Validators.Proposer.Address, val1PubKey.Address())
	assert.Equal(t, updatedState2.NextValidators.Proposer.Address, val1PubKey.Address())

	_, updatedVal1 := updatedState2.NextValidators.GetByAddress(val1PubKey.Address())
	_, oldVal1 := updatedState2.Validators.GetByAddress(val1PubKey.Address())
	_, updatedVal2 := updatedState2.NextValidators.GetByAddress(val2PubKey.Address())

	// 1. Add
	val2VotingPower := val1VotingPower
	totalPower = val1VotingPower + val2VotingPower           // 20
	v2PrioWhenAddedVal2 := -(totalPower + (totalPower >> 3)) // -22
	// 2. Scale - noop
	// 3. Center
	avgSum := big.NewInt(0).Add(big.NewInt(v2PrioWhenAddedVal2), big.NewInt(oldVal1.ProposerPriority))
	avg := avgSum.Div(avgSum, big.NewInt(2))                   // -11
	expectedVal2Prio := v2PrioWhenAddedVal2 - avg.Int64()      // -11
	expectedVal1Prio := oldVal1.ProposerPriority - avg.Int64() // 11
	// 4. Increment
	expectedVal2Prio += val2VotingPower // -11 + 10 = -1
	expectedVal1Prio += val1VotingPower // 11 + 10 == 21
	expectedVal1Prio -= totalPower      // 1, val1 proposer

	assert.EqualValues(t, expectedVal1Prio, updatedVal1.ProposerPriority)
	assert.EqualValues(t, expectedVal2Prio, updatedVal2.ProposerPriority, "unexpected proposer priority for validator: %v", updatedVal2)

	validatorUpdates, err = types.PB2TM.ValidatorUpdates(abciResponses.EndBlock.ValidatorUpdates)
	require.NoError(t, err)

	updatedState3, err := sm.UpdateState(updatedState2, blockID, &block.Header, abciResponses, validatorUpdates)
	assert.NoError(t, err)

	assert.Equal(t, updatedState3.Validators.Proposer.Address, updatedState3.NextValidators.Proposer.Address)

	assert.Equal(t, updatedState3.Validators, updatedState2.NextValidators)
	_, updatedVal1 = updatedState3.NextValidators.GetByAddress(val1PubKey.Address())
	_, updatedVal2 = updatedState3.NextValidators.GetByAddress(val2PubKey.Address())

	// val1 will still be proposer:
	assert.Equal(t, val1PubKey.Address(), updatedState3.NextValidators.Proposer.Address)

	// check if expected proposer prio is matched:
	// Increment
	expectedVal2Prio2 := expectedVal2Prio + val2VotingPower // -1 + 10 = 9
	expectedVal1Prio2 := expectedVal1Prio + val1VotingPower // 1 + 10 == 11
	expectedVal1Prio2 -= totalPower                         // -9, val1 proposer

	assert.EqualValues(t, expectedVal1Prio2, updatedVal1.ProposerPriority, "unexpected proposer priority for validator: %v", updatedVal2)
	assert.EqualValues(t, expectedVal2Prio2, updatedVal2.ProposerPriority, "unexpected proposer priority for validator: %v", updatedVal2)

	// no changes in voting power and both validators have same voting power
	// -> proposers should alternate:
	oldState := updatedState3
	abciResponses = &sm.ABCIResponses{
		EndBlock: &abci.ResponseEndBlock{ValidatorUpdates: nil},
	}
	validatorUpdates, err = types.PB2TM.ValidatorUpdates(abciResponses.EndBlock.ValidatorUpdates)
	require.NoError(t, err)

	oldState, err = sm.UpdateState(oldState, blockID, &block.Header, abciResponses, validatorUpdates)
	assert.NoError(t, err)
	expectedVal1Prio2 = 1
	expectedVal2Prio2 = -1
	expectedVal1Prio = -9
	expectedVal2Prio = 9

	for i := 0; i < 1000; i++ {
		// no validator updates:
		abciResponses := &sm.ABCIResponses{
			EndBlock: &abci.ResponseEndBlock{ValidatorUpdates: nil},
		}
		validatorUpdates, err = types.PB2TM.ValidatorUpdates(abciResponses.EndBlock.ValidatorUpdates)
		require.NoError(t, err)

		updatedState, err := sm.UpdateState(oldState, blockID, &block.Header, abciResponses, validatorUpdates)
		assert.NoError(t, err)
		// alternate (and cyclic priorities):
		assert.NotEqual(t, updatedState.Validators.Proposer.Address, updatedState.NextValidators.Proposer.Address, "iter: %v", i)
		assert.Equal(t, oldState.Validators.Proposer.Address, updatedState.NextValidators.Proposer.Address, "iter: %v", i)

		_, updatedVal1 = updatedState.NextValidators.GetByAddress(val1PubKey.Address())
		_, updatedVal2 = updatedState.NextValidators.GetByAddress(val2PubKey.Address())

		if i%2 == 0 {
			assert.Equal(t, updatedState.Validators.Proposer.Address, val2PubKey.Address())
			assert.Equal(t, expectedVal1Prio, updatedVal1.ProposerPriority) // -19
			assert.Equal(t, expectedVal2Prio, updatedVal2.ProposerPriority) // 0
		} else {
			assert.Equal(t, updatedState.Validators.Proposer.Address, val1PubKey.Address())
			assert.Equal(t, expectedVal1Prio2, updatedVal1.ProposerPriority) // -9
			assert.Equal(t, expectedVal2Prio2, updatedVal2.ProposerPriority) // -10
		}
		// update for next iteration:
		oldState = updatedState
	}
}

func TestLargeGenesisValidator(t *testing.T) {
	tearDown, _, state := setupTestCase(t)
	defer tearDown(t)

	genesisVotingPower := types.MaxTotalVotingPower / 1000
	genesisPubKey := ed25519.GenPrivKey().PubKey()
	// fmt.Println("genesis addr: ", genesisPubKey.Address())
	genesisVal := &types.Validator{Address: genesisPubKey.Address(), PubKey: genesisPubKey, VotingPower: genesisVotingPower}
	// reset state validators to above validator
	state.Validators = types.NewValidatorSet([]*types.Validator{genesisVal})
	state.NextValidators = state.Validators
	require.True(t, len(state.Validators.Validators) == 1)

	// update state a few times with no validator updates
	// asserts that the single validator's ProposerPrio stays the same
	oldState := state
	for i := 0; i < 10; i++ {
		// no updates:
		abciResponses := &sm.ABCIResponses{
			EndBlock: &abci.ResponseEndBlock{ValidatorUpdates: nil},
		}
		validatorUpdates, err := types.PB2TM.ValidatorUpdates(abciResponses.EndBlock.ValidatorUpdates)
		require.NoError(t, err)

		block := makeBlock(oldState, oldState.LastBlockHeight+1)
		blockID := types.BlockID{Hash: block.Hash(), PartsHeader: block.MakePartSet(testPartSize).Header()}

		updatedState, err := sm.UpdateState(oldState, blockID, &block.Header, abciResponses, validatorUpdates)
		require.NoError(t, err)
		// no changes in voting power (ProposerPrio += VotingPower == Voting in 1st round; than shiftByAvg == 0,
		// than -Total == -Voting)
		// -> no change in ProposerPrio (stays zero):
		assert.EqualValues(t, oldState.NextValidators, updatedState.NextValidators)
		assert.EqualValues(t, 0, updatedState.NextValidators.Proposer.ProposerPriority)

		oldState = updatedState
	}
	// add another validator, do a few iterations (create blocks),
	// add more validators with same voting power as the 2nd
	// let the genesis validator "unbond",
	// see how long it takes until the effect wears off and both begin to alternate
	// see: https://github.com/tendermint/tendermint/issues/2960
	firstAddedValPubKey := ed25519.GenPrivKey().PubKey()
	firstAddedValVotingPower := int64(10)
	firstAddedVal := abci.ValidatorUpdate{PubKey: types.TM2PB.PubKey(firstAddedValPubKey), Power: firstAddedValVotingPower}
	validatorUpdates, err := types.PB2TM.ValidatorUpdates([]abci.ValidatorUpdate{firstAddedVal})
	assert.NoError(t, err)
	abciResponses := &sm.ABCIResponses{
		EndBlock: &abci.ResponseEndBlock{ValidatorUpdates: []abci.ValidatorUpdate{firstAddedVal}},
	}
	block := makeBlock(oldState, oldState.LastBlockHeight+1)
	blockID := types.BlockID{Hash: block.Hash(), PartsHeader: block.MakePartSet(testPartSize).Header()}
	updatedState, err := sm.UpdateState(oldState, blockID, &block.Header, abciResponses, validatorUpdates)
	require.NoError(t, err)

	lastState := updatedState
	for i := 0; i < 200; i++ {
		// no updates:
		abciResponses := &sm.ABCIResponses{
			EndBlock: &abci.ResponseEndBlock{ValidatorUpdates: nil},
		}
		validatorUpdates, err := types.PB2TM.ValidatorUpdates(abciResponses.EndBlock.ValidatorUpdates)
		require.NoError(t, err)

		block := makeBlock(lastState, lastState.LastBlockHeight+1)
		blockID := types.BlockID{Hash: block.Hash(), PartsHeader: block.MakePartSet(testPartSize).Header()}

		updatedStateInner, err := sm.UpdateState(lastState, blockID, &block.Header, abciResponses, validatorUpdates)
		require.NoError(t, err)
		lastState = updatedStateInner
	}
	// set state to last state of above iteration
	state = lastState

	// set oldState to state before above iteration
	oldState = updatedState
	_, oldGenesisVal := oldState.NextValidators.GetByAddress(genesisVal.Address)
	_, newGenesisVal := state.NextValidators.GetByAddress(genesisVal.Address)
	_, addedOldVal := oldState.NextValidators.GetByAddress(firstAddedValPubKey.Address())
	_, addedNewVal := state.NextValidators.GetByAddress(firstAddedValPubKey.Address())
	// expect large negative proposer priority for both (genesis validator decreased, 2nd validator increased):
	assert.True(t, oldGenesisVal.ProposerPriority > newGenesisVal.ProposerPriority)
	assert.True(t, addedOldVal.ProposerPriority < addedNewVal.ProposerPriority)

	// add 10 validators with the same voting power as the one added directly after genesis:
	for i := 0; i < 10; i++ {
		addedPubKey := ed25519.GenPrivKey().PubKey()

		addedVal := abci.ValidatorUpdate{PubKey: types.TM2PB.PubKey(addedPubKey), Power: firstAddedValVotingPower}
		validatorUpdates, err := types.PB2TM.ValidatorUpdates([]abci.ValidatorUpdate{addedVal})
		assert.NoError(t, err)

		abciResponses := &sm.ABCIResponses{
			EndBlock: &abci.ResponseEndBlock{ValidatorUpdates: []abci.ValidatorUpdate{addedVal}},
		}
		block := makeBlock(oldState, oldState.LastBlockHeight+1)
		blockID := types.BlockID{Hash: block.Hash(), PartsHeader: block.MakePartSet(testPartSize).Header()}
		state, err = sm.UpdateState(state, blockID, &block.Header, abciResponses, validatorUpdates)
		require.NoError(t, err)
	}
	require.Equal(t, 10+2, len(state.NextValidators.Validators))

	// remove genesis validator:
	removeGenesisVal := abci.ValidatorUpdate{PubKey: types.TM2PB.PubKey(genesisPubKey), Power: 0}
	abciResponses = &sm.ABCIResponses{
		EndBlock: &abci.ResponseEndBlock{ValidatorUpdates: []abci.ValidatorUpdate{removeGenesisVal}},
	}
	block = makeBlock(oldState, oldState.LastBlockHeight+1)
	blockID = types.BlockID{Hash: block.Hash(), PartsHeader: block.MakePartSet(testPartSize).Header()}
	validatorUpdates, err = types.PB2TM.ValidatorUpdates(abciResponses.EndBlock.ValidatorUpdates)
	require.NoError(t, err)
	updatedState, err = sm.UpdateState(state, blockID, &block.Header, abciResponses, validatorUpdates)
	require.NoError(t, err)
	// only the first added val (not the genesis val) should be left
	assert.Equal(t, 11, len(updatedState.NextValidators.Validators))

	// call update state until the effect for the 3rd added validator
	// being proposer for a long time after the genesis validator left wears off:
	curState := updatedState
	count := 0
	isProposerUnchanged := true
	for isProposerUnchanged {
		abciResponses := &sm.ABCIResponses{
			EndBlock: &abci.ResponseEndBlock{ValidatorUpdates: nil},
		}
		validatorUpdates, err = types.PB2TM.ValidatorUpdates(abciResponses.EndBlock.ValidatorUpdates)
		require.NoError(t, err)
		block = makeBlock(curState, curState.LastBlockHeight+1)
		blockID = types.BlockID{Hash: block.Hash(), PartsHeader: block.MakePartSet(testPartSize).Header()}
		curState, err = sm.UpdateState(curState, blockID, &block.Header, abciResponses, validatorUpdates)
		require.NoError(t, err)
		if !bytes.Equal(curState.Validators.Proposer.Address, curState.NextValidators.Proposer.Address) {
			isProposerUnchanged = false
		}
		count++
	}
	updatedState = curState
	// the proposer changes after this number of blocks
	firstProposerChangeExpectedAfter := 1
	assert.Equal(t, firstProposerChangeExpectedAfter, count)
	// store proposers here to see if we see them again in the same order:
	numVals := len(updatedState.Validators.Validators)
	proposers := make([]*types.Validator, numVals)
	for i := 0; i < 100; i++ {
		// no updates:
		abciResponses := &sm.ABCIResponses{
			EndBlock: &abci.ResponseEndBlock{ValidatorUpdates: nil},
		}
		validatorUpdates, err := types.PB2TM.ValidatorUpdates(abciResponses.EndBlock.ValidatorUpdates)
		require.NoError(t, err)

		block := makeBlock(updatedState, updatedState.LastBlockHeight+1)
		blockID := types.BlockID{Hash: block.Hash(), PartsHeader: block.MakePartSet(testPartSize).Header()}

		updatedState, err = sm.UpdateState(updatedState, blockID, &block.Header, abciResponses, validatorUpdates)
		require.NoError(t, err)
		if i > numVals { // expect proposers to cycle through after the first iteration (of numVals blocks):
			if proposers[i%numVals] == nil {
				proposers[i%numVals] = updatedState.NextValidators.Proposer
			} else {
				assert.Equal(t, proposers[i%numVals], updatedState.NextValidators.Proposer)
			}
		}
	}
}

func TestStoreLoadValidatorsIncrementsProposerPriority(t *testing.T) {
	const valSetSize = 2
	tearDown, stateDB, state := setupTestCase(t)
	defer tearDown(t)
	state.Validators = genValSet(valSetSize)
	state.NextValidators = state.Validators.CopyIncrementProposerPriority(1)
	sm.SaveState(stateDB, state)

	nextHeight := state.LastBlockHeight + 1

	v0, err := sm.LoadValidators(stateDB, nextHeight)
	assert.Nil(t, err)
	acc0 := v0.Validators[0].ProposerPriority

	v1, err := sm.LoadValidators(stateDB, nextHeight+1)
	assert.Nil(t, err)
	acc1 := v1.Validators[0].ProposerPriority

	assert.NotEqual(t, acc1, acc0, "expected ProposerPriority value to change between heights")
}

// TestValidatorChangesSaveLoad tests saving and loading a validator set with
// changes.
func TestManyValidatorChangesSaveLoad(t *testing.T) {
	const valSetSize = 7
	tearDown, stateDB, state := setupTestCase(t)
	defer tearDown(t)
	require.Equal(t, int64(0), state.LastBlockHeight)
	state.Validators = genValSet(valSetSize)
	state.NextValidators = state.Validators.CopyIncrementProposerPriority(1)
	sm.SaveState(stateDB, state)

	_, valOld := state.Validators.GetByIndex(0)
	var pubkeyOld = valOld.PubKey
	pubkey := ed25519.GenPrivKey().PubKey()

	// Swap the first validator with a new one (validator set size stays the same).
	header, blockID, responses := makeHeaderPartsResponsesValPubKeyChange(state, pubkey)

	// Save state etc.
	var err error
	var validatorUpdates []*types.Validator
	validatorUpdates, err = types.PB2TM.ValidatorUpdates(responses.EndBlock.ValidatorUpdates)
	require.NoError(t, err)
	state, err = sm.UpdateState(state, blockID, &header, responses, validatorUpdates)
	require.Nil(t, err)
	nextHeight := state.LastBlockHeight + 1
	sm.SaveValidatorsInfo(stateDB, nextHeight+1, state.LastHeightValidatorsChanged, state.NextValidators)

	// Load nextheight, it should be the oldpubkey.
	v0, err := sm.LoadValidators(stateDB, nextHeight)
	assert.Nil(t, err)
	assert.Equal(t, valSetSize, v0.Size())
	index, val := v0.GetByAddress(pubkeyOld.Address())
	assert.NotNil(t, val)
	if index < 0 {
		t.Fatal("expected to find old validator")
	}

	// Load nextheight+1, it should be the new pubkey.
	v1, err := sm.LoadValidators(stateDB, nextHeight+1)
	assert.Nil(t, err)
	assert.Equal(t, valSetSize, v1.Size())
	index, val = v1.GetByAddress(pubkey.Address())
	assert.NotNil(t, val)
	if index < 0 {
		t.Fatal("expected to find newly added validator")
	}
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
		params[i].Block.MaxBytes += int64(i)
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
		header, blockID, responses := makeHeaderPartsResponsesParams(state, cp)
		validatorUpdates, err = types.PB2TM.ValidatorUpdates(responses.EndBlock.ValidatorUpdates)
		require.NoError(t, err)
		state, err = sm.UpdateState(state, blockID, &header, responses, validatorUpdates)

		require.Nil(t, err)
		nextHeight := state.LastBlockHeight + 1
		sm.SaveConsensusParamsInfo(stateDB, nextHeight, state.LastHeightConsensusParamsChanged, state.ConsensusParams)
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
		p, err := sm.LoadConsensusParams(stateDB, testCase.height)
		assert.Nil(t, err, fmt.Sprintf("expected no err at height %d", testCase.height))
		assert.Equal(t, testCase.params, p, fmt.Sprintf(`unexpected consensus params at
                height %d`, testCase.height))
	}
}

func TestApplyUpdates(t *testing.T) {
	initParams := makeConsensusParams(1, 2, 3, 4)

	cases := [...]struct {
		init     types.ConsensusParams
		updates  abci.ConsensusParams
		expected types.ConsensusParams
	}{
		0: {initParams, abci.ConsensusParams{}, initParams},
		1: {initParams, abci.ConsensusParams{}, initParams},
		2: {initParams,
			abci.ConsensusParams{
				Block: &abci.BlockParams{
					MaxBytes: 44,
					MaxGas:   55,
				},
			},
			makeConsensusParams(44, 55, 3, 4)},
		3: {initParams,
			abci.ConsensusParams{
				Evidence: &abci.EvidenceParams{
					MaxAge: 66,
				},
			},
			makeConsensusParams(1, 2, 3, 66)},
	}

	for i, tc := range cases {
		res := tc.init.Update(&(tc.updates))
		assert.Equal(t, tc.expected, res, "case %d", i)
	}
}
