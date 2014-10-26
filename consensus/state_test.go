package consensus

import (
	"testing"
	"time"

	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
	db_ "github.com/tendermint/tendermint/db"
	"github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/state"
)

func randAccountDetail(id uint64, status byte) (*state.AccountDetail, *state.PrivAccount) {
	privAccount := state.GenPrivAccount()
	privAccount.Id = id
	account := privAccount.Account
	return &state.AccountDetail{
		Account:  account,
		Sequence: RandUInt(),
		Balance:  1000,
		Status:   status,
	}, privAccount
}

// The first numValidators accounts are validators.
func randGenesisState(numAccounts int, numValidators int) (*state.State, []*state.PrivAccount) {
	db := db_.NewMemDB()
	accountDetails := make([]*state.AccountDetail, numAccounts)
	privAccounts := make([]*state.PrivAccount, numAccounts)
	for i := 0; i < numAccounts; i++ {
		if i < numValidators {
			accountDetails[i], privAccounts[i] =
				randAccountDetail(uint64(i), state.AccountStatusBonded)
		} else {
			accountDetails[i], privAccounts[i] =
				randAccountDetail(uint64(i), state.AccountStatusNominal)
		}
	}
	s0 := state.GenesisState(db, time.Now(), accountDetails)
	s0.Save()
	return s0, privAccounts
}

func makeConsensusState() (*ConsensusState, []*state.PrivAccount) {
	state, privAccounts := randGenesisState(20, 10)
	blockStore := NewBlockStore(db_.NewMemDB())
	mempool := mempool.NewMempool(state)
	cs := NewConsensusState(state, blockStore, mempool)
	return cs, privAccounts
}

func assertPanics(t *testing.T, msg string, f func()) {
	defer func() {
		if err := recover(); err == nil {
			t.Error("Should have panic'd, but didn't. %v", msg)
		}
	}()
	f()
}

//-----------------------------------------------------------------------------

func TestSetupRound(t *testing.T) {
	cs, privAccounts := makeConsensusState()

	// Add a vote, precommit, and commit by val0.
	voteTypes := []byte{VoteTypePrevote, VoteTypePrecommit, VoteTypeCommit}
	for _, voteType := range voteTypes {
		vote := &Vote{Height: 1, Round: 0, Type: voteType} // nil vote
		privAccounts[0].Sign(vote)
		cs.AddVote(vote)
	}

	// Ensure that vote appears in RoundState.
	rs0 := cs.GetRoundState()
	if vote := rs0.Prevotes.Get(0); vote == nil || vote.Type != VoteTypePrevote {
		t.Errorf("Expected to find prevote but got %v", vote)
	}
	if vote := rs0.Precommits.Get(0); vote == nil || vote.Type != VoteTypePrecommit {
		t.Errorf("Expected to find precommit but got %v", vote)
	}
	if vote := rs0.Commits.Get(0); vote == nil || vote.Type != VoteTypeCommit {
		t.Errorf("Expected to find commit but got %v", vote)
	}

	// Setup round 1 (next round)
	cs.SetupRound(1)

	// Now the commit should be copied over to prevotes and precommits.
	rs1 := cs.GetRoundState()
	if vote := rs1.Prevotes.Get(0); vote == nil || vote.Type != VoteTypeCommit {
		t.Errorf("Expected to find commit but got %v", vote)
	}
	if vote := rs1.Precommits.Get(0); vote == nil || vote.Type != VoteTypeCommit {
		t.Errorf("Expected to find commit but got %v", vote)
	}
	if vote := rs1.Commits.Get(0); vote == nil || vote.Type != VoteTypeCommit {
		t.Errorf("Expected to find commit but got %v", vote)
	}

	// Setup round 1 (should fail)
	assertPanics(t, "Round did not increment", func() {
		cs.SetupRound(1)
	})

}

func TestRunActionProposeNoPrivValidator(t *testing.T) {
	cs, _ := makeConsensusState()
	cs.RunActionPropose(1, 0)
	rs := cs.GetRoundState()
	if rs.Proposal != nil {
		t.Error("Expected to make no proposal, since no privValidator")
	}
}

func TestRunActionPropose(t *testing.T) {
	cs, privAccounts := makeConsensusState()
	priv := NewPrivValidator(db_.NewMemDB(), privAccounts[0])
	cs.SetPrivValidator(priv)

	cs.RunActionPropose(1, 0)
	rs := cs.GetRoundState()

	// Check that Proposal, ProposalBlock, ProposalBlockParts are set.
	if rs.Proposal == nil {
		t.Error("rs.Proposal should be set")
	}
	if rs.ProposalBlock == nil {
		t.Error("rs.ProposalBlock should be set")
	}
	if rs.ProposalBlockParts.Total() == 0 {
		t.Error("rs.ProposalBlockParts should be set")
	}
}

func checkRoundState(t *testing.T, cs *ConsensusState,
	height uint32, round uint16, step RoundStep) {
	rs := cs.GetRoundState()
	if rs.Height != height {
		t.Errorf("cs.RoundState.Height should be %v, got %v", height, rs.Height)
	}
	if rs.Round != round {
		t.Errorf("cs.RoundState.Round should be %v, got %v", round, rs.Round)
	}
	if rs.Step != step {
		t.Errorf("cs.RoundState.Step should be %v, got %v", step, rs.Step)
	}
}

func TestRunActionPrecommitCommitFinalize(t *testing.T) {
	cs, privAccounts := makeConsensusState()
	priv := NewPrivValidator(db_.NewMemDB(), privAccounts[0])
	cs.SetPrivValidator(priv)

	vote := cs.RunActionPrecommit(1, 0)
	if vote != nil {
		t.Errorf("RunActionPrecommit should return nil without a proposal")
	}

	cs.RunActionPropose(1, 0)

	// Test RunActionPrecommit failures:
	assertPanics(t, "Wrong height ", func() { cs.RunActionPrecommit(2, 0) })
	assertPanics(t, "Wrong round", func() { cs.RunActionPrecommit(1, 1) })
	vote = cs.RunActionPrecommit(1, 0)
	if vote != nil {
		t.Errorf("RunActionPrecommit should return nil, not enough prevotes")
	}

	// Add at least +2/3 prevotes.
	for i := 0; i < 7; i++ {
		vote := &Vote{
			Height:    1,
			Round:     0,
			Type:      VoteTypePrevote,
			BlockHash: cs.ProposalBlock.Hash(),
		}
		privAccounts[i].Sign(vote)
		cs.AddVote(vote)
	}

	// Test RunActionPrecommit success:
	vote = cs.RunActionPrecommit(1, 0)
	if vote == nil {
		t.Errorf("RunActionPrecommit should have succeeded")
	}
	checkRoundState(t, cs, 1, 0, RoundStepPrecommit)

	// Test RunActionCommit failures:
	assertPanics(t, "Wrong height ", func() { cs.RunActionCommit(2, 0) })
	assertPanics(t, "Wrong round", func() { cs.RunActionCommit(1, 1) })

	// Add at least +2/3 precommits.
	for i := 0; i < 7; i++ {
		vote := &Vote{
			Height:    1,
			Round:     0,
			Type:      VoteTypePrecommit,
			BlockHash: cs.ProposalBlock.Hash(),
		}
		privAccounts[i].Sign(vote)
		cs.AddVote(vote)
	}

	// Test RunActionCommit success:
	vote = cs.RunActionCommit(1, 0)
	if vote == nil {
		t.Errorf("RunActionCommit should have succeeded")
	}
	checkRoundState(t, cs, 1, 0, RoundStepCommit)

	// cs.CommitTime should still be zero
	if !cs.CommitTime.IsZero() {
		t.Errorf("Expected CommitTime to yet be zero")
	}

	// Add at least +2/3 commits.
	for i := 0; i < 7; i++ {
		vote := &Vote{
			Height:    1,
			Round:     uint16(i), // Doesn't matter what round
			Type:      VoteTypeCommit,
			BlockHash: cs.ProposalBlock.Hash(),
		}
		privAccounts[i].Sign(vote)
		cs.AddVote(vote)
	}

	// Test RunActionCommitWait:
	cs.RunActionCommitWait(1, 0)
	if cs.CommitTime.IsZero() {
		t.Errorf("Expected CommitTime to have been set")
	}
	checkRoundState(t, cs, 1, 0, RoundStepCommitWait)

	// Test RunActionFinalize:
	cs.RunActionFinalize(1, 0)
	checkRoundState(t, cs, 2, 0, RoundStepStart)
}
