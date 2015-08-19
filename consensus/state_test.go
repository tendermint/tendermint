package consensus

import (
	"bytes"
	"testing"

	_ "github.com/tendermint/tendermint/config/tendermint_test"
	"github.com/tendermint/tendermint/types"
)

/*

ProposeSuite
x * TestEnterProposeNoValidator - timeout into prevote round
x * TestEnterPropose - finish propose without timing out (we have the proposal)
x * TestBadProposal - 2 vals, bad proposal (bad block state hash), should prevote and precommit nil
FullRoundSuite
x * TestFullRound1 - 1 val, full successful round
x * TestFullRoundNil - 1 val, full round of nil
x * TestFullRound2 - 2 vals, both required for fuill round
LockSuite
x * TestLockNoPOL - 2 vals, tests for val 1 getting locked
  * TestLockPOL - 4 vals, one precommits, other 3 polka at next round, so we unlock and precomit the polka
  * TestNetworkLock - once +1/3 precommits, network should be locked
  * TestNetworkLockPOL - once +1/3 precommits, the block with more recent polka is committed
SlashingSuite
  * TestSlashingPrevotes - a validator prevoting twice in a round gets slashed
  * TestSlashingPrecommits - a validator precomitting twice in a round gets slashed
CatchupSuite
  * TestCatchup - if we might be behind and we've seen any 2/3 prevotes, round skip to new round, precommit, or prevote
HaltSuite
  * TestHalt1 -


// more(?)
// if the proposals pol round is the vote round and the proposal's complete, EnterPrevote for cs.Round (> vote.Round)
*/

//----------------------------------------------------------------------------------------------------
// ProposeSuite

// a non-validator should timeout into the prevote round
func TestEnterProposeNoPrivValidator(t *testing.T) {
	css, _ := simpleConsensusState(1)
	cs := css[0]

	// starts a go routine for EnterPropose
	cs.EnterNewRound(cs.Height, 0)

	// if we're not a validator, EnterPropose should timeout
	select {
	case rs := <-cs.NewStepCh():
		log.Info(rs.String())
		t.Fatal("Expected EnterPropose to timeout before going to EnterPrevote")
	case <-cs.timeoutChan:
		rs := cs.GetRoundState()
		if rs.Proposal != nil {
			t.Error("Expected to make no proposal, since no privValidator")
		}
		break
	}
}

// a validator should not timeout of the prevote round (TODO: unless the block is really big!)
func TestEnterPropose(t *testing.T) {
	css, privVals := simpleConsensusState(1)
	cs := css[0]
	cs.SetPrivValidator(privVals[0])

	// starts a go routine for EnterPropose
	cs.EnterNewRound(cs.Height, 0)

	// if we are a validator, we expect it not to timeout
	// and to be in PreVote
	select {
	case <-cs.NewStepCh():
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
		break
	case <-cs.timeoutChan:
		t.Fatal("Expected EnterPropose not to timeout")
	}
}

func TestBadProposal(t *testing.T) {
	css, privVals := simpleConsensusState(2)
	cs1, cs2 := css[0], css[1]
	cs1.newStepCh = make(chan *RoundState) // so it blocks

	cs1.SetPrivValidator(privVals[0])
	cs2.SetPrivValidator(privVals[1])

	// make the second validator the proposer
	_, v1 := cs1.Validators.GetByAddress(privVals[0].Address)
	v1.Accum, v1.VotingPower = 0, 0
	if updated := cs1.Validators.Update(v1); !updated {
		t.Fatal("failed to update validator")
	}
	_, v2 := cs1.Validators.GetByAddress(privVals[1].Address)
	v2.Accum, v2.VotingPower = 100, 100
	if updated := cs1.Validators.Update(v2); !updated {
		t.Fatal("failed to update validator")
	}

	// make the proposal
	propBlock, _ := cs2.createProposalBlock()
	if propBlock == nil {
		t.Fatal("Failed to create proposal block with cs2")
	}
	// make the block bad by tampering with statehash
	stateHash := propBlock.StateHash
	stateHash[0] = byte((stateHash[0] + 1) % 255)
	propBlock.StateHash = stateHash
	propBlockParts := propBlock.MakePartSet()
	proposal := types.NewProposal(cs2.Height, cs2.Round, propBlockParts.Header(), cs2.Votes.POLRound())
	if err := cs2.privValidator.SignProposal(cs2.state.ChainID, proposal); err != nil {
		t.Fatal("failed to sign bad proposal", err)
	}

	// start round
	cs1.EnterNewRound(cs1.Height, 0)

	// now we're on a new round and not the proposer, so wait for timeout
	<-cs1.timeoutChan
	// set the proposal block
	cs1.Proposal, cs1.ProposalBlock = proposal, propBlock
	// fix the voting powers
	// make the second validator the proposer
	_, v1 = cs1.Validators.GetByAddress(privVals[0].Address)
	_, v2 = cs1.Validators.GetByAddress(privVals[1].Address)
	v1.Accum, v1.VotingPower = v2.Accum, v2.VotingPower
	if updated := cs1.Validators.Update(v1); !updated {
		t.Fatal("failed to update validator")
	}
	// go to prevote, prevote for nil (proposal is bad)
	_, _ = <-cs1.NewStepCh(), <-cs1.NewStepCh()
	validatePrevote(t, cs1, 0, privVals[0], nil)

	addVoteToFrom(t, types.VoteTypePrevote, cs1, cs2, propBlock.Hash(), propBlock.MakePartSet().Header())
	_, _, _ = <-cs1.NewStepCh(), <-cs1.timeoutChan, <-cs1.NewStepCh()
	validatePrecommit(t, cs1, 0, 0, privVals[0], nil)
	addVoteToFrom(t, types.VoteTypePrecommit, cs1, cs2, propBlock.Hash(), propBlock.MakePartSet().Header())
}

//----------------------------------------------------------------------------------------------------
// FulLRoundSuite

// propose, prevote, and precommit a block
func TestFullRound1(t *testing.T) {
	css, privVals := simpleConsensusState(1)
	cs := css[0]
	cs.SetPrivValidator(privVals[0])

	// starts a go routine for EnterPropose
	cs.EnterNewRound(cs.Height, 0)
	// wait to finish propose and prevote
	_, _ = <-cs.NewStepCh(), <-cs.NewStepCh()

	// we should now be in precommit
	// verify our prevote is there
	validatePrevote(t, cs, 0, privVals[0], cs.ProposalBlock.Hash())
	// wait to finish precommit
	<-cs.NewStepCh()

	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs, 0, 0, privVals[0], cs.ProposalBlock)
}

// nil is proposed, so prevote and precommit nil
func TestFullRoundNil(t *testing.T) {
	css, privVals := simpleConsensusState(1)
	cs := css[0]
	cs.newStepCh = make(chan *RoundState) // so it blocks

	// starts a go routine for EnterPropose
	cs.EnterNewRound(cs.Height, 0)

	// wait to finish propose (we should time out)
	<-cs.timeoutChan
	cs.SetPrivValidator(privVals[0])
	<-cs.NewStepCh()

	// wait to finish prevote
	<-cs.NewStepCh()

	// we should now be in precommit
	// verify our vote is there
	validatePrevote(t, cs, 0, privVals[0], nil)
	// wait to finish precommit
	<-cs.NewStepCh()
	// we should have precommitted nil
	validatePrecommit(t, cs, 0, 0, privVals[0], nil)
}

// run through propose, prevote, precommit commit with two validators
// where the first validator has to wait for votes from the second
func TestFullRound2(t *testing.T) {
	css, privVals := simpleConsensusState(2)
	cs1, cs2 := css[0], css[1]
	cs1.newStepCh = make(chan *RoundState) // so it blocks
	cs2.newStepCh = make(chan *RoundState) // so it blocks

	cs1.SetPrivValidator(privVals[0])
	cs2.SetPrivValidator(privVals[1])

	// start round and wait for propose and prevote
	cs1.EnterNewRound(cs1.Height, 0)
	_, _ = <-cs1.NewStepCh(), <-cs1.NewStepCh()

	// we should now be stuck in limbo forever, waiting for more prevotes
	ensureNoNewStep(t, cs1)

	// prevote arrives from cs2:
	addVoteToFrom(t, types.VoteTypePrevote, cs1, cs2, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header())

	// wait to finish precommit
	<-cs1.NewStepCh()

	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, 0, 0, privVals[0], cs1.ProposalBlock)

	// we should now be stuck in limbo forever, waiting for more precommits
	ensureNoNewStep(t, cs1)

	// precommit arrives from cs2:
	addVoteToFrom(t, types.VoteTypePrecommit, cs1, cs2, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header())

	// wait to finish commit, propose in next height
	_, rs := <-cs1.NewStepCh(), <-cs1.NewStepCh()
	if rs.Height != 2 {
		t.Fatal("Expected height to increment")
	}
}

//------------------------------------------------------------------------------------------
// LockSuite

// two validators, a series of rounds in which the first validator is locked
func TestLockNoPOL(t *testing.T) {
	css, privVals := simpleConsensusState(2)
	cs1, cs2 := css[0], css[1]
	cs1.newStepCh = make(chan *RoundState) // so it blocks

	cs1.SetPrivValidator(privVals[0])
	cs2.SetPrivValidator(privVals[1])

	/* Round 1
	Proposer: cs1 proposes B
	Prevotes:
		cs1: B
		cs2: B
	Precommits:
		cs1: B
		cs2: B2
	*/

	// start round and wait for propose and prevote
	cs1.EnterNewRound(cs1.Height, 0)
	_, _ = <-cs1.NewStepCh(), <-cs1.NewStepCh()

	// we should now be stuck in limbo forever, waiting for more prevotes
	// prevote arrives from cs2:
	addVoteToFrom(t, types.VoteTypePrevote, cs1, cs2, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header())

	// wait to finish precommit
	<-cs1.NewStepCh()

	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, 0, 0, privVals[0], cs1.ProposalBlock)

	// we should now be stuck in limbo forever, waiting for more precommits
	// lets add one for a different block
	// NOTE: in practice we should never get to a point where there are precommits for different blocks at the same round
	hash := cs1.ProposalBlock.Hash()
	hash[0] = byte((hash[0] + 1) % 255)
	addVoteToFrom(t, types.VoteTypePrecommit, cs1, cs2, hash, cs1.ProposalBlockParts.Header())

	// (note we're entering precommit for a second time this round)
	// but with invalid args. then we EnterPrecommitWait, and the timeout to new round
	_, _ = <-cs1.NewStepCh(), <-cs1.timeoutChan

	/*Round2
	Proposer: cs1 reproposes B
	Prevotes:
		cs1: B
		cs2: B2
	*/

	cs2.Round += 1

	// go to prevote
	<-cs1.NewStepCh()

	// now we're on a new round and the proposer
	if cs1.ProposalBlock != cs1.LockedBlock {
		t.Fatalf("Expected proposal block to be locked block. Got %v, Expected %v", cs1.ProposalBlock, cs1.LockedBlock)
	}

	// wait to finish prevote
	<-cs1.NewStepCh()

	// we should have prevoted our locked block
	validatePrevote(t, cs1, 1, privVals[0], cs1.LockedBlock.Hash())

	// add a conflicting prevote from the other validator
	addVoteToFrom(t, types.VoteTypePrevote, cs1, cs2, hash, cs1.ProposalBlockParts.Header())

	// now we're going to enter prevote again, but with invalid args
	// and then prevote wait, which should timeout. then wait for precommit
	_, _, _ = <-cs1.NewStepCh(), <-cs1.timeoutChan, <-cs1.NewStepCh()

	// the proposed block should still be locked and our precommit added
	// TODO: we should actually be precommitting nil
	validatePrecommit(t, cs1, 1, 0, privVals[0], cs1.ProposalBlock)

	// add conflicting precommit from cs2
	// NOTE: in practice we should never get to a point where there are precommits for different blocks at the same round
	addVoteToFrom(t, types.VoteTypePrecommit, cs1, cs2, hash, cs1.ProposalBlockParts.Header())

	// (note we're entering precommit for a second time this round, but with invalid args
	// then we EnterPrecommitWait and timeout into NewRound
	_, _ = <-cs1.NewStepCh(), <-cs1.timeoutChan

	/* Round3
	Proposer: cs2, but cs1 doesn't receive anything
	Prevotes:
		cs1: B
		cs2: B2
	*/

	cs2.Round += 1

	// now we're on a new round and not the proposer, so wait for timeout
	<-cs1.timeoutChan
	if cs1.ProposalBlock != nil {
		t.Fatal("Expected proposal block to be nil")
	}
	// go to prevote, prevote for locked block
	_, _ = <-cs1.NewStepCh(), <-cs1.NewStepCh()
	validatePrevote(t, cs1, 0, privVals[0], cs1.LockedBlock.Hash())

	// TODO: quick fastforward to new round, set proposer
	addVoteToFrom(t, types.VoteTypePrevote, cs1, cs2, hash, cs1.ProposalBlockParts.Header())
	_, _, _ = <-cs1.NewStepCh(), <-cs1.timeoutChan, <-cs1.NewStepCh()
	validatePrecommit(t, cs1, 2, 0, privVals[0], cs1.LockedBlock)                              // TODO: should be nil
	addVoteToFrom(t, types.VoteTypePrecommit, cs1, cs2, hash, cs1.ProposalBlockParts.Header()) // NOTE: conflicting precommits at same height

	<-cs1.NewStepCh()

	// before we time out into new round, set next proposer
	// and next proposal block
	_, v1 := cs1.Validators.GetByAddress(privVals[0].Address)
	v1.VotingPower = 1
	if updated := cs1.Validators.Update(v1); !updated {
		t.Fatal("failed to update validator")
	}

	cs2.decideProposal(cs2.Height, cs2.Round+1)
	prop, propBlock := cs2.Proposal, cs2.ProposalBlock
	if prop == nil || propBlock == nil {
		t.Fatal("Failed to create proposal block with cs2")
	}

	cs2.Round += 1

	<-cs1.timeoutChan

	/* Round4
	Proposer: cs2 proposes C
	Prevotes:
		cs1: B
		cs2: C
	Precommits:
		cs1: B
		cs2: C
	*/

	// now we're on a new round and not the proposer, so wait for timeout
	<-cs1.timeoutChan
	// set the proposal block
	cs1.Proposal, cs1.ProposalBlock = prop, propBlock
	// go to prevote, prevote for locked block (not proposal)
	_, _ = <-cs1.NewStepCh(), <-cs1.NewStepCh()
	validatePrevote(t, cs1, 0, privVals[0], cs1.LockedBlock.Hash())

	addVoteToFrom(t, types.VoteTypePrevote, cs1, cs2, propBlock.Hash(), propBlock.MakePartSet().Header())
	_, _, _ = <-cs1.NewStepCh(), <-cs1.timeoutChan, <-cs1.NewStepCh()
	validatePrecommit(t, cs1, 2, 0, privVals[0], cs1.LockedBlock)                                           // TODO: should be nil
	addVoteToFrom(t, types.VoteTypePrecommit, cs1, cs2, propBlock.Hash(), propBlock.MakePartSet().Header()) // NOTE: conflicting precommits at same height
}

// once +2/3 prevote and +1/3 precommit for a block, the network will forever be locked on it
func TestNetworkLock(t *testing.T) {
	css, privVals := simpleConsensusState(4)
	cs1, cs2, cs3, cs4 := css[0], css[1], css[2], css[3]
	cs1.newStepCh = make(chan *RoundState) // so it blocks

	cs1.SetPrivValidator(privVals[0])
	cs2.SetPrivValidator(privVals[1])
	cs3.SetPrivValidator(privVals[2])
	cs4.SetPrivValidator(privVals[3])

	// everything done from perspective of cs1

	/* Round 1
	Proposer: cs1 proposes B
	Prevotes:
		cs1: B
		cs2: B
		cs3: B
		cs4: nil // eg. didn't get proposal
	Precommits:
		cs1: B
		cs2: B
		cs3: _ // eg. we didn't see it
		cs4: nil // eg. didn't see the 2/3 prevotes

	NOTE: now that +1/3 has precommitted for B, the network is locked on B
	*/

	// start round and wait for propose and prevote
	cs1.EnterNewRound(cs1.Height, 0)
	_, _ = <-cs1.NewStepCh(), <-cs1.NewStepCh()

	// wait to finish precommit after prevotes done
	// we do this in a go routine with another channel since otherwise
	// we may get deadlock with EnterPrecommit waiting to send on newStepCh and the final
	// addVoteToFrom waiting for the cs.mtx.Lock
	donePrecommits := make(chan struct{})
	go func() {
		<-cs1.NewStepCh()
		donePrecommits <- struct{}{}
	}()
	addVoteToFrom(t, types.VoteTypePrevote, cs1, cs2, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header())
	addVoteToFrom(t, types.VoteTypePrevote, cs1, cs3, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header())
	addVoteToFrom(t, types.VoteTypePrevote, cs1, cs4, nil, types.PartSetHeader{})

	<-donePrecommits

	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, 0, 0, privVals[0], cs1.ProposalBlock)

	// add precommits from cs2 and cs4 (we failed to get one from cs3)
	addVoteToFrom(t, types.VoteTypePrecommit, cs1, cs2, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header())
	addVoteToFrom(t, types.VoteTypePrecommit, cs1, cs4, nil, types.PartSetHeader{})

	// (note we're entering precommit for a second time this round)
	// but with invalid args. then we EnterPrecommitWait, and the timeout to new round
	_, _ = <-cs1.NewStepCh(), <-cs1.timeoutChan

	/*Round2
	Proposer: cs1 reproposes B
	Prevotes:
		cs1: B
		cs2: B
		cs3: B
	*/

	// TODO: actually test something here ...

}

//------------------------------------------------------------------------------------------
// SlashingSuite

// slashing requires the
func TestSlashingPrevotes(t *testing.T) {
	css, privVals := simpleConsensusState(2)
	cs1, cs2 := css[0], css[1]
	cs1.newStepCh = make(chan *RoundState) // so it blocks

	cs1.SetPrivValidator(privVals[0])
	cs2.SetPrivValidator(privVals[1])

	// start round and wait for propose and prevote
	cs1.EnterNewRound(cs1.Height, 0)
	_, _ = <-cs1.NewStepCh(), <-cs1.NewStepCh()

	// we should now be stuck in limbo forever, waiting for more prevotes
	// add one for a different block should cause us to go into prevote wait
	hash := cs1.ProposalBlock.Hash()
	hash[0] = byte(hash[0]+1) % 255
	addVoteToFrom(t, types.VoteTypePrevote, cs1, cs2, hash, cs1.ProposalBlockParts.Header())

	// pass prevote wait
	<-cs1.NewStepCh()

	// NOTE: we have to send the vote for different block first so we don't just go into precommit round right
	// away and ignore more prevotes (and thus fail to slash!)

	// add the conflicting vote
	addVoteToFrom(t, types.VoteTypePrevote, cs1, cs2, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header())

	// conflicting vote should cause us to broadcast dupeout tx on mempool
	txs := cs1.mempoolReactor.Mempool.GetProposalTxs()
	if len(txs) != 1 {
		t.Fatal("expected to find a transaction in the mempool after double signing")
	}
	dupeoutTx, ok := txs[0].(*types.DupeoutTx)
	if !ok {
		t.Fatal("expected to find DupeoutTx in mempool after double signing")
	}

	if !bytes.Equal(dupeoutTx.Address, cs2.privValidator.Address) {
		t.Fatalf("expected DupeoutTx for %X, got %X", cs2.privValidator.Address, dupeoutTx.Address)
	}

	// TODO: validate the sig
}

func TestSlashingPrecommits(t *testing.T) {
	css, privVals := simpleConsensusState(2)
	cs1, cs2 := css[0], css[1]
	cs1.newStepCh = make(chan *RoundState) // so it blocks

	cs1.SetPrivValidator(privVals[0])
	cs2.SetPrivValidator(privVals[1])

	// start round and wait for propose and prevote
	cs1.EnterNewRound(cs1.Height, 0)
	_, _ = <-cs1.NewStepCh(), <-cs1.NewStepCh()

	// add prevote from cs2
	addVoteToFrom(t, types.VoteTypePrevote, cs1, cs2, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header())

	// wait to finish precommit
	<-cs1.NewStepCh()

	// we should now be stuck in limbo forever, waiting for more prevotes
	// add one for a different block should cause us to go into prevote wait
	hash := cs1.ProposalBlock.Hash()
	hash[0] = byte(hash[0]+1) % 255
	addVoteToFrom(t, types.VoteTypePrecommit, cs1, cs2, hash, cs1.ProposalBlockParts.Header())

	// pass prevote wait
	<-cs1.NewStepCh()

	// NOTE: we have to send the vote for different block first so we don't just go into precommit round right
	// away and ignore more prevotes (and thus fail to slash!)

	// add precommit from cs2
	addVoteToFrom(t, types.VoteTypePrecommit, cs1, cs2, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header())

	// conflicting vote should cause us to broadcast dupeout tx on mempool
	txs := cs1.mempoolReactor.Mempool.GetProposalTxs()
	if len(txs) != 1 {
		t.Fatal("expected to find a transaction in the mempool after double signing")
	}
	dupeoutTx, ok := txs[0].(*types.DupeoutTx)
	if !ok {
		t.Fatal("expected to find DupeoutTx in mempool after double signing")
	}

	if !bytes.Equal(dupeoutTx.Address, cs2.privValidator.Address) {
		t.Fatalf("expected DupeoutTx for %X, got %X", cs2.privValidator.Address, dupeoutTx.Address)
	}

	// TODO: validate the sig

}

//------------------------------------------------------------------------------------------
// CatchupSuite

//------------------------------------------------------------------------------------------
// HaltSuite
