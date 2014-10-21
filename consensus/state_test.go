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
		Balance:  RandUInt64() + 1000, // At least 1000.
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
	s0.Save(time.Now())
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
		vote := &Vote{Height: 0, Round: 0, Type: voteType} // nil vote
		privAccounts[0].Sign(vote)
		cs.AddVote(vote)
	}

	// Ensure that vote appears in RoundState.
	rs0 := cs.GetRoundState()
	if vote := rs0.Prevotes.Get(0); vote == nil || vote.Type != VoteTypePrevote {
		t.Errorf("Expected to find prevote %v, not there", vote)
	}
	if vote := rs0.Precommits.Get(0); vote == nil || vote.Type != VoteTypePrecommit {
		t.Errorf("Expected to find precommit %v, not there", vote)
	}
	if vote := rs0.Commits.Get(0); vote == nil || vote.Type != VoteTypeCommit {
		t.Errorf("Expected to find commit %v, not there", vote)
	}

	// Setup round 1 (next round)
	cs.SetupRound(1)

	// Now the commit should be copied over to prevotes and precommits.
	rs1 := cs.GetRoundState()
	if vote := rs1.Prevotes.Get(0); vote == nil || vote.Type != VoteTypeCommit {
		t.Errorf("Expected to find commit %v, not there", vote)
	}
	if vote := rs1.Precommits.Get(0); vote == nil || vote.Type != VoteTypeCommit {
		t.Errorf("Expected to find commit %v, not there", vote)
	}
	if vote := rs1.Commits.Get(0); vote == nil || vote.Type != VoteTypeCommit {
		t.Errorf("Expected to find commit %v, not there", vote)
	}

	// Setup round 1 (should fail)
	{
		defer func() {
			if e := recover(); e == nil {
				t.Errorf("Expected to panic, round did not increment")
			}
		}()
		cs.SetupRound(1)
	}

}

func TestMakeProposalNoPrivValidator(t *testing.T) {
	cs, _ := makeConsensusState()
	cs.MakeProposal()
	rs := cs.GetRoundState()
	if rs.Proposal != nil {
		t.Error("Expected to make no proposal, since no privValidator")
	}
}

func TestMakeProposalEmptyMempool(t *testing.T) {
	cs, privAccounts := makeConsensusState()
	priv := NewPrivValidator(privAccounts[0], db_.NewMemDB())
	cs.SetPrivValidator(priv)

	cs.MakeProposal()
	rs := cs.GetRoundState()

	t.Log(rs.Proposal)
}
