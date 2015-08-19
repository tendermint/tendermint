package consensus

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	bc "github.com/tendermint/tendermint/blockchain"
	dbm "github.com/tendermint/tendermint/db"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

//-------------------------------------------------------------------------------
// utils

// add vote to one cs from another
func addVoteToFrom(t *testing.T, voteType byte, to, from *ConsensusState, hash []byte, header types.PartSetHeader) {
	vote, err := from.signVote(voteType, hash, header)
	if err != nil {
		panic(fmt.Sprintln("Failed to sign vote", err))
	}
	valIndex, _ := to.Validators.GetByAddress(from.privValidator.Address)
	added, err := to.TryAddVote(to.GetRoundState(), vote, valIndex, "")
	if _, ok := err.(*types.ErrVoteConflictingSignature); ok {
		// let it fly
	} else if !added {
		panic("Failed to add vote")
	} else if err != nil {
		panic(fmt.Sprintln("Failed to add vote:", err))
	}
}

func ensureNoNewStep(t *testing.T, cs *ConsensusState) {
	timeout := time.NewTicker(2 * time.Second)
	select {
	case <-timeout.C:
		break
	case <-cs.NewStepCh():
		t.Fatal("We should be stuck waiting for more prevotes, not moving to the next step")
	}
}

func validatePrevote(t *testing.T, cs *ConsensusState, round int, privVal *types.PrivValidator, blockHash []byte) {
	prevotes := cs.Votes.Prevotes(round)
	var vote *types.Vote
	if vote = prevotes.GetByAddress(privVal.Address); vote == nil {
		t.Fatal("Failed to find prevote from validator")
	}
	if blockHash == nil {
		if vote.BlockHash != nil {
			t.Fatal("Expected prevote to be for nil")
		}
	} else {
		if !bytes.Equal(vote.BlockHash, blockHash) {
			t.Fatal("Expected prevote to be for proposal block")
		}
	}
}

func validatePrecommit(t *testing.T, cs *ConsensusState, thisRound, lockRound int, privVal *types.PrivValidator, block *types.Block) {
	precommits := cs.Votes.Precommits(thisRound)
	var vote *types.Vote
	if vote = precommits.GetByAddress(privVal.Address); vote == nil {
		panic("Failed to find precommit from validator")
	}

	if block == nil {
		if vote.BlockHash != nil {
			panic("Expected precommit to be for nil")
		}
		if cs.LockedRound != lockRound || cs.LockedBlock != nil {
			panic("Expected to be locked on nil")
		}
	} else {
		if !bytes.Equal(vote.BlockHash, block.Hash()) {
			panic("Expected precommit to be for proposal block")
		}
		if cs.LockedRound != lockRound || cs.LockedBlock != block {
			panic(fmt.Sprintf("Expected block to be locked on round %d, got %d. Got locked block %v, expected %v", lockRound, cs.LockedRound, cs.LockedBlock, block))
		}
	}

}

func simpleConsensusState(nValidators int) ([]*ConsensusState, []*types.PrivValidator) {
	// Get State
	state, privAccs, privVals := sm.RandGenesisState(10, true, 1000, nValidators, false, 10)
	_, _ = privAccs, privVals

	fmt.Println(state.BondedValidators)

	css := make([]*ConsensusState, nValidators)
	for i := 0; i < nValidators; i++ {
		// Get BlockStore
		blockDB := dbm.NewMemDB()
		blockStore := bc.NewBlockStore(blockDB)

		// Make MempoolReactor
		mempool := mempl.NewMempool(state.Copy())
		mempoolReactor := mempl.NewMempoolReactor(mempool)

		mempoolReactor.SetSwitch(p2p.NewSwitch())

		// Make ConsensusReactor
		cs := NewConsensusState(state, blockStore, mempoolReactor)

		// read off the NewHeightStep
		<-cs.NewStepCh()

		css[i] = cs
	}

	return css, privVals
}

func randConsensusState() (*ConsensusState, []*types.PrivValidator) {
	state, _, privValidators := sm.RandGenesisState(20, false, 1000, 10, false, 1000)
	blockStore := bc.NewBlockStore(dbm.NewMemDB())
	mempool := mempl.NewMempool(state)
	mempoolReactor := mempl.NewMempoolReactor(mempool)
	cs := NewConsensusState(state, blockStore, mempoolReactor)
	return cs, privValidators
}
