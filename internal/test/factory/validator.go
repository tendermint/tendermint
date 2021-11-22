package factory

import (
	"context"
	"fmt"
	"sort"

	"github.com/tendermint/tendermint/types"
)

func Validator(votingPower int64) (*types.Validator, types.PrivValidator) {
	privVal := types.NewMockPV()
	pubKey, err := privVal.GetPubKey(context.Background())
	if err != nil {
		panic(fmt.Errorf("could not retrieve pubkey %w", err))
	}
	val := types.NewValidator(pubKey, votingPower)
	return val, privVal
}

func ValidatorSet(numValidators int, votingPower int64) (*types.ValidatorSet, []types.PrivValidator) {
	var (
		valz           = make([]*types.Validator, numValidators)
		privValidators = make([]types.PrivValidator, numValidators)
	)

	for i := 0; i < numValidators; i++ {
		val, privValidator := Validator(votingPower)
		valz[i] = val
		privValidators[i] = privValidator
	}

	sort.Sort(types.PrivValidatorsByAddress(privValidators))

	return types.NewValidatorSet(valz), privValidators
}
