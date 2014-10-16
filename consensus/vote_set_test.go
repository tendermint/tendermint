package consensus

import (
	. "github.com/tendermint/tendermint/blocks"
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

func makeVoteSet(numValidators int, votingPower uint64) (*VoteSet, *ValidatorSet, []*PrivAccount) {
	vals := make([]*Validator, numValidators)
	privAccounts := make([]*PrivAccount, numValidators)
	for i := 0; i < numValidators; i++ {
		val, privAccount := makeValidator(uint64(i), votingPower)
		vals[i] = val
		privAccounts[i] = privAccount
	}
	valSet := NewValidatorSet(vals)
	return NewVoteSet(0, 0, VoteTypeBare, valSet), valSet, privAccounts
}

func TestAddVote(t *testing.T) {
	voteSet, valSet, privAccounts := makeVoteSet(10, 1)
	vote := &Vote{Height: 0, Round: 0, Type: VoteTypeBare, BlockHash: nil}

	t.Logf(">> %v", voteSet)
	t.Logf(">> %v", valSet)
	t.Logf(">> %v", privAccounts)

	privAccounts[0].Sign(vote)
	voteSet.Add(vote)

	t.Logf(">> %v", voteSet)
	t.Logf(">> %v", valSet)
	t.Logf(">> %v", privAccounts)

}

func Test2_3Majority(t *testing.T) {
	// XXX
}
