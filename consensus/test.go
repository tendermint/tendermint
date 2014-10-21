package consensus

import (
	. "github.com/tendermint/tendermint/blocks"
	"github.com/tendermint/tendermint/state"
)

// Common test methods

func makeValidator(id uint64, votingPower uint64) (*state.Validator, *state.PrivAccount) {
	privAccount := state.GenPrivAccount()
	privAccount.Id = id
	return &state.Validator{
		Account:     privAccount.Account,
		VotingPower: votingPower,
	}, privAccount
}

func makeVoteSet(height uint32, round uint16, numValidators int, votingPower uint64) (*VoteSet, *state.ValidatorSet, []*state.PrivAccount) {
	vals := make([]*state.Validator, numValidators)
	privAccounts := make([]*state.PrivAccount, numValidators)
	for i := 0; i < numValidators; i++ {
		val, privAccount := makeValidator(uint64(i), votingPower)
		vals[i] = val
		privAccounts[i] = privAccount
	}
	valSet := state.NewValidatorSet(vals)
	return NewVoteSet(height, round, VoteTypePrevote, valSet), valSet, privAccounts
}
