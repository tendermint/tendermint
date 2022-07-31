package state_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/test/factory"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

const (
	// make sure this is the same as in state/store.go
	valSetCheckpointInterval = 100000
)

func TestStoreBootstrap(t *testing.T) {
	stateDB := dbm.NewMemDB()
	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardABCIResponses: false,
	})
	val, _ := factory.RandValidator(true, 10)
	val2, _ := factory.RandValidator(true, 10)
	val3, _ := factory.RandValidator(true, 10)
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
	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardABCIResponses: false,
	})
	val, _ := factory.RandValidator(true, 10)
	val2, _ := factory.RandValidator(true, 10)
	val3, _ := factory.RandValidator(true, 10)
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

	cfg, err := config.ResetTestRoot("state_")
	require.NoError(b, err)

	defer os.RemoveAll(cfg.RootDir)
	dbType := dbm.BackendType(cfg.DBBackend)
	stateDB, err := dbm.NewDB("state", dbType, cfg.DBDir())
	require.NoError(b, err)
	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardABCIResponses: false,
	})
	state, err := sm.MakeGenesisStateFromFile(cfg.GenesisFile())
	if err != nil {
		b.Fatal(err)
	}

	state.Validators = genValSet(valSetSize)
	state.NextValidators = state.Validators.CopyIncrementProposerPriority(1)
	err = stateStore.Save(state)
	require.NoError(b, err)

	b.ResetTimer()

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
	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardABCIResponses: false,
	})
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
		startHeight           int64
		endHeight             int64
		pruneHeight           int64
		expectErr             bool
		remainingValSetHeight int64
		remainingParamsHeight int64
	}{
		"error when prune height is 0":           {1, 100, 0, true, 0, 0},
		"error when prune height is negative":    {1, 100, -10, true, 0, 0},
		"error when prune height does not exist": {1, 100, 101, true, 0, 0},
		"prune all":                              {1, 100, 100, false, 93, 95},
		"prune from non 1 height":                {10, 50, 40, false, 33, 35},
		"prune some":                             {1, 10, 8, false, 3, 5},
		// we test this because we flush to disk every 1000 "states"
		"prune more than 1000 state": {1, 1010, 1010, false, 1003, 1005},
		"prune across checkpoint":    {99900, 100002, 100002, false, 100000, 99995},
	}
	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			db := dbm.NewMemDB()

			stateStore := sm.NewStore(db, sm.StoreOptions{
				DiscardABCIResponses: false,
			})
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

			for h := tc.startHeight; h <= tc.endHeight; h++ {
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

			for h := tc.pruneHeight; h <= tc.endHeight; h++ {
				vals, err := stateStore.LoadValidators(h)
				require.NoError(t, err, h)
				require.NotNil(t, vals, h)

				params, err := stateStore.LoadConsensusParams(h)
				require.NoError(t, err, h)
				require.NotNil(t, params, h)

				abci, err := stateStore.LoadABCIResponses(h)
				require.NoError(t, err, h)
				require.NotNil(t, abci, h)
			}

			emptyParams := types.ConsensusParams{}

			for h := tc.startHeight; h < tc.pruneHeight; h++ {
				vals, err := stateStore.LoadValidators(h)
				if h == tc.remainingValSetHeight {
					require.NoError(t, err, h)
					require.NotNil(t, vals, h)
				} else {
					require.Error(t, err, h)
					require.Nil(t, vals, h)
				}

				params, err := stateStore.LoadConsensusParams(h)
				if h == tc.remainingParamsHeight {
					require.NoError(t, err, h)
					require.NotEqual(t, emptyParams, params, h)
				} else {
					require.Error(t, err, h)
					require.Equal(t, emptyParams, params, h)
				}

				abci, err := stateStore.LoadABCIResponses(h)
				require.Error(t, err, h)
				require.Nil(t, abci, h)
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

func TestLastABCIResponses(t *testing.T) {
	// create an empty state store.
	t.Run("Not persisting responses", func(t *testing.T) {
		stateDB := dbm.NewMemDB()
		stateStore := sm.NewStore(stateDB, sm.StoreOptions{
			DiscardABCIResponses: false,
		})
		responses, err := stateStore.LoadABCIResponses(1)
		require.Error(t, err)
		require.Nil(t, responses)
		// stub the abciresponses.
		response1 := &tmstate.ABCIResponses{
			BeginBlock: &abci.ResponseBeginBlock{},
			DeliverTxs: []*abci.ResponseDeliverTx{
				{Code: 32, Data: []byte("Hello"), Log: "Huh?"},
			},
			EndBlock: &abci.ResponseEndBlock{},
		}
		// create new db and state store and set discard abciresponses to false.
		stateDB = dbm.NewMemDB()
		stateStore = sm.NewStore(stateDB, sm.StoreOptions{DiscardABCIResponses: false})
		height := int64(10)
		// save the last abci response.
		err = stateStore.SaveABCIResponses(height, response1)
		require.NoError(t, err)
		// search for the last abciresponse and check if it has saved.
		lastResponse, err := stateStore.LoadLastABCIResponse(height)
		require.NoError(t, err)
		// check to see if the saved response height is the same as the loaded height.
		assert.Equal(t, lastResponse, response1)
		// use an incorret height to make sure the state store errors.
		_, err = stateStore.LoadLastABCIResponse(height + 1)
		assert.Error(t, err)
		// check if the abci response didnt save in the abciresponses.
		responses, err = stateStore.LoadABCIResponses(height)
		require.NoError(t, err, responses)
		require.Equal(t, response1, responses)
	})

	t.Run("persisting responses", func(t *testing.T) {
		stateDB := dbm.NewMemDB()
		height := int64(10)
		// stub the second abciresponse.
		response2 := &tmstate.ABCIResponses{
			BeginBlock: &abci.ResponseBeginBlock{},
			DeliverTxs: []*abci.ResponseDeliverTx{
				{Code: 44, Data: []byte("Hello again"), Log: "????"},
			},
			EndBlock: &abci.ResponseEndBlock{},
		}
		// create a new statestore with the responses on.
		stateStore := sm.NewStore(stateDB, sm.StoreOptions{
			DiscardABCIResponses: true,
		})
		// save an additional response.
		err := stateStore.SaveABCIResponses(height+1, response2)
		require.NoError(t, err)
		// check to see if the response saved by calling the last response.
		lastResponse2, err := stateStore.LoadLastABCIResponse(height + 1)
		require.NoError(t, err)
		// check to see if the saved response height is the same as the loaded height.
		assert.Equal(t, response2, lastResponse2)
		// should error as we are no longer saving the response.
		_, err = stateStore.LoadABCIResponses(height + 1)
		assert.Equal(t, sm.ErrABCIResponsesNotPersisted, err)
	})

}
