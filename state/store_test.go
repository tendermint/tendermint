package state

import (
	"fmt"
	"os"
	"testing"

	cfg "github.com/tendermint/tendermint/config"
	dbm "github.com/tendermint/tendermint/libs/db"
)

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

	for i := 10; i < 10000000000; i *= 10 {
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
