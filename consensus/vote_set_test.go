package consensus

import (
	. "github.com/tendermint/tendermint/state"

	"testing"
)

func makeValidator(id uint64, votingPower uint64) (*Validator, *PrivAccount) {
	privAccount := GenPrivAccount()
	privAccount.Id = id
	return &Validator{
		Account:     privAccount.Account,
		VotingPower: votingPower,
	}, privAccount
}

func TestAddVote(t *testing.T) {
	// XXX
}

func Test2_3Majority(t *testing.T) {
	// XXX
}
