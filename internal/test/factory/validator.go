package factory

import (
	"context"
	"fmt"
	"math/rand"
	"sort"

	"github.com/tendermint/tendermint/types"
)

func RandValidator(ctx context.Context, randPower bool, minPower int64) (*types.Validator, types.PrivValidator) {
	privVal := types.NewMockPV()
	votePower := minPower
	if randPower {
		// nolint:gosec // G404: Use of weak random number generator
		votePower += int64(rand.Uint32())
	}
	pubKey, err := privVal.GetPubKey(ctx)
	if err != nil {
		panic(fmt.Errorf("could not retrieve pubkey %w", err))
	}
	val := types.NewValidator(pubKey, votePower)
	return val, privVal
}

func RandValidatorSet(ctx context.Context, numValidators int, votingPower int64) (*types.ValidatorSet, []types.PrivValidator) {
	var (
		valz           = make([]*types.Validator, numValidators)
		privValidators = make([]types.PrivValidator, numValidators)
	)

	for i := 0; i < numValidators; i++ {
		val, privValidator := RandValidator(ctx, false, votingPower)
		valz[i] = val
		privValidators[i] = privValidator
	}

	sort.Sort(types.PrivValidatorsByAddress(privValidators))

	return types.NewValidatorSet(valz), privValidators
}
