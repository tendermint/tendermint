package consensus

import (
	"bytes"
	"testing"

	blk "github.com/tendermint/tendermint/block"
)

func TestSetupRound(t *testing.T) {
	cs, privValidators := randConsensusState()
	val0 := privValidators[0]

	// Add a vote, precommit, and commit by val0.
	voteTypes := []byte{blk.VoteTypePrevote, blk.VoteTypePrecommit, blk.VoteTypeCommit}
	for _, voteType := range voteTypes {
		vote := &blk.Vote{Height: 1, Round: 0, Type: voteType} // nil vote
		err := val0.SignVote(vote)
		if err != nil {
			t.Error("Error signing vote: %v", err)
		}
		cs.AddVote(val0.Address, vote)
	}

	// Ensure that vote appears in RoundState.
	rs0 := cs.GetRoundState()
	if vote := rs0.Prevotes.GetByAddress(val0.Address); vote == nil || vote.Type != blk.VoteTypePrevote {
		t.Errorf("Expected to find prevote but got %v", vote)
	}
	if vote := rs0.Precommits.GetByAddress(val0.Address); vote == nil || vote.Type != blk.VoteTypePrecommit {
		t.Errorf("Expected to find precommit but got %v", vote)
	}
	if vote := rs0.Commits.GetByAddress(val0.Address); vote == nil || vote.Type != blk.VoteTypeCommit {
		t.Errorf("Expected to find commit but got %v", vote)
	}

	// Setup round 1 (next round)
	cs.SetupNewRound(1, 1)
	<-cs.NewStepCh()

	// Now the commit should be copied over to prevotes and precommits.
	rs1 := cs.GetRoundState()
	if vote := rs1.Prevotes.GetByAddress(val0.Address); vote == nil || vote.Type != blk.VoteTypeCommit {
		t.Errorf("Expected to find commit but got %v", vote)
	}
	if vote := rs1.Precommits.GetByAddress(val0.Address); vote == nil || vote.Type != blk.VoteTypeCommit {
		t.Errorf("Expected to find commit but got %v", vote)
	}
	if vote := rs1.Commits.GetByAddress(val0.Address); vote == nil || vote.Type != blk.VoteTypeCommit {
		t.Errorf("Expected to find commit but got %v", vote)
	}

}

func TestRunActionProposeNoPrivValidator(t *testing.T) {
	cs, _ := randConsensusState()
	cs.RunActionPropose(1, 0)
	rs := cs.GetRoundState()
	if rs.Proposal != nil {
		t.Error("Expected to make no proposal, since no privValidator")
	}
}

func TestRunActionPropose(t *testing.T) {
	cs, privValidators := randConsensusState()
	val0 := privValidators[0]
	cs.SetPrivValidator(val0)

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
	height uint, round uint, step RoundStep) {
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
	cs, privValidators := randConsensusState()
	val0 := privValidators[0]
	cs.SetPrivValidator(val0)

	cs.RunActionPrecommit(1, 0)
	<-cs.NewStepCh() // TODO: test this value too.
	if cs.Precommits.GetByAddress(val0.Address) != nil {
		t.Errorf("RunActionPrecommit should return nil without a proposal")
	}

	cs.RunActionPropose(1, 0)
	<-cs.NewStepCh() // TODO: test this value too.

	cs.RunActionPrecommit(1, 0)
	<-cs.NewStepCh() // TODO: test this value too.
	if cs.Precommits.GetByAddress(val0.Address) != nil {
		t.Errorf("RunActionPrecommit should return nil, not enough prevotes")
	}

	// Add at least +2/3 prevotes.
	for i := 0; i < 7; i++ {
		vote := &blk.Vote{
			Height:     1,
			Round:      0,
			Type:       blk.VoteTypePrevote,
			BlockHash:  cs.ProposalBlock.Hash(),
			BlockParts: cs.ProposalBlockParts.Header(),
		}
		err := privValidators[i].SignVote(vote)
		if err != nil {
			t.Error("Error signing vote: %v", err)
		}
		cs.AddVote(privValidators[i].Address, vote)
	}

	// Test RunActionPrecommit success:
	cs.RunActionPrecommit(1, 0)
	<-cs.NewStepCh() // TODO: test this value too.
	if cs.Precommits.GetByAddress(val0.Address) == nil {
		t.Errorf("RunActionPrecommit should have succeeded")
	}
	checkRoundState(t, cs.GetRoundState(), 1, 0, RoundStepPrecommit)

	// Add at least +2/3 precommits.
	for i := 0; i < 7; i++ {
		if bytes.Equal(privValidators[i].Address, val0.Address) {
			if cs.Precommits.GetByAddress(val0.Address) == nil {
				t.Errorf("Proposer should already have signed a precommit vote")
			}
			continue
		}
		vote := &blk.Vote{
			Height:     1,
			Round:      0,
			Type:       blk.VoteTypePrecommit,
			BlockHash:  cs.ProposalBlock.Hash(),
			BlockParts: cs.ProposalBlockParts.Header(),
		}
		err := privValidators[i].SignVote(vote)
		if err != nil {
			t.Error("Error signing vote: %v", err)
		}
		added, _, err := cs.AddVote(privValidators[i].Address, vote)
		if !added || err != nil {
			t.Errorf("Error adding precommit: %v", err)
		}
	}

	// Test RunActionCommit success:
	cs.RunActionCommit(1)
	<-cs.NewStepCh() // TODO: test this value too.
	if cs.Commits.GetByAddress(val0.Address) == nil {
		t.Errorf("RunActionCommit should have succeeded")
	}
	checkRoundState(t, cs.GetRoundState(), 1, 0, RoundStepCommit)

	// cs.CommitTime should still be zero
	if !cs.CommitTime.IsZero() {
		t.Errorf("Expected CommitTime to yet be zero")
	}

	// Add at least +2/3 commits.
	for i := 0; i < 7; i++ {
		if bytes.Equal(privValidators[i].Address, val0.Address) {
			if cs.Commits.GetByAddress(val0.Address) == nil {
				t.Errorf("Proposer should already have signed a commit vote")
			}
			continue
		}
		vote := &blk.Vote{
			Height:     1,
			Round:      uint(i), // Doesn't matter what round
			Type:       blk.VoteTypeCommit,
			BlockHash:  cs.ProposalBlock.Hash(),
			BlockParts: cs.ProposalBlockParts.Header(),
		}
		err := privValidators[i].SignVote(vote)
		if err != nil {
			t.Error("Error signing vote: %v", err)
		}
		added, _, err := cs.AddVote(privValidators[i].Address, vote)
		if !added || err != nil {
			t.Errorf("Error adding commit: %v", err)
		}
	}

	// Test TryFinalizeCommit:
	cs.TryFinalizeCommit(1)
	<-cs.NewStepCh() // TODO: test this value too.
	checkRoundState(t, cs.GetRoundState(), 2, 0, RoundStepNewHeight)
}
