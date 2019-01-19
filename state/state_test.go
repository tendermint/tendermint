package state

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
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

	// some random test cases with up to 300 validators
	maxVals := 100
	maxPower := 1000
	nTestCases := 5
	for i := 0; i < nTestCases; i++ {
		N := cmn.RandInt() % maxVals
		vals := make([]*types.Validator, N)
		totalVotePower := int64(0)
		for j := 0; j < N; j++ {
			votePower := int64(cmn.RandInt() % maxPower)
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
	blockID := types.BlockID{block.Hash(), block.MakePartSet(testPartSize).Header()}
	abciResponses := &ABCIResponses{
		EndBlock: &abci.ResponseEndBlock{ValidatorUpdates: nil},
	}
	validatorUpdates, err := types.PB2TM.ValidatorUpdates(abciResponses.EndBlock.ValidatorUpdates)
	require.NoError(t, err)
	updatedState, err := updateState(state, blockID, &block.Header, abciResponses, validatorUpdates)
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
	updatedState2, err := updateState(updatedState, blockID, &block.Header, abciResponses, validatorUpdates)
	assert.NoError(t, err)

	require.Equal(t, len(updatedState2.NextValidators.Validators), 2)
	_, updatedVal1 := updatedState2.NextValidators.GetByAddress(val1PubKey.Address())
	_, addedVal2 := updatedState2.NextValidators.GetByAddress(val2PubKey.Address())
	// adding a validator should not lead to a ProposerPriority equal to zero (unless the combination of averaging and
	// incrementing would cause so; which is not the case here)
	totalPowerBefore2 := curTotal
	// while adding we compute prio == -1.125 * total:
	wantVal2ProposerPrio := -(totalPowerBefore2 + (totalPowerBefore2 >> 3))
	wantVal2ProposerPrio = wantVal2ProposerPrio + val2VotingPower
	// then increment:
	totalPowerAfter := val1VotingPower + val2VotingPower
	// mostest:
	wantVal2ProposerPrio = wantVal2ProposerPrio - totalPowerAfter
	avg := big.NewInt(0).Add(big.NewInt(val1VotingPower), big.NewInt(wantVal2ProposerPrio))
	avg.Div(avg, big.NewInt(2))
	wantVal2ProposerPrio = wantVal2ProposerPrio - avg.Int64()
	wantVal1Prio := 0 + val1VotingPower - avg.Int64()
	assert.Equal(t, wantVal1Prio, updatedVal1.ProposerPriority)
	assert.Equal(t, wantVal2ProposerPrio, addedVal2.ProposerPriority)

	// Updating a validator does not reset the ProposerPriority to zero:
	updatedVotingPowVal2 := int64(1)
	updateVal := abci.ValidatorUpdate{PubKey: types.TM2PB.PubKey(val2PubKey), Power: updatedVotingPowVal2}
	validatorUpdates, err = types.PB2TM.ValidatorUpdates([]abci.ValidatorUpdate{updateVal})
	assert.NoError(t, err)

	// this will cause the diff of priorities (31)
	// to be larger than threshold == 2*totalVotingPower (22):
	updatedState3, err := updateState(updatedState2, blockID, &block.Header, abciResponses, validatorUpdates)
	assert.NoError(t, err)

	require.Equal(t, len(updatedState3.NextValidators.Validators), 2)
	_, prevVal1 := updatedState3.Validators.GetByAddress(val1PubKey.Address())
	_, updatedVal2 := updatedState3.NextValidators.GetByAddress(val2PubKey.Address())

	// divide previous priorities by 2 == CEIL(31/22) as diff > threshold:
	expectedVal1PrioBeforeAvg := prevVal1.ProposerPriority/2 + prevVal1.VotingPower
	wantVal2ProposerPrio = wantVal2ProposerPrio/2 + updatedVotingPowVal2
	// val1 will be proposer:
	total := val1VotingPower + updatedVotingPowVal2
	expectedVal1PrioBeforeAvg = expectedVal1PrioBeforeAvg - total
	avgI64 := (wantVal2ProposerPrio + expectedVal1PrioBeforeAvg) / 2
	wantVal2ProposerPrio = wantVal2ProposerPrio - avgI64
	wantVal1Prio = expectedVal1PrioBeforeAvg - avgI64
	assert.Equal(t, wantVal2ProposerPrio, updatedVal2.ProposerPriority)
	_, updatedVal1 = updatedState3.NextValidators.GetByAddress(val1PubKey.Address())
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
	blockID := types.BlockID{block.Hash(), block.MakePartSet(testPartSize).Header()}
	// no updates:
	abciResponses := &ABCIResponses{
		EndBlock: &abci.ResponseEndBlock{ValidatorUpdates: nil},
	}
	validatorUpdates, err := types.PB2TM.ValidatorUpdates(abciResponses.EndBlock.ValidatorUpdates)
	require.NoError(t, err)

	updatedState, err := updateState(state, blockID, &block.Header, abciResponses, validatorUpdates)
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

	updatedState2, err := updateState(updatedState, blockID, &block.Header, abciResponses, validatorUpdates)
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

	totalPower = val1VotingPower // no update
	v2PrioWhenAddedVal2 := -(totalPower + (totalPower >> 3))
	v2PrioWhenAddedVal2 = v2PrioWhenAddedVal2 + val1VotingPower       // -11 + 10 == -1
	v1PrioWhenAddedVal2 := oldVal1.ProposerPriority + val1VotingPower // -10 + 10 == 0
	totalPower = 2 * val1VotingPower                                  // now we have to validators with that power
	v1PrioWhenAddedVal2 = v1PrioWhenAddedVal2 - totalPower            // mostest
	// have to express the AVG in big.Ints as -1/2 == -1 in big.Int while -1/2 == 0 in int64
	avgSum := big.NewInt(0).Add(big.NewInt(v2PrioWhenAddedVal2), big.NewInt(v1PrioWhenAddedVal2))
	avg := avgSum.Div(avgSum, big.NewInt(2))
	expectedVal2Prio := v2PrioWhenAddedVal2 - avg.Int64()
	totalPower = 2 * val1VotingPower // 10 + 10
	expectedVal1Prio := oldVal1.ProposerPriority + val1VotingPower - avg.Int64() - totalPower
	// val1's ProposerPriority story: -10 (see above) + 10 (voting pow) - (-1) (avg) - 20 (total) == -19
	assert.EqualValues(t, expectedVal1Prio, updatedVal1.ProposerPriority)
	// val2 prio when added: -(totalVotingPower + (totalVotingPower >> 3)) == -11
	// -> -11 + 10  (voting power) - (-1) (avg) == 0
	assert.EqualValues(t, expectedVal2Prio, updatedVal2.ProposerPriority, "unexpected proposer priority for validator: %v", updatedVal2)

	validatorUpdates, err = types.PB2TM.ValidatorUpdates(abciResponses.EndBlock.ValidatorUpdates)
	require.NoError(t, err)

	updatedState3, err := updateState(updatedState2, blockID, &block.Header, abciResponses, validatorUpdates)
	assert.NoError(t, err)

	// proposer changes from now on (every iteration) as long as there are no changes in the validator set:
	assert.NotEqual(t, updatedState3.Validators.Proposer.Address, updatedState3.NextValidators.Proposer.Address)

	assert.Equal(t, updatedState3.Validators, updatedState2.NextValidators)
	_, updatedVal1 = updatedState3.NextValidators.GetByAddress(val1PubKey.Address())
	_, oldVal1 = updatedState3.Validators.GetByAddress(val1PubKey.Address())
	_, updatedVal2 = updatedState3.NextValidators.GetByAddress(val2PubKey.Address())
	_, oldVal2 := updatedState3.Validators.GetByAddress(val2PubKey.Address())

	// val2 will be proposer:
	assert.Equal(t, val2PubKey.Address(), updatedState3.NextValidators.Proposer.Address)
	// check if expected proposer prio is matched:

	expectedVal1Prio2 := oldVal1.ProposerPriority + val1VotingPower
	expectedVal2Prio2 := oldVal2.ProposerPriority + val1VotingPower - totalPower
	avgSum = big.NewInt(expectedVal1Prio + expectedVal2Prio)
	avg = avgSum.Div(avgSum, big.NewInt(2))
	expectedVal1Prio -= avg.Int64()
	expectedVal2Prio -= avg.Int64()

	// -19 + 10 - 0 (avg) == -9
	assert.EqualValues(t, expectedVal1Prio2, updatedVal1.ProposerPriority, "unexpected proposer priority for validator: %v", updatedVal2)
	// 0 + 10 - 0 (avg) - 20 (total) == -10
	assert.EqualValues(t, expectedVal2Prio2, updatedVal2.ProposerPriority, "unexpected proposer priority for validator: %v", updatedVal2)

	// no changes in voting power and both validators have same voting power
	// -> proposers should alternate:
	oldState := updatedState3
	for i := 0; i < 1000; i++ {
		// no validator updates:
		abciResponses := &ABCIResponses{
			EndBlock: &abci.ResponseEndBlock{ValidatorUpdates: nil},
		}
		validatorUpdates, err = types.PB2TM.ValidatorUpdates(abciResponses.EndBlock.ValidatorUpdates)
		require.NoError(t, err)

		updatedState, err := updateState(oldState, blockID, &block.Header, abciResponses, validatorUpdates)
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

	genesisVotingPower := int64(types.MaxTotalVotingPower / 1000)
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
		abciResponses := &ABCIResponses{
			EndBlock: &abci.ResponseEndBlock{ValidatorUpdates: nil},
		}
		validatorUpdates, err := types.PB2TM.ValidatorUpdates(abciResponses.EndBlock.ValidatorUpdates)
		require.NoError(t, err)

		block := makeBlock(oldState, oldState.LastBlockHeight+1)
		blockID := types.BlockID{block.Hash(), block.MakePartSet(testPartSize).Header()}

		updatedState, err := updateState(oldState, blockID, &block.Header, abciResponses, validatorUpdates)
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
	abciResponses := &ABCIResponses{
		EndBlock: &abci.ResponseEndBlock{ValidatorUpdates: []abci.ValidatorUpdate{firstAddedVal}},
	}
	block := makeBlock(oldState, oldState.LastBlockHeight+1)
	blockID := types.BlockID{block.Hash(), block.MakePartSet(testPartSize).Header()}
	updatedState, err := updateState(oldState, blockID, &block.Header, abciResponses, validatorUpdates)

	lastState := updatedState
	for i := 0; i < 200; i++ {
		// no updates:
		abciResponses := &ABCIResponses{
			EndBlock: &abci.ResponseEndBlock{ValidatorUpdates: nil},
		}
		validatorUpdates, err := types.PB2TM.ValidatorUpdates(abciResponses.EndBlock.ValidatorUpdates)
		require.NoError(t, err)

		block := makeBlock(lastState, lastState.LastBlockHeight+1)
		blockID := types.BlockID{block.Hash(), block.MakePartSet(testPartSize).Header()}

		updatedStateInner, err := updateState(lastState, blockID, &block.Header, abciResponses, validatorUpdates)
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

		abciResponses := &ABCIResponses{
			EndBlock: &abci.ResponseEndBlock{ValidatorUpdates: []abci.ValidatorUpdate{addedVal}},
		}
		block := makeBlock(oldState, oldState.LastBlockHeight+1)
		blockID := types.BlockID{block.Hash(), block.MakePartSet(testPartSize).Header()}
		state, err = updateState(state, blockID, &block.Header, abciResponses, validatorUpdates)
	}
	require.Equal(t, 10+2, len(state.NextValidators.Validators))

	// remove genesis validator:
	removeGenesisVal := abci.ValidatorUpdate{PubKey: types.TM2PB.PubKey(genesisPubKey), Power: 0}
	abciResponses = &ABCIResponses{
		EndBlock: &abci.ResponseEndBlock{ValidatorUpdates: []abci.ValidatorUpdate{removeGenesisVal}},
	}
	block = makeBlock(oldState, oldState.LastBlockHeight+1)
	blockID = types.BlockID{block.Hash(), block.MakePartSet(testPartSize).Header()}
	validatorUpdates, err = types.PB2TM.ValidatorUpdates(abciResponses.EndBlock.ValidatorUpdates)
	require.NoError(t, err)
	updatedState, err = updateState(state, blockID, &block.Header, abciResponses, validatorUpdates)
	require.NoError(t, err)
	// only the first added val (not the genesis val) should be left
	assert.Equal(t, 11, len(updatedState.NextValidators.Validators))

	// call update state until the effect for the 3rd added validator
	// being proposer for a long time after the genesis validator left wears off:
	curState := updatedState
	count := 0
	isProposerUnchanged := true
	for isProposerUnchanged {
		abciResponses := &ABCIResponses{
			EndBlock: &abci.ResponseEndBlock{ValidatorUpdates: nil},
		}
		validatorUpdates, err = types.PB2TM.ValidatorUpdates(abciResponses.EndBlock.ValidatorUpdates)
		require.NoError(t, err)
		block = makeBlock(curState, curState.LastBlockHeight+1)
		blockID = types.BlockID{block.Hash(), block.MakePartSet(testPartSize).Header()}
		curState, err = updateState(curState, blockID, &block.Header, abciResponses, validatorUpdates)
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
		abciResponses := &ABCIResponses{
			EndBlock: &abci.ResponseEndBlock{ValidatorUpdates: nil},
		}
		validatorUpdates, err := types.PB2TM.ValidatorUpdates(abciResponses.EndBlock.ValidatorUpdates)
		require.NoError(t, err)

		block := makeBlock(updatedState, updatedState.LastBlockHeight+1)
		blockID := types.BlockID{block.Hash(), block.MakePartSet(testPartSize).Header()}

		updatedState, err = updateState(updatedState, blockID, &block.Header, abciResponses, validatorUpdates)
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
