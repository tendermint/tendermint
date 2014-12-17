package consensus

import (
	"sort"

	. "github.com/tendermint/tendermint/block"
	db_ "github.com/tendermint/tendermint/db"
	mempool_ "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/state"
)

// Common test methods

func makeValidator(votingPower uint64) (*state.Validator, *state.PrivValidator) {
	privValidator := state.GenPrivValidator()
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

func makeVoteSet(height uint, round uint, type_ byte, numValidators int, votingPower uint64) (*VoteSet, *state.ValidatorSet, []*state.PrivValidator) {
	vals := make([]*state.Validator, numValidators)
	privValidators := make([]*state.PrivValidator, numValidators)
	for i := 0; i < numValidators; i++ {
		val, privValidator := makeValidator(votingPower)
		vals[i] = val
		privValidators[i] = privValidator
	}
	valSet := state.NewValidatorSet(vals)
	sort.Sort(state.PrivValidatorsByAddress(privValidators))
	return NewVoteSet(height, round, type_, valSet), valSet, privValidators
}

func makeConsensusState() (*ConsensusState, []*state.PrivValidator) {
	state, _, privValidators := state.RandGenesisState(20, false, 1000, 10, false, 1000)
	blockStore := NewBlockStore(db_.NewMemDB())
	mempool := mempool_.NewMempool(state)
	mempoolReactor := mempool_.NewMempoolReactor(mempool)
	cs := NewConsensusState(state, blockStore, mempoolReactor)
	return cs, privValidators
}
