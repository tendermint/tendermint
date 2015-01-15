package consensus

import (
	"sort"

	"github.com/tendermint/tendermint/block"
	dbm "github.com/tendermint/tendermint/db"
	mempl "github.com/tendermint/tendermint/mempool"
	sm "github.com/tendermint/tendermint/state"
)

// Common test methods

func makeValidator(valInfo *sm.ValidatorInfo) *sm.Validator {
	return &sm.Validator{
		Address:          valInfo.Address,
		PubKey:           valInfo.PubKey,
		BondHeight:       0,
		UnbondHeight:     0,
		LastCommitHeight: 0,
		VotingPower:      valInfo.FirstBondAmount,
		Accum:            0,
	}
}

func randVoteSet(height uint, round uint, type_ byte, numValidators int, votingPower uint64) (*VoteSet, *sm.ValidatorSet, []*sm.PrivValidator) {
	vals := make([]*sm.Validator, numValidators)
	privValidators := make([]*sm.PrivValidator, numValidators)
	for i := 0; i < numValidators; i++ {
		valInfo, privValidator := sm.RandValidator(false, votingPower)
		val := makeValidator(valInfo)
		vals[i] = val
		privValidators[i] = privValidator
	}
	valSet := sm.NewValidatorSet(vals)
	sort.Sort(sm.PrivValidatorsByAddress(privValidators))
	return NewVoteSet(height, round, type_, valSet), valSet, privValidators
}

func randConsensusState() (*ConsensusState, []*sm.PrivValidator) {
	state, _, privValidators := sm.RandGenesisState(20, false, 1000, 10, false, 1000)
	blockStore := block.NewBlockStore(dbm.NewMemDB())
	mempool := mempl.NewMempool(state)
	mempoolReactor := mempl.NewMempoolReactor(mempool)
	cs := NewConsensusState(state, blockStore, mempoolReactor)
	return cs, privValidators
}
