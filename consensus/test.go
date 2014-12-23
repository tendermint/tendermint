package consensus

import (
	"sort"

	. "github.com/tendermint/tendermint/block"
	db_ "github.com/tendermint/tendermint/db"
	mempool_ "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/state"
)

// Common test methods

func makeValidator(valInfo *state.ValidatorInfo) *state.Validator {
	return &state.Validator{
		Address:          valInfo.Address,
		PubKey:           valInfo.PubKey,
		BondHeight:       0,
		UnbondHeight:     0,
		LastCommitHeight: 0,
		VotingPower:      valInfo.FirstBondAmount,
		Accum:            0,
	}
}

func randVoteSet(height uint, round uint, type_ byte, numValidators int, votingPower uint64) (*VoteSet, *state.ValidatorSet, []*state.PrivValidator) {
	vals := make([]*state.Validator, numValidators)
	privValidators := make([]*state.PrivValidator, numValidators)
	for i := 0; i < numValidators; i++ {
		valInfo, privValidator := state.RandValidator(false, votingPower)
		val := makeValidator(valInfo)
		vals[i] = val
		privValidators[i] = privValidator
	}
	valSet := state.NewValidatorSet(vals)
	sort.Sort(state.PrivValidatorsByAddress(privValidators))
	return NewVoteSet(height, round, type_, valSet), valSet, privValidators
}

func randConsensusState() (*ConsensusState, []*state.PrivValidator) {
	state, _, privValidators := state.RandGenesisState(20, false, 1000, 10, false, 1000)
	blockStore := NewBlockStore(db_.NewMemDB())
	mempool := mempool_.NewMempool(state)
	mempoolReactor := mempool_.NewMempoolReactor(mempool)
	cs := NewConsensusState(state, blockStore, mempoolReactor)
	return cs, privValidators
}
