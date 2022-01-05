package factory

import (
	"context"
	"sort"
	"testing"

	"github.com/tendermint/tendermint/config"
	tmtime "github.com/tendermint/tendermint/libs/time"
	"github.com/tendermint/tendermint/types"
)

func RandGenesisDoc(ctx context.Context, t *testing.T, cfg *config.Config, numValidators int, randPower bool, minPower int64) (*types.GenesisDoc, []types.PrivValidator) {
	t.Helper()

	validators := make([]types.GenesisValidator, numValidators)
	privValidators := make([]types.PrivValidator, numValidators)
	for i := 0; i < numValidators; i++ {
		val, privVal := RandValidator(ctx, Require(t), randPower, minPower)
		validators[i] = types.GenesisValidator{
			PubKey: val.PubKey,
			Power:  val.VotingPower,
		}
		privValidators[i] = privVal
	}
	sort.Sort(types.PrivValidatorsByAddress(privValidators))

	return &types.GenesisDoc{
		GenesisTime:   tmtime.Now(),
		InitialHeight: 1,
		ChainID:       cfg.ChainID(),
		Validators:    validators,
	}, privValidators
}
