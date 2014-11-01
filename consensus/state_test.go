package consensus

import (
	"testing"
	"time"

	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
	. "github.com/tendermint/tendermint/common/test"
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
	if vote := rs0.Prevotes.GetById(0); vote == nil || vote.Type != VoteTypePrevote {
		t.Errorf("Expected to find prevote but got %v", vote)
	}
	if vote := rs0.Precommits.GetById(0); vote == nil || vote.Type != VoteTypePrecommit {
		t.Errorf("Expected to find precommit but got %v", vote)
	}
	if vote := rs0.Commits.GetById(0); vote == nil || vote.Type != VoteTypeCommit {
		t.Errorf("Expected to find commit but got %v", vote)
	}

	// Setup round 1 (next round)
	cs.SetupNewRound(1, 1)
	<-cs.NewStepCh() // TODO: test this value too.

	// Now the commit should be copied over to prevotes and precommits.
	rs1 := cs.GetRoundState()
	if vote := rs1.Prevotes.GetById(0); vote == nil || vote.Type != VoteTypeCommit {
		t.Errorf("Expected to find commit but got %v", vote)
	}
	if vote := rs1.Precommits.GetById(0); vote == nil || vote.Type != VoteTypeCommit {
		t.Errorf("Expected to find commit but got %v", vote)
	}
	if vote := rs1.Commits.GetById(0); vote == nil || vote.Type != VoteTypeCommit {
		t.Errorf("Expected to find commit but got %v", vote)
	}

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

func checkRoundState(t *testing.T, rs *RoundState,
	height uint32, round uint16, step RoundStep) {
	if rs.Height != height {
		t.Errorf("rs.Height should be %v, got %v", height, rs.Height)
	}
	if rs.Round != round {
		t.Errorf("rs.Round should be %v, got %v", round, rs.Round)
	}
	if rs.Step != step {
		t.Errorf("rs.Step should be %v, got %v", step, rs.Step)
	}
}

func TestRunActionPrecommitCommitFinalize(t *testing.T) {
	cs, privAccounts := makeConsensusState()
	priv := NewPrivValidator(db_.NewMemDB(), privAccounts[0])
	cs.SetPrivValidator(priv)

	cs.RunActionPrecommit(1, 0)
	<-cs.NewStepCh() // TODO: test this value too.
	if cs.Precommits.GetById(0) != nil {
		t.Errorf("RunActionPrecommit should return nil without a proposal")
	}

	cs.RunActionPropose(1, 0)
	<-cs.NewStepCh() // TODO: test this value too.

	// Test RunActionPrecommit failures:
	AssertPanics(t, "Wrong height ", func() { cs.RunActionPrecommit(2, 0) })
	AssertPanics(t, "Wrong round", func() { cs.RunActionPrecommit(1, 1) })
	cs.RunActionPrecommit(1, 0)
	<-cs.NewStepCh() // TODO: test this value too.
	if cs.Precommits.GetById(0) != nil {
		t.Errorf("RunActionPrecommit should return nil, not enough prevotes")
	}

	// Add at least +2/3 prevotes.
	for i := 0; i < 7; i++ {
		vote := &Vote{
			Height:     1,
			Round:      0,
			Type:       VoteTypePrevote,
			BlockHash:  cs.ProposalBlock.Hash(),
			BlockParts: cs.ProposalBlockParts.Header(),
		}
		privAccounts[i].Sign(vote)
		cs.AddVote(vote)
	}

	// Test RunActionPrecommit success:
	cs.RunActionPrecommit(1, 0)
	<-cs.NewStepCh() // TODO: test this value too.
	if cs.Precommits.GetById(0) == nil {
		t.Errorf("RunActionPrecommit should have succeeded")
	}
	checkRoundState(t, cs.GetRoundState(), 1, 0, RoundStepPrecommit)

	// Test RunActionCommit failures:
	AssertPanics(t, "Wrong height ", func() { cs.RunActionCommit(2) })
	AssertPanics(t, "Wrong round", func() { cs.RunActionCommit(1) })

	// Add at least +2/3 precommits.
	for i := 0; i < 7; i++ {
		vote := &Vote{
			Height:     1,
			Round:      0,
			Type:       VoteTypePrecommit,
			BlockHash:  cs.ProposalBlock.Hash(),
			BlockParts: cs.ProposalBlockParts.Header(),
		}
		privAccounts[i].Sign(vote)
		cs.AddVote(vote)
	}

	// Test RunActionCommit success:
	cs.RunActionCommit(1)
	<-cs.NewStepCh() // TODO: test this value too.
	if cs.Commits.GetById(0) == nil {
		t.Errorf("RunActionCommit should have succeeded")
	}
	checkRoundState(t, cs.GetRoundState(), 1, 0, RoundStepCommit)

	// cs.CommitTime should still be zero
	if !cs.CommitTime.IsZero() {
		t.Errorf("Expected CommitTime to yet be zero")
	}

	// Add at least +2/3 commits.
	for i := 0; i < 7; i++ {
		vote := &Vote{
			Height:     1,
			Round:      uint16(i), // Doesn't matter what round
			Type:       VoteTypeCommit,
			BlockHash:  cs.ProposalBlock.Hash(),
			BlockParts: cs.ProposalBlockParts.Header(),
		}
		privAccounts[i].Sign(vote)
		cs.AddVote(vote)
	}

	// Test TryFinalizeCommit:
	cs.TryFinalizeCommit(1)
	<-cs.NewStepCh() // TODO: test this value too.
	checkRoundState(t, cs.GetRoundState(), 2, 0, RoundStepNewHeight)
}
