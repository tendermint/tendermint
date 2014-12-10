package consensus

import (
	"github.com/tendermint/tendermint/state"
)

// Common test methods

func makeValidator(votingPower uint64) (*state.Validator, *PrivValidator) {
	privValidator := GenPrivValidator()
	return &state.Validator{
		Address:          privValidator.Address,
		PubKey:           privValidator.PubKey,
		BondHeight:       0,
		UnbondHeight:     0,
		LastCommitHeight: 0,
		VotingPower:      votingPower,
		Accum:            0,
	}, privValidator
}

func makeVoteSet(height uint, round uint, type_ byte, numValidators int, votingPower uint64) (*VoteSet, *state.ValidatorSet, []*PrivValidator) {
	vals := make([]*state.Validator, numValidators)
	privValidators := make([]*PrivValidator, numValidators)
	for i := 0; i < numValidators; i++ {
		val, privValidator := makeValidator(votingPower)
		vals[i] = val
		privValidators[i] = privValidator
	}
	valSet := state.NewValidatorSet(vals)
	return NewVoteSet(height, round, type_, valSet), valSet, privValidators
}
