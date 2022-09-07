//nolint: lll
package state_test

import (
	"bytes"
	"fmt"
	"math/big"
	"os"
	"testing"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/dash/llmq"
	sm "github.com/tendermint/tendermint/internal/state"
	statefactory "github.com/tendermint/tendermint/internal/state/test/factory"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/types"
)

// setupTestCase does setup common to all test cases.
func setupTestCase(t *testing.T) (func(t *testing.T), dbm.DB, sm.State) {
	cfg, err := config.ResetTestRoot(t.TempDir(), "state_")
	require.NoError(t, err)

	dbType := dbm.BackendType(cfg.DBBackend)
	stateDB, err := dbm.NewDB("state", dbType, cfg.DBDir())
	require.NoError(t, err)
	stateStore := sm.NewStore(stateDB)
	state, err := stateStore.Load()
	require.NoError(t, err)
	require.Empty(t, state)
	state, err = sm.MakeGenesisStateFromFile(cfg.GenesisFile())
	assert.NoError(t, err)
	assert.NotNil(t, state)
	err = stateStore.Save(state)
	require.NoError(t, err)

	tearDown := func(t *testing.T) { _ = os.RemoveAll(cfg.RootDir) }

	return tearDown, stateDB, state
}

// TestStateCopy tests the correct copying behavior of State.
func TestStateCopy(t *testing.T) {
	tearDown, _, state := setupTestCase(t)
	defer tearDown(t)

	stateCopy := state.Copy()

	seq, err := state.Equals(stateCopy)
	require.NoError(t, err)
	assert.True(t, seq,
		"expected state and its copy to be identical.\ngot: %v\nexpected: %v",
		stateCopy, state)

	stateCopy.LastBlockHeight++
	stateCopy.LastValidators = state.Validators

	seq, err = state.Equals(stateCopy)
	require.NoError(t, err)
	assert.False(t, seq, "expected states to be different. got same %v", state)
}

// TestMakeGenesisStateNilValidators tests state's consistency when genesis file's validators field is nil.
func TestMakeGenesisStateNilValidators(t *testing.T) {
	doc := types.GenesisDoc{
		ChainID:    "dummy",
		Validators: nil,
	}
	require.Nil(t, doc.ValidateAndComplete())
	state, err := sm.MakeGenesisState(&doc)
	require.NoError(t, err)
	require.Equal(t, 0, len(state.Validators.Validators))
	require.Equal(t, 0, len(state.NextValidators.Validators))
}

// TestStateSaveLoad tests saving and loading State from a db.
func TestStateSaveLoad(t *testing.T) {
	tearDown, stateDB, state := setupTestCase(t)
	defer tearDown(t)
	stateStore := sm.NewStore(stateDB)

	state.LastBlockHeight++
	state.LastValidators = state.Validators
	err := stateStore.Save(state)
	require.NoError(t, err)

	loadedState, err := stateStore.Load()
	require.NoError(t, err)
	seq, err := state.Equals(loadedState)
	require.NoError(t, err)
	assert.True(t, seq,
		"expected state and its copy to be identical.\ngot: %v\nexpected: %v",
		loadedState, state)
}

// TestFinalizeBlockResponsesSaveLoad1 tests saving and loading responses to FinalizeBlock.
func TestFinalizeBlockResponsesSaveLoad1(t *testing.T) {
	tearDown, stateDB, state := setupTestCase(t)
	defer tearDown(t)
	stateStore := sm.NewStore(stateDB)

	state.LastBlockHeight++

	// Build mock responses.
	block, err := statefactory.MakeBlock(state, 2, new(types.Commit), nil, 0)
	require.NoError(t, err)

	dtxs := make([]*abci.ExecTxResult, 2)
	finalizeBlockResponses := new(abci.ResponseFinalizeBlock)
	finalizeBlockResponses.TxResults = dtxs

	finalizeBlockResponses.TxResults[0] = &abci.ExecTxResult{Data: []byte("foo"), Events: nil}
	finalizeBlockResponses.TxResults[1] = &abci.ExecTxResult{Data: []byte("bar"), Log: "ok", Events: nil}
	pubKey := bls12381.GenPrivKey().PubKey()
	abciPubKey, err := cryptoenc.PubKeyToProto(pubKey)
	require.NoError(t, err)

	vu := types.TM2PB.NewValidatorUpdate(pubKey, 100, crypto.RandProTxHash(), types.RandValidatorAddress().String())
	finalizeBlockResponses.ValidatorSetUpdate = &abci.ValidatorSetUpdate{
		ValidatorUpdates:   []abci.ValidatorUpdate{vu},
		ThresholdPublicKey: abciPubKey,
	}

	err = stateStore.SaveFinalizeBlockResponses(block.Height, finalizeBlockResponses)
	require.NoError(t, err)
	loadedFinalizeBlockResponses, err := stateStore.LoadFinalizeBlockResponses(block.Height)
	require.NoError(t, err)
	assert.Equal(t, finalizeBlockResponses, loadedFinalizeBlockResponses,
		"FinalizeBlockResponses don't match:\ngot:       %v\nexpected: %v\n",
		loadedFinalizeBlockResponses, finalizeBlockResponses)
}

// TestFinalizeBlockResponsesSaveLoad2 tests saving and loading responses to FinalizeBlock.
func TestFinalizeBlockResponsesSaveLoad2(t *testing.T) {
	tearDown, stateDB, _ := setupTestCase(t)
	defer tearDown(t)

	stateStore := sm.NewStore(stateDB)

	cases := [...]struct {
		// Height is implied to equal index+2,
		// as block 1 is created from genesis.
		added    []*abci.ExecTxResult
		expected []*abci.ExecTxResult
	}{
		0: {
			nil,
			nil,
		},
		1: {
			[]*abci.ExecTxResult{
				{Code: 32, Data: []byte("Hello"), Log: "Huh?"},
			},
			[]*abci.ExecTxResult{
				{Code: 32, Data: []byte("Hello")},
			},
		},
		2: {
			[]*abci.ExecTxResult{
				{Code: 383},
				{
					Data: []byte("Gotcha!"),
					Events: []abci.Event{
						{Type: "type1", Attributes: []abci.EventAttribute{{Key: "a", Value: "1"}}},
						{Type: "type2", Attributes: []abci.EventAttribute{{Key: "build", Value: "stuff"}}},
					},
				},
			},
			[]*abci.ExecTxResult{
				{Code: 383, Data: nil},
				{Code: 0, Data: []byte("Gotcha!"), Events: []abci.Event{
					{Type: "type1", Attributes: []abci.EventAttribute{{Key: "a", Value: "1"}}},
					{Type: "type2", Attributes: []abci.EventAttribute{{Key: "build", Value: "stuff"}}},
				}},
			},
		},
		3: {
			nil,
			nil,
		},
		4: {
			[]*abci.ExecTxResult{nil},
			nil,
		},
	}

	// Query all before, this should return error.
	for i := range cases {
		h := int64(i + 1)
		res, err := stateStore.LoadFinalizeBlockResponses(h)
		assert.Error(t, err, "%d: %#v", i, res)
	}

	// Add all cases.
	for i, tc := range cases {
		h := int64(i + 1) // last block height, one below what we save
		responses := &abci.ResponseFinalizeBlock{
			TxResults: tc.added,
			AppHash:   []byte("a_hash"),
		}
		err := stateStore.SaveFinalizeBlockResponses(h, responses)
		require.NoError(t, err)
	}

	// Query all after, should return expected value.
	for i, tc := range cases {
		h := int64(i + 1)
		res, err := stateStore.LoadFinalizeBlockResponses(h)
		if assert.NoError(t, err, "%d", i) {
			t.Log(res)
			e, err := abci.MarshalTxResults(tc.expected)
			require.NoError(t, err)
			he := merkle.HashFromByteSlices(e)
			rs, err := abci.MarshalTxResults(res.TxResults)
			hrs := merkle.HashFromByteSlices(rs)
			require.NoError(t, err)
			assert.Equal(t, he, hrs, "%d", i)
		}
	}
}

// TestValidatorSimpleSaveLoad tests saving and loading validators.
func TestValidatorSimpleSaveLoad(t *testing.T) {
	tearDown, stateDB, state := setupTestCase(t)
	defer tearDown(t)

	statestore := sm.NewStore(stateDB)

	// Can't load anything for height 0.
	_, err := statestore.LoadValidators(0)
	assert.IsType(t, sm.ErrNoValSetForHeight{}, err, "expected err at height 0")

	// Should be able to load for height 1.
	v, err := statestore.LoadValidators(1)
	require.NoError(t, err, "expected no err at height 1")
	assert.Equal(t, v.Hash(), state.Validators.Hash(), "expected validator hashes to match")

	// Should be able to load for height 2.
	v, err = statestore.LoadValidators(2)
	require.NoError(t, err, "expected no err at height 2")
	assert.Equal(t, v.Hash(), state.NextValidators.Hash(), "expected validator hashes to match")

	// Increment height, save; should be able to load for next & next next height.
	state.LastBlockHeight++
	nextHeight := state.LastBlockHeight + 1
	err = statestore.Save(state)
	require.NoError(t, err)
	vp0, err := statestore.LoadValidators(nextHeight + 0)
	assert.NoError(t, err)
	vp1, err := statestore.LoadValidators(nextHeight + 1)
	assert.NoError(t, err)
	assert.Equal(t, vp0.Hash(), state.Validators.Hash(), "expected validator hashes to match")
	assert.Equal(t, vp1.Hash(), state.NextValidators.Hash(), "expected next validator hashes to match")
}

// TestValidatorChangesSaveLoad tests saving and loading a validator set with changes.
func TestOneValidatorChangesSaveLoad(t *testing.T) {
	tearDown, stateDB, state := setupTestCase(t)
	defer tearDown(t)
	stateStore := sm.NewStore(stateDB)

	// Change vals at these heights.
	changeHeights := []int64{1, 2, 4, 5, 10, 15, 16, 17, 20}
	N := len(changeHeights)

	// Build the validator history by running updateState
	// with the right validator set for each height.
	highestHeight := changeHeights[N-1] + 5
	changeIndex := 0

	testCases := make([]crypto.PubKey, highestHeight-1)
	for i := int64(1); i < highestHeight; i++ {
		// When we get to a change height, use the next pubkey.
		if changeIndex < len(changeHeights) && i == changeHeights[changeIndex] {
			changeIndex++
		}
		header, _, blockID, responses := makeHeaderPartsResponsesValPowerChange(t, state, types.DefaultDashVotingPower)
		validatorUpdates, thresholdPubKey, quorumHash, err := types.PB2TM.ValidatorUpdatesFromValidatorSet(responses.ValidatorSetUpdate)
		require.NoError(t, err)
		rs, err := abci.MarshalTxResults(responses.TxResults)
		require.NoError(t, err)
		// Any node pro tx hash should do
		firstNodeProTxHash, _ := state.Validators.GetByIndex(0)
		h := merkle.HashFromByteSlices(rs)
		state, err = state.Update(firstNodeProTxHash, blockID, &header, h, responses.ConsensusParamUpdates, validatorUpdates, thresholdPubKey, quorumHash)
		require.NoError(t, err)
		validator := state.Validators.Validators[0]
		testCases[i-1] = validator.PubKey
		err = stateStore.Save(state)
		require.NoError(t, err)
	}

	for i, pubKey := range testCases {
		v, err := stateStore.LoadValidators(int64(i + 1 + 1)) // +1 because vset changes delayed by 1 block.
		assert.NoError(t, err, fmt.Sprintf("expected no err at height %d", i))
		assert.Equal(t, v.Size(), 1, "validator set size is greater than 1: %d", v.Size())
		_, val := v.GetByIndex(0)

		assert.Equal(t, val.PubKey, pubKey, fmt.Sprintf(`unexpected pubKey
                height %d`, i))
	}
}

//func TestProposerFrequency(t *testing.T) {
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	// some explicit test cases
//	testCases := []struct {
//		powers []int64
//	}{
//		// 2 vals
//		{[]int64{1, 1}},
//		{[]int64{1, 2}},
//		{[]int64{1, 100}},
//		{[]int64{5, 5}},
//		{[]int64{5, 100}},
//		{[]int64{50, 50}},
//		{[]int64{50, 100}},
//		{[]int64{1, 1000}},
//
//		// 3 vals
//		{[]int64{1, 1, 1}},
//		{[]int64{1, 2, 3}},
//		{[]int64{1, 2, 3}},
//		{[]int64{1, 1, 10}},
//		{[]int64{1, 1, 100}},
//		{[]int64{1, 10, 100}},
//		{[]int64{1, 1, 1000}},
//		{[]int64{1, 10, 1000}},
//		{[]int64{1, 100, 1000}},
//
//		// 4 vals
//		{[]int64{1, 1, 1, 1}},
//		{[]int64{1, 2, 3, 4}},
//		{[]int64{1, 1, 1, 10}},
//		{[]int64{1, 1, 1, 100}},
//		{[]int64{1, 1, 1, 1000}},
//		{[]int64{1, 1, 10, 100}},
//		{[]int64{1, 1, 10, 1000}},
//		{[]int64{1, 1, 100, 1000}},
//		{[]int64{1, 10, 100, 1000}},
//	}
//
//	for caseNum, testCase := range testCases {
//		// run each case 5 times to sample different
//		// initial priorities
//		for i := 0; i < 5; i++ {
//			valSet := genValSetWithPowers(testCase.powers)
//			testProposerFreq(t, caseNum, valSet)
//		}
//	}
//
//	// some random test cases with up to 100 validators
//	maxVals := 100
//	maxPower := 1000
//	nTestCases := 5
//	for i := 0; i < nTestCases; i++ {
//		N := mrand.Int()%maxVals + 1
//		vals := make([]*types.Validator, N)
//		totalVotePower := int64(0)
//		for j := 0; j < N; j++ {
//			// make sure votePower > 0
//			votePower := int64(mrand.Int()%maxPower) + 1
//			totalVotePower += votePower
//			privVal := types.NewMockPV()
//			pubKey, err := privVal.GetPubKey(ctx)
//			require.NoError(t, err)
//			val := types.NewValidator(pubKey, votePower)
//			val.ProposerPriority = mrand.Int63()
//			vals[j] = val
//		}
//		valSet := types.NewValidatorSet(vals)
//		valSet.RescalePriorities(totalVotePower)
//		testProposerFreq(t, i, valSet)
//	}
//}
//
//// new val set with given powers and random initial priorities
//func genValSetWithPowers(powers []int64) *types.ValidatorSet {
//	size := len(powers)
//	vals := make([]*types.Validator, size)
//	totalVotePower := int64(0)
//	for i := 0; i < size; i++ {
//		totalVotePower += powers[i]
//		val := types.NewValidator(ed25519.GenPrivKey().PubKey(), powers[i])
//		val.ProposerPriority = mrand.Int63()
//		vals[i] = val
//	}
//	valSet := types.NewValidatorSet(vals)
//	valSet.RescalePriorities(totalVotePower)
//	return valSet
//}
//
//// test a proposer appears as frequently as expected
//func testProposerFreq(t *testing.T, caseNum int, valSet *types.ValidatorSet) {
//	N := valSet.Size()
//	totalPower := valSet.TotalVotingPower()
//
//	// run the proposer selection and track frequencies
//	runMult := 1
//	runs := int(totalPower) * runMult
//	freqs := make([]int, N)
//	for i := 0; i < runs; i++ {
//		prop := valSet.GetProposer()
//		idx, _ := valSet.GetByAddress(prop.Address)
//		freqs[idx]++
//		valSet.IncrementProposerPriority(1)
//	}
//
//	// assert frequencies match expected (max off by 1)
//	for i, freq := range freqs {
//		_, val := valSet.GetByIndex(int32(i))
//		expectFreq := int(val.VotingPower) * runMult
//		gotFreq := freq
//		abs := int(math.Abs(float64(expectFreq - gotFreq)))
//
//		// max bound on expected vs seen freq was proven
//		// to be 1 for the 2 validator case in
//		// https://github.com/cwgoes/tm-proposer-idris
//		// and inferred to generalize to N-1
//		bound := N - 1
//		require.True(
//			t,
//			abs <= bound,
//			fmt.Sprintf("Case %d val %d (%d): got %d, expected %d", caseNum, i, N, gotFreq, expectFreq),
//		)
//	}
//}

// TestProposerPriorityDoesNotGetResetToZero assert that we preserve accum when calling updateState
// see https://github.com/tendermint/tendermint/issues/2718
func TestProposerPriorityDoesNotGetResetToZero(t *testing.T) {
	tearDown, _, state := setupTestCase(t)
	defer tearDown(t)

	ld := llmq.MustGenerate(crypto.RandProTxHashes(2))

	val1VotingPower := types.DefaultDashVotingPower
	val1ProTxHash := ld.ProTxHashes[0]
	val1PubKey := ld.PubKeyShares[0]
	val1 := &types.Validator{ProTxHash: val1ProTxHash, PubKey: val1PubKey, VotingPower: val1VotingPower}

	quorumHash := crypto.RandQuorumHash()
	state.Validators = types.NewValidatorSet([]*types.Validator{val1}, val1PubKey, btcjson.LLMQType_5_60, quorumHash, true)
	state.NextValidators = state.Validators

	// NewValidatorSet calls IncrementProposerPriority but uses on a copy of val1
	assert.EqualValues(t, 0, val1.ProposerPriority)

	block, err := statefactory.MakeBlock(state, state.LastBlockHeight+1, new(types.Commit), nil, 0)
	require.NoError(t, err)
	blockID, err := block.BlockID()
	require.NoError(t, err)
	fb := &abci.ResponseFinalizeBlock{
		ValidatorSetUpdate: nil,
	}
	validatorUpdates, thresholdPublicKeyUpdate, _, err :=
		types.PB2TM.ValidatorUpdatesFromValidatorSet(nil)
	require.NoError(t, err)
	// Any node pro tx hash should do
	firstNodeProTxHash, _ := state.Validators.GetByIndex(0)
	rs, err := abci.MarshalTxResults(fb.TxResults)
	require.NoError(t, err)
	h := merkle.HashFromByteSlices(rs)
	updatedState, err := state.Update(firstNodeProTxHash, blockID, &block.Header, h, fb.ConsensusParamUpdates, validatorUpdates, thresholdPublicKeyUpdate, quorumHash)
	assert.NoError(t, err)
	curTotal := val1VotingPower
	// one increment step and one validator: 0 + power - total_power == 0
	assert.Equal(t, 0+val1VotingPower-curTotal, updatedState.NextValidators.Validators[0].ProposerPriority)

	// add a validator
	val2ProTxHash := ld.ProTxHashes[1]
	val2PubKey := ld.PubKeyShares[1]
	val2VotingPower := types.DefaultDashVotingPower
	fvp, err := cryptoenc.PubKeyToProto(val2PubKey)
	require.NoError(t, err)

	updateAddVal := abci.ValidatorUpdate{ProTxHash: val2ProTxHash, PubKey: &fvp, Power: val2VotingPower}
	validatorUpdates, err = types.PB2TM.ValidatorUpdates([]abci.ValidatorUpdate{updateAddVal})
	assert.NoError(t, err)
	rs, err = abci.MarshalTxResults(fb.TxResults)
	require.NoError(t, err)
	h = merkle.HashFromByteSlices(rs)
	updatedState2, err := updatedState.Update(firstNodeProTxHash, blockID, &block.Header, h, fb.ConsensusParamUpdates, validatorUpdates, ld.ThresholdPubKey, quorumHash)
	assert.NoError(t, err)

	require.Equal(t, len(updatedState2.NextValidators.Validators), 2)
	_, updatedVal1 := updatedState2.NextValidators.GetByProTxHash(val1ProTxHash)
	_, addedVal2 := updatedState2.NextValidators.GetByProTxHash(val2ProTxHash)

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

	// Updating validators does not reset the ProposerPriority to zero if we keep the same quorum:
	// If we change quorums it will!
	// 1. Add - Val2 VotingPower change to 1 =>
	abciValidatorUpdates := types.ValidatorUpdatesRegenerateOnProTxHashes(ld.ProTxHashes)
	validatorUpdates, thresholdPubKey, _, err := types.PB2TM.ValidatorUpdatesFromValidatorSet(&abciValidatorUpdates)
	require.NoError(t, err)

	// this will cause the diff of priorities (77)
	// to be larger than threshold == 2*totalVotingPower (22):
	rs, err = abci.MarshalTxResults(fb.TxResults)
	require.NoError(t, err)
	h = merkle.HashFromByteSlices(rs)
	updatedState3, err := updatedState2.Update(firstNodeProTxHash, blockID, &block.Header, h, fb.ConsensusParamUpdates, validatorUpdates, thresholdPubKey, quorumHash)
	assert.NoError(t, err)

	require.Equal(t, len(updatedState3.NextValidators.Validators), 2)
	_, prevVal1 := updatedState3.Validators.GetByProTxHash(val1ProTxHash)
	_, prevVal2 := updatedState3.Validators.GetByProTxHash(val2ProTxHash)
	_, updatedVal1 = updatedState3.NextValidators.GetByProTxHash(val1ProTxHash)
	_, updatedVal2 := updatedState3.NextValidators.GetByProTxHash(val2ProTxHash)

	// 2. Scale
	// old prios: v1(100):13, v2(100):-12
	wantVal1Prio = prevVal1.ProposerPriority
	wantVal2Prio = prevVal2.ProposerPriority
	// scale to diffMax = 400 = 2 * tvp, diff=13-(-12)=25
	// new totalPower
	totalPower := updatedVal1.VotingPower + updatedVal2.VotingPower
	dist := wantVal2Prio - wantVal1Prio
	if dist < 0 { // get the absolute distance
		dist *= -1
	}
	// ratio := (dist + 2*totalPower - 1) / 2*totalPower = 224/200 = 1
	ratio := int64(float64(dist+2*totalPower-1) / float64(2*totalPower))
	// v1(100):13/1, v2(100):-12/1
	if ratio != 0 {
		wantVal1Prio /= ratio // 13
		wantVal2Prio /= ratio // -12
	}

	// 3. Center - noop
	// 4. IncrementProposerPriority() ->
	// v1(100):13+100, v2(100):-12+100 -> v2 proposer so subtract tvp(11)
	// v1(100):-87, v2(1):88
	wantVal2Prio += updatedVal2.VotingPower // 88 -> prop
	wantVal1Prio += updatedVal1.VotingPower // 113
	wantVal1Prio -= totalPower              // -87

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

	ld := llmq.MustGenerate(crypto.RandProTxHashes(2))

	val1VotingPower := types.DefaultDashVotingPower
	val1ProTxHash := ld.ProTxHashes[0]
	val1PubKey := ld.PubKeyShares[0]
	val1 := &types.Validator{ProTxHash: val1ProTxHash, PubKey: val1PubKey, VotingPower: val1VotingPower}

	// reset state validators to above validator, the threshold key is just the validator key since there is only 1 validator
	quorumHash := crypto.RandQuorumHash()
	state.Validators = types.NewValidatorSet([]*types.Validator{val1}, val1PubKey, btcjson.LLMQType_5_60, quorumHash, true)
	state.NextValidators = state.Validators
	// we only have one validator:
	assert.Equal(t, val1ProTxHash, state.Validators.Proposer.ProTxHash)

	block, err := statefactory.MakeBlock(state, state.LastBlockHeight+1, new(types.Commit), nil, 0)
	require.NoError(t, err)
	blockID, err := block.BlockID()
	require.NoError(t, err)
	// no updates:
	fb := &abci.ResponseFinalizeBlock{
		ValidatorSetUpdate: nil,
	}
	validatorUpdates, thresholdPublicKeyUpdate, _, err :=
		types.PB2TM.ValidatorUpdatesFromValidatorSet(nil)
	require.NoError(t, err)

	rs, err := abci.MarshalTxResults(fb.TxResults)
	require.NoError(t, err)
	h := merkle.HashFromByteSlices(rs)

	// Any node pro tx hash should do
	firstNodeProTxHash, _ := state.Validators.GetByIndex(0)

	updatedState, err := state.Update(firstNodeProTxHash, blockID, &block.Header, h, fb.ConsensusParamUpdates, validatorUpdates, thresholdPublicKeyUpdate, quorumHash)
	assert.NoError(t, err)

	// 0 + 10 (initial prio) - 10 (avg) - 10 (mostest - total) = -10
	totalPower := val1VotingPower
	wantVal1Prio := 0 + val1VotingPower - totalPower
	assert.Equal(t, wantVal1Prio, updatedState.NextValidators.Validators[0].ProposerPriority)
	assert.Equal(t, val1ProTxHash, updatedState.NextValidators.Proposer.ProTxHash)

	// add a validator with the same voting power as the first
	val2ProTxHash := ld.ProTxHashes[1]
	val2PubKey := ld.PubKeyShares[1]
	fvp, err := cryptoenc.PubKeyToProto(val2PubKey)
	require.NoError(t, err)
	updateAddVal := abci.ValidatorUpdate{ProTxHash: val2ProTxHash, PubKey: &fvp, Power: val1VotingPower}
	validatorUpdates, err = types.PB2TM.ValidatorUpdates([]abci.ValidatorUpdate{updateAddVal})
	assert.NoError(t, err)

	rs, err = abci.MarshalTxResults(fb.TxResults)
	require.NoError(t, err)
	h = merkle.HashFromByteSlices(rs)
	updatedState2, err := updatedState.Update(firstNodeProTxHash, blockID, &block.Header, h, fb.ConsensusParamUpdates, validatorUpdates, ld.ThresholdPubKey, quorumHash)
	assert.NoError(t, err)

	require.Equal(t, len(updatedState2.NextValidators.Validators), 2)
	assert.Equal(t, updatedState2.Validators, updatedState.NextValidators)

	// val1 will still be proposer as val2 just got added:
	assert.Equal(t, val1ProTxHash, updatedState.NextValidators.Proposer.ProTxHash)
	assert.Equal(t, updatedState2.Validators.Proposer.ProTxHash, updatedState2.NextValidators.Proposer.ProTxHash)
	assert.Equal(t, updatedState2.Validators.Proposer.ProTxHash, val1ProTxHash)
	assert.Equal(t, updatedState2.NextValidators.Proposer.ProTxHash, val1ProTxHash)

	_, updatedVal1 := updatedState2.NextValidators.GetByProTxHash(val1ProTxHash)
	_, oldVal1 := updatedState2.Validators.GetByProTxHash(val1ProTxHash)
	_, updatedVal2 := updatedState2.NextValidators.GetByProTxHash(val2ProTxHash)

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
	assert.EqualValues(
		t,
		expectedVal2Prio,
		updatedVal2.ProposerPriority,
		"unexpected proposer priority for validator: %v",
		updatedVal2,
	)

	validatorUpdates, thresholdPublicKeyUpdate, quorumHash, err =
		types.PB2TM.ValidatorUpdatesFromValidatorSet(fb.ValidatorSetUpdate)
	require.NoError(t, err)

	rs, err = abci.MarshalTxResults(fb.TxResults)
	require.NoError(t, err)
	h = merkle.HashFromByteSlices(rs)
	updatedState3, err := updatedState2.Update(firstNodeProTxHash, blockID, &block.Header, h, fb.ConsensusParamUpdates, validatorUpdates, thresholdPublicKeyUpdate, quorumHash)
	assert.NoError(t, err)

	assert.Equal(t, updatedState3.Validators.Proposer.ProTxHash, updatedState3.NextValidators.Proposer.ProTxHash)

	assert.Equal(t, updatedState3.Validators, updatedState2.NextValidators)
	_, updatedVal1 = updatedState3.NextValidators.GetByProTxHash(val1ProTxHash)
	_, updatedVal2 = updatedState3.NextValidators.GetByProTxHash(val2ProTxHash)

	// val1 will still be proposer:
	assert.Equal(t, val1ProTxHash, updatedState3.NextValidators.Proposer.ProTxHash)

	// check if expected proposer prio is matched:
	// Increment
	expectedVal2Prio2 := expectedVal2Prio + val2VotingPower // -1 + 10 = 9
	expectedVal1Prio2 := expectedVal1Prio + val1VotingPower // 1 + 10 == 11
	expectedVal1Prio2 -= totalPower                         // -9, val1 proposer

	assert.EqualValues(
		t,
		expectedVal1Prio2,
		updatedVal1.ProposerPriority,
		"unexpected proposer priority for validator: %v",
		updatedVal2,
	)
	assert.EqualValues(
		t,
		expectedVal2Prio2,
		updatedVal2.ProposerPriority,
		"unexpected proposer priority for validator: %v",
		updatedVal2,
	)

	// no changes in voting power and both validators have same voting power
	// -> proposers should alternate:
	oldState := updatedState3
	fb = &abci.ResponseFinalizeBlock{
		ValidatorSetUpdate: nil,
	}
	validatorUpdates, thresholdPublicKeyUpdate, quorumHash, err =
		types.PB2TM.ValidatorUpdatesFromValidatorSet(fb.ValidatorSetUpdate)
	require.NoError(t, err)

	rs, err = abci.MarshalTxResults(fb.TxResults)
	require.NoError(t, err)
	h = merkle.HashFromByteSlices(rs)
	oldState, err = oldState.Update(firstNodeProTxHash, blockID, &block.Header, h, fb.ConsensusParamUpdates, validatorUpdates, thresholdPublicKeyUpdate, quorumHash)
	assert.NoError(t, err)
	expectedVal1Prio2 = 13
	expectedVal2Prio2 = -12
	expectedVal1Prio = -87
	expectedVal2Prio = 88

	for i := 0; i < 1000; i++ {
		// no validator updates:
		fb := &abci.ResponseFinalizeBlock{
			ValidatorSetUpdate: nil,
		}
		validatorUpdates, thresholdPublicKeyUpdate, quorumHash, err =
			types.PB2TM.ValidatorUpdatesFromValidatorSet(fb.ValidatorSetUpdate)
		require.NoError(t, err)

		rs, err := abci.MarshalTxResults(fb.TxResults)
		require.NoError(t, err)
		h := merkle.HashFromByteSlices(rs)
		updatedState, err := oldState.Update(firstNodeProTxHash, blockID, &block.Header, h, fb.ConsensusParamUpdates, validatorUpdates, thresholdPublicKeyUpdate, quorumHash)
		assert.NoError(t, err)
		// alternate (and cyclic priorities):
		assert.NotEqual(
			t,
			updatedState.Validators.Proposer.ProTxHash,
			updatedState.NextValidators.Proposer.ProTxHash,
			"iter: %v",
			i,
		)
		assert.Equal(t, oldState.Validators.Proposer.ProTxHash, updatedState.NextValidators.Proposer.ProTxHash, "iter: %v", i)

		_, updatedVal1 = updatedState.NextValidators.GetByProTxHash(val1ProTxHash)
		_, updatedVal2 = updatedState.NextValidators.GetByProTxHash(val2ProTxHash)

		if i%2 == 0 {
			assert.Equal(t, updatedState.Validators.Proposer.ProTxHash, val2ProTxHash)
			assert.Equal(t, expectedVal1Prio, updatedVal1.ProposerPriority) // -19
			assert.Equal(t, expectedVal2Prio, updatedVal2.ProposerPriority) // 0
		} else {
			assert.Equal(t, updatedState.Validators.Proposer.ProTxHash, val1ProTxHash)
			assert.Equal(t, expectedVal1Prio2, updatedVal1.ProposerPriority) // -9
			assert.Equal(t, expectedVal2Prio2, updatedVal2.ProposerPriority) // -10
		}
		// update for next iteration:
		oldState = updatedState
	}
}

func TestFourAddFourMinusOneGenesisValidators(t *testing.T) {
	tearDown, _, state := setupTestCase(t)
	defer tearDown(t)

	originalValidatorSet, _ := types.RandValidatorSet(4)
	// reset state validators to above validator
	state.Validators = originalValidatorSet
	state.NextValidators = originalValidatorSet

	// Any node pro tx hash should do
	firstProTxHash, _ := state.Validators.GetByIndex(0)

	execute := blockExecutorFunc(t, firstProTxHash)

	// All operations will be on same quorum hash
	quorumHash := crypto.RandQuorumHash()
	quorumHashOpt := abci.WithQuorumHash(quorumHash)

	// update state a few times with no validator updates
	// asserts that the single validator's ProposerPrio stays the same
	oldState := state
	for i := 0; i < 10; i++ {
		// no updates:
		updatedState := execute(oldState, oldState, nil)
		// no changes in voting power (ProposerPrio += VotingPower == Voting in 1st round; than shiftByAvg == 0,
		// than -Total == -Voting)
		// -> no change in ProposerPrio (stays zero):
		assert.EqualValues(t, oldState.NextValidators.GetProTxHashesOrdered(), updatedState.NextValidators.GetProTxHashesOrdered())
		oldState = updatedState
	}

	addedProTxHashes := crypto.RandProTxHashes(4)
	proTxHashes := append(originalValidatorSet.GetProTxHashes(), addedProTxHashes...)
	abciValidatorUpdates0 := types.ValidatorUpdatesRegenerateOnProTxHashes(proTxHashes)
	updatedState := execute(state, state, &abciValidatorUpdates0)

	lastState := updatedState
	for i := 0; i < 200; i++ {
		lastState = execute(lastState, lastState, nil)
	}
	// set state to last state of above iteration
	state = lastState

	// set oldState to state before above iteration
	oldState = updatedState

	// we will keep the same quorum hash as to be able to add validators

	// add 10 validators with the same voting power as the one added directly after genesis:

	for i := 0; i < 5; i++ {
		ld := llmq.MustGenerate(append(proTxHashes, crypto.RandProTxHash()))
		abciValidatorSetUpdate, err := abci.LLMQToValidatorSetProto(*ld, quorumHashOpt)
		require.NoError(t, err)
		state = execute(oldState, state, abciValidatorSetUpdate)
		assertLLMQDataWithValidatorSet(t, ld, state.NextValidators)
		proTxHashes = ld.ProTxHashes
	}

	ld := llmq.MustGenerate(append(proTxHashes, crypto.RandProTxHashes(5)...))
	abciValidatorSetUpdate, err := abci.LLMQToValidatorSetProto(*ld, quorumHashOpt)
	require.NoError(t, err)
	state = execute(oldState, state, abciValidatorSetUpdate)
	assertLLMQDataWithValidatorSet(t, ld, state.NextValidators)

	abciValidatorSetUpdate.ValidatorUpdates[0] = abci.ValidatorUpdate{ProTxHash: ld.ProTxHashes[0]}
	updatedState = execute(oldState, state, abciValidatorSetUpdate)

	// only the first added val (not the genesis val) should be left
	ld.ProTxHashes = ld.ProTxHashes[1:]
	assertLLMQDataWithValidatorSet(t, ld, updatedState.NextValidators)

	abciValidatorSetUpdate.ValidatorUpdates = []abci.ValidatorUpdate{
		{ProTxHash: ld.ProTxHashes[0]},
		{ProTxHash: ld.ProTxHashes[1]},
	}
	updatedState = execute(state, updatedState, abciValidatorSetUpdate)

	// the second and third should be left
	ld.ProTxHashes = ld.ProTxHashes[2:]
	assertLLMQDataWithValidatorSet(t, ld, updatedState.NextValidators)

	updatedState = execute(updatedState, updatedState, nil)
	// store proposers here to see if we see them again in the same order:
	numVals := len(updatedState.Validators.Validators)
	proposers := make([]*types.Validator, numVals)
	for i := 0; i < 100; i++ {
		updatedState = execute(state, updatedState, nil)
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
	t.Cleanup(func() { tearDown(t) })
	stateStore := sm.NewStore(stateDB)
	state.Validators, _ = types.RandValidatorSet(valSetSize)
	state.NextValidators = state.Validators.CopyIncrementProposerPriority(1)
	err := stateStore.Save(state)
	require.NoError(t, err)

	nextHeight := state.LastBlockHeight + 1

	v0, err := stateStore.LoadValidators(nextHeight)
	assert.NoError(t, err)
	acc0 := v0.Validators[0].ProposerPriority

	v1, err := stateStore.LoadValidators(nextHeight + 1)
	assert.NoError(t, err)
	acc1 := v1.Validators[0].ProposerPriority

	assert.NotEqual(t, acc1, acc0, "expected ProposerPriority value to change between heights")
}

// TestValidatorChangesSaveLoad tests saving and loading a validator set with
// changes.
func TestManyValidatorChangesSaveLoad(t *testing.T) {
	const valSetSize = 7
	tearDown, stateDB, state := setupTestCase(t)
	defer tearDown(t)
	stateStore := sm.NewStore(stateDB)
	require.Equal(t, int64(0), state.LastBlockHeight)
	state.Validators, _ = types.RandValidatorSet(valSetSize)
	state.NextValidators = state.Validators.CopyIncrementProposerPriority(1)
	err := stateStore.Save(state)
	require.NoError(t, err)

	firstNodeProTxHash, val0 := state.Validators.GetByIndex(0)
	proTxHash := val0.ProTxHash // this is not really old, as it stays the same
	oldPubkey := val0.PubKey

	// Swap the first validator with a new one (validator set size stays the same).
	header, _, blockID, responses := makeHeaderPartsResponsesValKeysRegenerate(t, state, true)

	// Save state etc.
	var validatorUpdates []*types.Validator
	validatorUpdates, thresholdPublicKeyUpdate, quorumHash, err :=
		types.PB2TM.ValidatorUpdatesFromValidatorSet(responses.ValidatorSetUpdate)
	require.NoError(t, err)
	rs, err := abci.MarshalTxResults(responses.TxResults)
	require.NoError(t, err)
	h := merkle.HashFromByteSlices(rs)
	state, err = state.Update(firstNodeProTxHash, blockID, &header, h, responses.ConsensusParamUpdates, validatorUpdates,
		thresholdPublicKeyUpdate, quorumHash)
	require.NoError(t, err)
	nextHeight := state.LastBlockHeight + 1
	err = stateStore.Save(state)
	require.NoError(t, err)

	var newPubkey crypto.PubKey
	for _, valUpdate := range validatorUpdates {
		if bytes.Equal(valUpdate.ProTxHash.Bytes(), proTxHash.Bytes()) {
			newPubkey = valUpdate.PubKey
		}
	}

	// Load nextheight, it should be the oldpubkey.
	v0, err := stateStore.LoadValidators(nextHeight)
	assert.NoError(t, err)
	assert.Equal(t, valSetSize, v0.Size())
	index, val := v0.GetByProTxHash(proTxHash)
	assert.NotNil(t, val)
	assert.Equal(t, val.PubKey, oldPubkey, "the public key should match the old public key")
	if index < 0 {
		t.Fatal("expected to find old validator")
	}

	// Load nextheight+1, it should be the new pubkey.
	v1, err := stateStore.LoadValidators(nextHeight + 1)
	assert.NoError(t, err)
	assert.Equal(t, valSetSize, v1.Size())
	index, val = v1.GetByProTxHash(proTxHash)
	assert.NotNil(t, val)
	assert.Equal(t, val.PubKey, newPubkey, "the public key should match the regenerated public key")
	if index < 0 {
		t.Fatal("expected to find same validator by new address")
	}
}

func TestStateMakeBlock(t *testing.T) {
	tearDown, _, state := setupTestCase(t)
	defer tearDown(t)

	proposerProTxHash := state.Validators.GetProposer().ProTxHash
	stateVersion := state.Version.Consensus
	var height int64 = 2
	state.LastBlockHeight = height - 1
	block, err := statefactory.MakeBlock(state, height, new(types.Commit), nil, 0)
	require.NoError(t, err)

	// test we set some fields
	assert.Equal(t, stateVersion, block.Version)
	assert.Equal(t, proposerProTxHash, block.ProposerProTxHash)
}

// TestConsensusParamsChangesSaveLoad tests saving and loading consensus params
// with changes.
func TestConsensusParamsChangesSaveLoad(t *testing.T) {
	tearDown, stateDB, state := setupTestCase(t)
	defer tearDown(t)

	stateStore := sm.NewStore(stateDB)

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
	var (
		validatorUpdates         []*types.Validator
		thresholdPublicKeyUpdate crypto.PubKey
		quorumHash               crypto.QuorumHash
		err                      error
	)
	for i := int64(1); i < highestHeight; i++ {
		// When we get to a change height, use the next params.
		if changeIndex < len(changeHeights) && i == changeHeights[changeIndex] {
			changeIndex++
			cp = params[changeIndex]
		}
		header, _, blockID, fpResp := makeHeaderPartsResponsesParams(t, state, &cp)
		validatorUpdates, thresholdPublicKeyUpdate, quorumHash, err = types.PB2TM.ValidatorUpdatesFromValidatorSet(fpResp.ValidatorSetUpdate)
		require.NoError(t, err)
		rs, err := abci.MarshalTxResults(fpResp.TxResults)
		require.NoError(t, err)
		h := merkle.HashFromByteSlices(rs)

		// Any node pro tx hash should do
		firstNodeProTxHash, _ := state.Validators.GetByIndex(0)
		state, err = state.Update(firstNodeProTxHash, blockID, &header, h, fpResp.ConsensusParamUpdates, validatorUpdates, thresholdPublicKeyUpdate, quorumHash)

		require.NoError(t, err)
		err = stateStore.Save(state)
		require.NoError(t, err)
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
		p, err := stateStore.LoadConsensusParams(testCase.height)

		assert.NoError(t, err, fmt.Sprintf("expected no err at height %d", testCase.height))
		assert.EqualValues(t, testCase.params, p, fmt.Sprintf(`unexpected consensus params at
                height %d`, testCase.height))
	}
}

func TestStateProto(t *testing.T) {
	tearDown, _, state := setupTestCase(t)
	defer tearDown(t)

	tc := []struct {
		testName string
		state    *sm.State
		expPass1 bool
		expPass2 bool
	}{
		{"empty state", &sm.State{}, true, false},
		{"nil failure state", nil, false, false},
		{"success state", &state, true, true},
	}

	for _, tt := range tc {
		tt := tt
		pbs, err := tt.state.ToProto()
		if !tt.expPass1 {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err, tt.testName)
		}

		smt, err := sm.FromProto(pbs)
		if tt.expPass2 {
			require.NoError(t, err, tt.testName)
			require.Equal(t, tt.state, smt, tt.testName)
		} else {
			require.Error(t, err, tt.testName)
		}
	}
}

func TestState_StateID(t *testing.T) {

	state := sm.State{
		LastBlockHeight: 2,
	}
	state.AppHash = make([]byte, crypto.DefaultAppHashSize)
	want := tmrand.Bytes(32)
	copy(state.AppHash, want)

	stateID := state.StateID()
	assert.Equal(t, int64(2), stateID.Height)
	assert.EqualValues(t, want, stateID.LastAppHash)

	err := stateID.ValidateBasic()
	assert.NoError(t, err, "StateID validation failed")
}

func blockExecutorFunc(t *testing.T, firstProTxHash crypto.ProTxHash) func(prevState, state sm.State, vsu *abci.ValidatorSetUpdate) sm.State {
	return func(prevState, state sm.State, vsu *abci.ValidatorSetUpdate) sm.State {
		t.Helper()
		fpResp := &abci.ResponseFinalizeBlock{ValidatorSetUpdate: vsu}
		validatorUpdates, thresholdPubKey, quorumHash, err :=
			types.PB2TM.ValidatorUpdatesFromValidatorSet(fpResp.ValidatorSetUpdate)
		require.NoError(t, err)
		block, err := statefactory.MakeBlock(prevState, prevState.LastBlockHeight+1, new(types.Commit), nil, 0)
		require.NoError(t, err)
		blockID, err := block.BlockID()
		require.NoError(t, err)
		rs, err := abci.MarshalTxResults(fpResp.TxResults)
		require.NoError(t, err)
		h := merkle.HashFromByteSlices(rs)
		state, err = state.Update(firstProTxHash, blockID, &block.Header, h, fpResp.ConsensusParamUpdates,
			validatorUpdates, thresholdPubKey, quorumHash)
		require.NoError(t, err)
		return state
	}
}

func assertLLMQDataWithValidatorSet(t *testing.T, ld *llmq.Data, valSet *types.ValidatorSet) {
	require.Equal(t, len(ld.ProTxHashes), len(valSet.Validators))
	m := make(map[string]struct{})
	for _, proTxHash := range ld.ProTxHashes {
		m[proTxHash.String()] = struct{}{}
	}
	for _, val := range valSet.Validators {
		_, ok := m[val.ProTxHash.String()]
		require.True(t, ok)
	}
	require.Equal(t, ld.ThresholdPubKey, valSet.ThresholdPublicKey)
}
