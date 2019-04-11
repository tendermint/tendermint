package state

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	cfg "github.com/tendermint/tendermint/config"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/types"
)

func TestSaveValidatorsInfo(t *testing.T) {
	// test we persist validators every 100000 blocks
	stateDB := dbm.NewMemDB()
	val, _ := types.RandValidator(true, 10)
	vals := types.NewValidatorSet([]*types.Validator{val})

	// TODO(melekes): remove in 0.33 release
	// https://github.com/tendermint/tendermint/issues/3543
	assert.NotPanics(t, func() {
		LoadValidators(stateDB, 100000)
	})

	saveValidatorsInfo(stateDB, 100000, 1, vals)

	loadedVals, err := LoadValidators(stateDB, 100000)
	assert.NoError(t, err)
	assert.NotZero(t, loadedVals.Size())
}

func BenchmarkLoadValidators(b *testing.B) {
	const valSetSize = 100

	config := cfg.ResetTestRoot("state_")
	defer os.RemoveAll(config.RootDir)
	dbType := dbm.DBBackendType(config.DBBackend)
	stateDB := dbm.NewDB("state", dbType, config.DBDir())
	state, err := LoadStateFromDBOrGenesisFile(stateDB, config.GenesisFile())
	if err != nil {
		b.Fatal(err)
	}
	state.Validators = genValSet(valSetSize)
	state.NextValidators = state.Validators.CopyIncrementProposerPriority(1)
	SaveState(stateDB, state)

	for i := 10; i < 10000000000; i *= 10 { // 10, 100, 1000, ...
		saveValidatorsInfo(stateDB, int64(i), state.LastHeightValidatorsChanged, state.NextValidators)

		b.Run(fmt.Sprintf("height=%d", i), func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				_, err := LoadValidators(stateDB, int64(i))
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
