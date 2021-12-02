package state_test

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/crypto/tmhash"
	tmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/state/mocks"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

func TestRollback(t *testing.T) {
	stateStore := state.NewStore(dbm.NewMemDB())
	blockStore := &mocks.BlockStore{}
	var (
		height     int64 = 100
		nextHeight int64 = 101
	)

	valSet, _ := types.RandValidatorSet(5, 10)

	params := types.DefaultConsensusParams()
	params.Version.AppVersion = appVersion
	newParams := types.DefaultConsensusParams()
	newParams.Block.MaxBytes = 10000

	initialState := state.State{
		Version: tmstate.Version{
			Consensus: tmversion.Consensus{
				Block: version.BlockProtocol,
				App:   10,
			},
			Software: version.TMCoreSemVer,
		},
		ChainID:                          "test-chain",
		InitialHeight:                    10,
		LastBlockID:                      makeBlockIDRandom(),
		AppHash:                          tmhash.Sum([]byte("app_hash")),
		LastResultsHash:                  tmhash.Sum([]byte("last_results_hash")),
		LastBlockHeight:                  height,
		LastValidators:                   valSet,
		Validators:                       valSet.CopyIncrementProposerPriority(1),
		NextValidators:                   valSet.CopyIncrementProposerPriority(2),
		LastHeightValidatorsChanged:      height + 1,
		ConsensusParams:                  *params,
		LastHeightConsensusParamsChanged: height + 1,
	}
	require.NoError(t, stateStore.Bootstrap(initialState))

<<<<<<< HEAD:state/rollback_test.go
	height++
	block := &types.BlockMeta{
		Header: types.Header{
			Height:          height,
			AppHash:         initialState.AppHash,
			LastBlockID:     initialState.LastBlockID,
			LastResultsHash: initialState.LastResultsHash,
		},
	}
	blockStore.On("LoadBlockMeta", height).Return(block)

	appVersion++
	newParams.Version.AppVersion = appVersion
	nextState := initialState.Copy()
	nextState.LastBlockHeight = height
	nextState.Version.Consensus.App = appVersion
	nextState.LastBlockID = makeBlockIDRandom()
	nextState.AppHash = tmhash.Sum([]byte("next_app_hash"))
=======
	// perform the rollback over a version bump
	newParams := types.DefaultConsensusParams()
	newParams.Version.AppVersion = 11
	newParams.Block.MaxBytes = 1000
	nextState := initialState.Copy()
	nextState.LastBlockHeight = nextHeight
	nextState.Version.Consensus.App = 11
	nextState.LastBlockID = factory.MakeBlockID()
	nextState.AppHash = factory.RandomHash()
>>>>>>> bca2080c0 (cmd: add integration test and fix bug in rollback command (#7315)):internal/state/rollback_test.go
	nextState.LastValidators = initialState.Validators
	nextState.Validators = initialState.NextValidators
	nextState.NextValidators = initialState.NextValidators.CopyIncrementProposerPriority(1)
	nextState.ConsensusParams = *newParams
	nextState.LastHeightConsensusParamsChanged = nextHeight + 1
	nextState.LastHeightValidatorsChanged = nextHeight + 1

	// update the state
	require.NoError(t, stateStore.Save(nextState))

	block := &types.BlockMeta{
		BlockID: initialState.LastBlockID,
		Header: types.Header{
			Height:          initialState.LastBlockHeight,
			AppHash:         initialState.AppHash,
			LastBlockID:     factory.MakeBlockID(),
			LastResultsHash: initialState.LastResultsHash,
		},
	}
	blockStore.On("LoadBlockMeta", initialState.LastBlockHeight).Return(block)
	blockStore.On("Height").Return(nextHeight)

	// rollback the state
	rollbackHeight, rollbackHash, err := state.Rollback(blockStore, stateStore)
	require.NoError(t, err)
	require.EqualValues(t, height, rollbackHeight)
	require.EqualValues(t, initialState.AppHash, rollbackHash)
	blockStore.AssertExpectations(t)

	// assert that we've recovered the prior state
	loadedState, err := stateStore.Load()
	require.NoError(t, err)
	require.EqualValues(t, initialState, loadedState)
}

func TestRollbackNoState(t *testing.T) {
	stateStore := state.NewStore(dbm.NewMemDB())
	blockStore := &mocks.BlockStore{}

	_, _, err := state.Rollback(blockStore, stateStore)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no state found")
}

func TestRollbackNoBlocks(t *testing.T) {
<<<<<<< HEAD:state/rollback_test.go
=======
	const height = int64(100)
	stateStore := setupStateStore(t, height)
	blockStore := &mocks.BlockStore{}
	blockStore.On("Height").Return(height)
	blockStore.On("LoadBlockMeta", height-1).Return(nil)

	_, _, err := state.Rollback(blockStore, stateStore)
	require.Error(t, err)
	require.Contains(t, err.Error(), "block at height 99 not found")
}

func TestRollbackDifferentStateHeight(t *testing.T) {
	const height = int64(100)
	stateStore := setupStateStore(t, height)
	blockStore := &mocks.BlockStore{}
	blockStore.On("Height").Return(height + 2)

	_, _, err := state.Rollback(blockStore, stateStore)
	require.Error(t, err)
	require.Equal(t, err.Error(), "statestore height (100) is not one below or equal to blockstore height (102)")
}

func setupStateStore(t *testing.T, height int64) state.Store {
>>>>>>> bca2080c0 (cmd: add integration test and fix bug in rollback command (#7315)):internal/state/rollback_test.go
	stateStore := state.NewStore(dbm.NewMemDB())
	blockStore := &mocks.BlockStore{}
	var (
		height     int64  = 100
		appVersion uint64 = 10
	)

	valSet, _ := types.RandValidatorSet(5, 10)

	params := types.DefaultConsensusParams()
	params.Version.AppVersion = appVersion
	newParams := types.DefaultConsensusParams()
	newParams.Block.MaxBytes = 10000

	initialState := state.State{
		Version: tmstate.Version{
			Consensus: tmversion.Consensus{
				Block: version.BlockProtocol,
				App:   10,
			},
			Software: version.TMCoreSemVer,
		},
		ChainID:                          "test-chain",
		InitialHeight:                    10,
		LastBlockID:                      makeBlockIDRandom(),
		AppHash:                          tmhash.Sum([]byte("app_hash")),
		LastResultsHash:                  tmhash.Sum([]byte("last_results_hash")),
		LastBlockHeight:                  height,
		LastValidators:                   valSet,
		Validators:                       valSet.CopyIncrementProposerPriority(1),
		NextValidators:                   valSet.CopyIncrementProposerPriority(2),
		LastHeightValidatorsChanged:      height + 1,
		ConsensusParams:                  *params,
		LastHeightConsensusParamsChanged: height + 1,
	}
	require.NoError(t, stateStore.Save(initialState))
	blockStore.On("LoadBlockMeta", height).Return(nil)

	_, _, err := state.Rollback(blockStore, stateStore)
	require.Error(t, err)
	require.Contains(t, err.Error(), "block at height 100 not found")
}

func makeBlockIDRandom() types.BlockID {
	var (
		blockHash   = make([]byte, tmhash.Size)
		partSetHash = make([]byte, tmhash.Size)
	)
	rand.Read(blockHash)   //nolint: errcheck // ignore errcheck for read
	rand.Read(partSetHash) //nolint: errcheck // ignore errcheck for read
	return types.BlockID{
		Hash: blockHash,
		PartSetHeader: types.PartSetHeader{
			Total: 123,
			Hash:  partSetHash,
		},
	}
}
