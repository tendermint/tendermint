package factory

import (
	"context"
	"fmt"
	"math/rand"
	"sort"

	"github.com/tendermint/tendermint/pkg/consensus"
)

func RandValidator(randPower bool, minPower int64) (*consensus.Validator, consensus.PrivValidator) {
	privVal := consensus.NewMockPV()
	votePower := minPower
	if randPower {
		// nolint:gosec // G404: Use of weak random number generator
		votePower += int64(rand.Uint32())
	}
	pubKey, err := privVal.GetPubKey(context.Background())
	if err != nil {
		panic(fmt.Errorf("could not retrieve pubkey %w", err))
	}
	val := consensus.NewValidator(pubKey, votePower)
	return val, privVal
}

func RandValidatorSet(numValidators int, votingPower int64) (*consensus.ValidatorSet, []consensus.PrivValidator) {
	var (
		valz           = make([]*consensus.Validator, numValidators)
		privValidators = make([]consensus.PrivValidator, numValidators)
	)

	for i := 0; i < numValidators; i++ {
		val, privValidator := RandValidator(false, votingPower)
		valz[i] = val
		privValidators[i] = privValidator
	}

	sort.Sort(consensus.PrivValidatorsByAddress(privValidators))

	return consensus.NewValidatorSet(valz), privValidators
}
