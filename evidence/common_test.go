// +build unit integration

package evidence

import (
	"os"
	"testing"
	"time"

	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
	dbm "github.com/tendermint/tm-db"
)

func TestMain(m *testing.M) {
	RegisterMockEvidences()

	code := m.Run()
	os.Exit(code)
}

// initializeBlockStore creates a block storage and populates it w/ a dummy
// block at +height+.
func initializeBlockStore(db dbm.DB, state sm.State, valAddr []byte) *store.BlockStore {
	blockStore := store.NewBlockStore(db)

	for i := int64(1); i <= state.LastBlockHeight; i++ {
		lastCommit := makeCommit(i-1, valAddr)
		block, _ := state.MakeBlock(i, []types.Tx{}, lastCommit, nil,
			state.Validators.GetProposer().Address)

		const parts = 1
		partSet := block.MakePartSet(parts)

		seenCommit := makeCommit(i, valAddr)
		blockStore.SaveBlock(block, partSet, seenCommit)
	}

	return blockStore
}

func initializeValidatorState(valAddr []byte, height int64) dbm.DB {
	stateDB := dbm.NewMemDB()

	// create validator set and state
	valSet := &types.ValidatorSet{
		Validators: []*types.Validator{
			{Address: valAddr, VotingPower: 0},
		},
	}
	state := sm.State{
		LastBlockHeight:             height,
		LastBlockTime:               tmtime.Now(),
		LastValidators:              valSet,
		Validators:                  valSet,
		NextValidators:              valSet.CopyIncrementProposerPriority(1),
		LastHeightValidatorsChanged: 1,
		ConsensusParams: types.ConsensusParams{
			Block: types.BlockParams{
				MaxBytes: 22020096,
				MaxGas:   -1,
			},
			Evidence: types.EvidenceParams{
				MaxAgeNumBlocks: 20,
				MaxAgeDuration:  48 * time.Hour,
			},
		},
	}

	// save all states up to height
	for i := int64(0); i <= height; i++ {
		state.LastBlockHeight = i
		sm.SaveState(stateDB, state)
	}

	return stateDB
}

func makeCommit(height int64, valAddr []byte) *types.Commit {
	commitSigs := []types.CommitSig{{
		BlockIDFlag:      types.BlockIDFlagCommit,
		ValidatorAddress: valAddr,
		Timestamp:        time.Now(),
		Signature:        []byte("Signature"),
	}}
	return types.NewCommit(height, 0, types.BlockID{}, commitSigs)
}
