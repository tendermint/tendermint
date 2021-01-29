package state_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

const (
	// make sure this is the same as in state/store.go
	valSetCheckpointInterval = 100000
)

func TestStoreBootstrap(t *testing.T) {
	stateDB := dbm.NewMemDB()
	stateStore := sm.NewStore(stateDB)
	val, _ := types.RandValidator(true, 10)
	val2, _ := types.RandValidator(true, 10)
	val3, _ := types.RandValidator(true, 10)
	vals := types.NewValidatorSet([]*types.Validator{val, val2, val3})
	bootstrapState := makeRandomStateFromValidatorSet(vals, 100, 100)
	err := stateStore.Bootstrap(bootstrapState)
	require.NoError(t, err)

	// bootstrap should also save the previous validator
	_, err = stateStore.LoadValidators(99)
	require.NoError(t, err)

	_, err = stateStore.LoadValidators(100)
	require.NoError(t, err)

	_, err = stateStore.LoadValidators(101)
	require.NoError(t, err)

	state, err := stateStore.Load()
	require.NoError(t, err)
	require.Equal(t, bootstrapState, state)
}

func TestStoreLoadValidators(t *testing.T) {
	stateDB := dbm.NewMemDB()
	stateStore := sm.NewStore(stateDB)
	val, _ := types.RandValidator(true, 10)
	val2, _ := types.RandValidator(true, 10)
	val3, _ := types.RandValidator(true, 10)
	vals := types.NewValidatorSet([]*types.Validator{val, val2, val3})

	// 1) LoadValidators loads validators using a height where they were last changed
	// Note that only the next validators at height h + 1 are saved
	err := stateStore.Save(makeRandomStateFromValidatorSet(vals, 1, 1))
	require.NoError(t, err)
	err = stateStore.Save(makeRandomStateFromValidatorSet(vals.CopyIncrementProposerPriority(1), 2, 1))
	require.NoError(t, err)
	loadedVals, err := stateStore.LoadValidators(3)
	require.NoError(t, err)
	require.Equal(t, vals.CopyIncrementProposerPriority(3), loadedVals)

	// 2) LoadValidators loads validators using a checkpoint height

	// add a validator set at the checkpoint
	err = stateStore.Save(makeRandomStateFromValidatorSet(vals, valSetCheckpointInterval, 1))
	require.NoError(t, err)

	// check that a request will go back to the last checkpoint
	_, err = stateStore.LoadValidators(valSetCheckpointInterval + 1)
	require.Error(t, err)
	require.Equal(t, fmt.Sprintf("couldn't find validators at height %d (height %d was originally requested): "+
		"value retrieved from db is empty",
		valSetCheckpointInterval, valSetCheckpointInterval+1), err.Error())

	// now save a validator set at that checkpoint
	err = stateStore.Save(makeRandomStateFromValidatorSet(vals, valSetCheckpointInterval-1, 1))
	require.NoError(t, err)

	loadedVals, err = stateStore.LoadValidators(valSetCheckpointInterval)
	require.NoError(t, err)
	// validator set gets updated with the one given hence we expect it to equal next validators (with an increment of one)
	// as opposed to being equal to an increment of 100000 - 1 (if we didn't save at the checkpoint)
	require.Equal(t, vals.CopyIncrementProposerPriority(2), loadedVals)
	require.NotEqual(t, vals.CopyIncrementProposerPriority(valSetCheckpointInterval), loadedVals)
}

// This benchmarks the speed of loading validators from different heights if there is no validator set change.
// NOTE: This isn't too indicative of validator retrieval speed as the db is always (regardless of height) only
// performing two operations: 1) retrieve validator info at height x, which has a last validator set change of 1
// and 2) retrieve the validator set at the aforementioned height 1.
func BenchmarkLoadValidators(b *testing.B) {
	const valSetSize = 100

	config := cfg.ResetTestRoot("state_")
	defer os.RemoveAll(config.RootDir)
	dbType := dbm.BackendType(config.DBBackend)
	stateDB, err := dbm.NewDB("state", dbType, config.DBDir())
	require.NoError(b, err)
	stateStore := sm.NewStore(stateDB)
	state, err := stateStore.LoadFromDBOrGenesisFile(config.GenesisFile())
	if err != nil {
		b.Fatal(err)
	}

	state.Validators = genValSet(valSetSize)
	state.NextValidators = state.Validators.CopyIncrementProposerPriority(1)
	err = stateStore.Save(state)
	require.NoError(b, err)

	for i := 10; i < 10000000000; i *= 10 { // 10, 100, 1000, ...
		i := i
		err = stateStore.Save(makeRandomStateFromValidatorSet(state.NextValidators,
			int64(i)-1, state.LastHeightValidatorsChanged))
		if err != nil {
			b.Fatalf("error saving store: %v", err)
		}

		b.Run(fmt.Sprintf("height=%d", i), func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				_, err := stateStore.LoadValidators(int64(i))
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestStoreLoadConsensusParams(t *testing.T) {
	stateDB := dbm.NewMemDB()
	stateStore := sm.NewStore(stateDB)
	err := stateStore.Save(makeRandomStateFromConsensusParams(types.DefaultConsensusParams(), 1, 1))
	require.NoError(t, err)
	params, err := stateStore.LoadConsensusParams(1)
	require.NoError(t, err)
	require.Equal(t, types.DefaultConsensusParams(), &params)

	// we give the state store different params but say that the height hasn't changed, hence
	// it should save a pointer to the params at height 1
	differentParams := types.DefaultConsensusParams()
	differentParams.Block.MaxBytes = 20000
	err = stateStore.Save(makeRandomStateFromConsensusParams(differentParams, 10, 1))
	require.NoError(t, err)
	res, err := stateStore.LoadConsensusParams(10)
	require.NoError(t, err)
	require.Equal(t, res, params)
	require.NotEqual(t, res, differentParams)
}

func TestPruneStates(t *testing.T) {
	testcases := map[string]struct {
		makeHeights  int64
		pruneHeight  int64
		expectErr    bool
		expectVals   []int64
		expectParams []int64
		expectABCI   []int64
	}{
		"error when prune height is 0":           {100, 0, true, nil, nil, nil},
		"error when prune height is negative":    {100, -10, true, nil, nil, nil},
		"error when prune height does not exist": {100, 101, true, nil, nil, nil},
		"prune all":                              {100, 100, false, []int64{93, 100}, []int64{95, 100}, []int64{100}},
		"prune some": {10, 8, false, []int64{3, 8, 9, 10},
			[]int64{5, 8, 9, 10}, []int64{8, 9, 10}},
		"prune across checkpoint": {100002, 100002, false, []int64{100000, 100002},
			[]int64{99995, 100002}, []int64{100002}},
	}
	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			db := dbm.NewMemDB()
			stateStore := sm.NewStore(db)
			pk := ed25519.GenPrivKey().PubKey()

			// Generate a bunch of state data. Validators change for heights ending with 3, and
			// parameters when ending with 5.
			validator := &types.Validator{Address: tmrand.Bytes(crypto.AddressSize), VotingPower: 100, PubKey: pk}
			validatorSet := &types.ValidatorSet{
				Validators: []*types.Validator{validator},
				Proposer:   validator,
			}
			valsChanged := int64(0)
			paramsChanged := int64(0)

			for h := int64(1); h <= tc.makeHeights; h++ {
				if valsChanged == 0 || h%10 == 2 {
					valsChanged = h + 1 // Have to add 1, since NextValidators is what's stored
				}
				if paramsChanged == 0 || h%10 == 5 {
					paramsChanged = h
				}

				state := sm.State{
					InitialHeight:   1,
					LastBlockHeight: h - 1,
					Validators:      validatorSet,
					NextValidators:  validatorSet,
					ConsensusParams: types.ConsensusParams{
						Block: types.BlockParams{MaxBytes: 10e6},
					},
					LastHeightValidatorsChanged:      valsChanged,
					LastHeightConsensusParamsChanged: paramsChanged,
				}

				if state.LastBlockHeight >= 1 {
					state.LastValidators = state.Validators
				}

				err := stateStore.Save(state)
				require.NoError(t, err)

				err = stateStore.SaveABCIResponses(h, &tmstate.ABCIResponses{
					DeliverTxs: []*abci.ResponseDeliverTx{
						{Data: []byte{1}},
						{Data: []byte{2}},
						{Data: []byte{3}},
					},
				})
				require.NoError(t, err)
			}

			// Test assertions
			err := stateStore.PruneStates(tc.pruneHeight)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			expectVals := sliceToMap(tc.expectVals)
			expectParams := sliceToMap(tc.expectParams)
			expectABCI := sliceToMap(tc.expectABCI)

			for h := int64(1); h <= tc.makeHeights; h++ {
				vals, err := stateStore.LoadValidators(h)
				if expectVals[h] {
					require.NoError(t, err, "validators height %v", h)
					require.NotNil(t, vals)
				} else {
					require.Error(t, err, "validators height %v", h)
					require.Equal(t, sm.ErrNoValSetForHeight{Height: h}, err)
				}

				params, err := stateStore.LoadConsensusParams(h)
				if expectParams[h] {
					require.NoError(t, err, "params height %v", h)
					require.False(t, params.Equals(&types.ConsensusParams{}), "params should not be empty")
				} else {
					require.Error(t, err, "params height %v", h)
				}

				abci, err := stateStore.LoadABCIResponses(h)
				if expectABCI[h] {
					require.NoError(t, err, "abci height %v", h)
					require.NotNil(t, abci)
				} else {
					require.Error(t, err, "abci height %v", h)
					require.Equal(t, sm.ErrNoABCIResponsesForHeight{Height: h}, err)
				}
			}
		})
	}
}

func TestABCIResponsesResultsHash(t *testing.T) {
	responses := &tmstate.ABCIResponses{
		BeginBlock: &abci.ResponseBeginBlock{},
		DeliverTxs: []*abci.ResponseDeliverTx{
			{Code: 32, Data: []byte("Hello"), Log: "Huh?"},
		},
		EndBlock: &abci.ResponseEndBlock{},
	}

	root := sm.ABCIResponsesResultsHash(responses)

	// root should be Merkle tree root of DeliverTxs responses
	results := types.NewResults(responses.DeliverTxs)
	assert.Equal(t, root, results.Hash())

	// test we can prove first DeliverTx
	proof := results.ProveResult(0)
	bz, err := results[0].Marshal()
	require.NoError(t, err)
	assert.NoError(t, proof.Verify(root, bz))
}

func sliceToMap(s []int64) map[int64]bool {
	m := make(map[int64]bool, len(s))
	for _, i := range s {
		m[i] = true
	}
	return m
}
