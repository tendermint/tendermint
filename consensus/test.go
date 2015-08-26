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

func changeProposer(t *testing.T, perspectiveOf, newProposer *ConsensusState) *types.Block {
	_, v1 := perspectiveOf.Validators.GetByAddress(perspectiveOf.privValidator.Address)
	v1.Accum, v1.VotingPower = 0, 0
	if updated := perspectiveOf.Validators.Update(v1); !updated {
		t.Fatal("failed to update validator")
	}
	_, v2 := perspectiveOf.Validators.GetByAddress(newProposer.privValidator.Address)
	v2.Accum, v2.VotingPower = 100, 100
	if updated := perspectiveOf.Validators.Update(v2); !updated {
		t.Fatal("failed to update validator")
	}

	// make the proposal
	propBlock, _ := newProposer.createProposalBlock()
	if propBlock == nil {
		t.Fatal("Failed to create proposal block with cs2")
	}
	return propBlock
}

func fixVotingPower(t *testing.T, cs1 *ConsensusState, addr2 []byte) {
	_, v1 := cs1.Validators.GetByAddress(cs1.privValidator.Address)
	_, v2 := cs1.Validators.GetByAddress(addr2)
	v1.Accum, v1.VotingPower = v2.Accum, v2.VotingPower
	if updated := cs1.Validators.Update(v1); !updated {
		t.Fatal("failed to update validator")
	}
}

func addVoteToFromMany(to *ConsensusState, votes []*types.Vote, froms ...*ConsensusState) {
	if len(votes) != len(froms) {
		panic("len(votes) and len(froms) must match")
	}
	for i, from := range froms {
		addVoteToFrom(to, from, votes[i])
	}
}

func addVoteToFrom(to, from *ConsensusState, vote *types.Vote) {
	valIndex, _ := to.Validators.GetByAddress(from.privValidator.Address)
	added, err := to.TryAddVote(valIndex, vote, "")
	if _, ok := err.(*types.ErrVoteConflictingSignature); ok {
		// let it fly
	} else if !added {
		panic("Failed to add vote")
	} else if err != nil {
		panic(fmt.Sprintln("Failed to add vote:", err))
	}
}

func signVote(from *ConsensusState, voteType byte, hash []byte, header types.PartSetHeader) *types.Vote {
	vote, err := from.signVote(voteType, hash, header)
	if err != nil {
		panic(fmt.Sprintln("Failed to sign vote", err))
	}
	return vote
}

func signVoteMany(voteType byte, hash []byte, header types.PartSetHeader, css ...*ConsensusState) []*types.Vote {
	votes := make([]*types.Vote, len(css))
	for i, cs := range css {
		votes[i] = signVote(cs, voteType, hash, header)
	}
	return votes
}

// add vote to one cs from another
func signAddVoteToFromMany(voteType byte, to *ConsensusState, hash []byte, header types.PartSetHeader, froms ...*ConsensusState) {
	for _, from := range froms {
		vote := signVote(from, voteType, hash, header)
		addVoteToFrom(to, from, vote)
	}
}

func signAddVoteToFrom(voteType byte, to, from *ConsensusState, hash []byte, header types.PartSetHeader) *types.Vote {
	vote := signVote(from, voteType, hash, header)
	addVoteToFrom(to, from, vote)
	return vote
}

func ensureNoNewStep(t *testing.T, cs *ConsensusState) {
	timeout := time.NewTicker(2 * time.Second)
	select {
	case <-timeout.C:
		break
	case <-cs.NewStepCh():
		panic("We should be stuck waiting for more votes, not moving to the next step")
	}
}

func ensureNewStep(t *testing.T, cs *ConsensusState) {
	timeout := time.NewTicker(2 * time.Second)
	select {
	case <-timeout.C:
		panic("We should have gone to the next step, not be stuck waiting")
	case <-cs.NewStepCh():
		break
	}
}

func validatePrevote(t *testing.T, cs *ConsensusState, round int, privVal *types.PrivValidator, blockHash []byte) {
	prevotes := cs.Votes.Prevotes(round)
	var vote *types.Vote
	if vote = prevotes.GetByAddress(privVal.Address); vote == nil {
		panic("Failed to find prevote from validator")
	}
	if blockHash == nil {
		if vote.BlockHash != nil {
			panic(fmt.Sprintf("Expected prevote to be for nil, got %X", vote.BlockHash))
		}
	} else {
		if !bytes.Equal(vote.BlockHash, blockHash) {
			panic(fmt.Sprintf("Expected prevote to be for %X, got %X", blockHash, vote.BlockHash))
		}
	}
}

func incrementRound(css ...*ConsensusState) {
	for _, cs := range css {
		cs.Round += 1
	}
}

func validatePrecommit(t *testing.T, cs *ConsensusState, thisRound, lockRound int, privVal *types.PrivValidator, votedBlockHash, lockedBlockHash []byte) {
	precommits := cs.Votes.Precommits(thisRound)
	var vote *types.Vote
	if vote = precommits.GetByAddress(privVal.Address); vote == nil {
		panic("Failed to find precommit from validator")
	}

	if votedBlockHash == nil {
		if vote.BlockHash != nil {
			panic("Expected precommit to be for nil")
		}
	} else {
		if !bytes.Equal(vote.BlockHash, votedBlockHash) {
			panic("Expected precommit to be for proposal block")
		}
	}

	if lockedBlockHash == nil {
		if cs.LockedRound != lockRound || cs.LockedBlock != nil {
			panic(fmt.Sprintf("Expected to be locked on nil at round %d. Got locked at round %d with block %v", lockRound, cs.LockedRound, cs.LockedBlock))
		}
	} else {
		if cs.LockedRound != lockRound || !bytes.Equal(cs.LockedBlock.Hash(), lockedBlockHash) {
			panic(fmt.Sprintf("Expected block to be locked on round %d, got %d. Got locked block %X, expected %X", lockRound, cs.LockedRound, cs.LockedBlock.Hash(), lockedBlockHash))
		}
	}

}

func validatePrevoteAndPrecommit(t *testing.T, cs *ConsensusState, thisRound, lockRound int, privVal *types.PrivValidator, votedBlockHash, lockedBlockHash []byte, f func()) {
	// verify the prevote
	validatePrevote(t, cs, thisRound, privVal, votedBlockHash)
	if f != nil {
		f()
	}
	// wait to finish precommit
	<-cs.NewStepCh()
	// verify precommit
	cs.mtx.Lock()
	validatePrecommit(t, cs, thisRound, lockRound, privVal, votedBlockHash, lockedBlockHash)
	cs.mtx.Unlock()
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
		cs.SetPrivValidator(privVals[i])

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
