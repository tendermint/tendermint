package consensus

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/tendermint/tendermint/config/tendermint_test"
	//"github.com/tendermint/go-events"
	"github.com/tendermint/tendermint/types"
)

func init() {
	tendermint_test.ResetConfig("consensus_state_test")
}

func (tp *TimeoutParams) ensureProposeTimeout() time.Duration {
	return time.Duration(tp.Propose0*2) * time.Millisecond
}

/*

ProposeSuite
x * TestProposerSelection0 - round robin ordering, round 0
x * TestProposerSelection2 - round robin ordering, round 2++
x * TestEnterProposeNoValidator - timeout into prevote round
x * TestEnterPropose - finish propose without timing out (we have the proposal)
x * TestBadProposal - 2 vals, bad proposal (bad block state hash), should prevote and precommit nil
FullRoundSuite
x * TestFullRound1 - 1 val, full successful round
x * TestFullRoundNil - 1 val, full round of nil
x * TestFullRound2 - 2 vals, both required for fuill round
LockSuite
x * TestLockNoPOL - 2 vals, 4 rounds. one val locked, precommits nil every round except first.
x * TestLockPOLRelock - 4 vals, one precommits, other 3 polka at next round, so we unlock and precomit the polka
x * TestLockPOLUnlock - 4 vals, one precommits, other 3 polka nil at next round, so we unlock and precomit nil
x * TestLockPOLSafety1 - 4 vals. We shouldn't change lock based on polka at earlier round
x * TestLockPOLSafety2 - 4 vals. After unlocking, we shouldn't relock based on polka at earlier round
  * TestNetworkLock - once +1/3 precommits, network should be locked
  * TestNetworkLockPOL - once +1/3 precommits, the block with more recent polka is committed
SlashingSuite
x * TestSlashingPrevotes - a validator prevoting twice in a round gets slashed
x * TestSlashingPrecommits - a validator precomitting twice in a round gets slashed
CatchupSuite
  * TestCatchup - if we might be behind and we've seen any 2/3 prevotes, round skip to new round, precommit, or prevote
HaltSuite
x * TestHalt1 - if we see +2/3 precommits after timing out into new round, we should still commit

*/

//----------------------------------------------------------------------------------------------------
// ProposeSuite

func TestProposerSelection0(t *testing.T) {
	cs1, vss := randConsensusState(4)
	height, round := cs1.Height, cs1.Round

	newRoundCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringNewRound(), 1)
	proposalCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringCompleteProposal(), 1)

	startTestRound(cs1, height, round)

	// wait for new round so proposer is set
	<-newRoundCh

	// lets commit a block and ensure proposer for the next height is correct
	prop := cs1.GetRoundState().Validators.Proposer()
	if !bytes.Equal(prop.Address, cs1.privValidator.Address) {
		t.Fatalf("expected proposer to be validator %d. Got %X", 0, prop.Address)
	}

	// wait for complete proposal
	<-proposalCh

	rs := cs1.GetRoundState()
	signAddVoteToFromMany(types.VoteTypePrecommit, cs1, rs.ProposalBlock.Hash(), rs.ProposalBlockParts.Header(), vss[1:]...)

	// wait for new round so next validator is set
	<-newRoundCh

	prop = cs1.GetRoundState().Validators.Proposer()
	if !bytes.Equal(prop.Address, vss[1].Address) {
		t.Fatalf("expected proposer to be validator %d. Got %X", 1, prop.Address)
	}
}

// Now let's do it all again, but starting from round 2 instead of 0
func TestProposerSelection2(t *testing.T) {
	cs1, vss := randConsensusState(4) // test needs more work for more than 3 validators

	newRoundCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringNewRound(), 1)

	// this time we jump in at round 2
	incrementRound(vss[1:]...)
	incrementRound(vss[1:]...)
	startTestRound(cs1, cs1.Height, 2)

	<-newRoundCh // wait for the new round

	// everyone just votes nil. we get a new proposer each round
	for i := 0; i < len(vss); i++ {
		prop := cs1.GetRoundState().Validators.Proposer()
		if !bytes.Equal(prop.Address, vss[(i+2)%len(vss)].Address) {
			t.Fatalf("expected proposer to be validator %d. Got %X", (i+2)%len(vss), prop.Address)
		}

		rs := cs1.GetRoundState()
		signAddVoteToFromMany(types.VoteTypePrecommit, cs1, nil, rs.ProposalBlockParts.Header(), vss[1:]...)
		<-newRoundCh // wait for the new round event each round

		incrementRound(vss[1:]...)
	}

}

// a non-validator should timeout into the prevote round
func TestEnterProposeNoPrivValidator(t *testing.T) {
	cs, _ := randConsensusState(1)
	cs.SetPrivValidator(nil)
	height, round := cs.Height, cs.Round

	// Listen for propose timeout event
	timeoutCh := subscribeToEvent(cs.evsw, "tester", types.EventStringTimeoutPropose(), 1)

	startTestRound(cs, height, round)

	// if we're not a validator, EnterPropose should timeout
	ticker := time.NewTicker(cs.timeoutParams.ensureProposeTimeout())
	select {
	case <-timeoutCh:
	case <-ticker.C:
		t.Fatal("Expected EnterPropose to timeout")

	}

	if cs.GetRoundState().Proposal != nil {
		t.Error("Expected to make no proposal, since no privValidator")
	}
}

// a validator should not timeout of the prevote round (TODO: unless the block is really big!)
func TestEnterProposeYesPrivValidator(t *testing.T) {
	cs, _ := randConsensusState(1)
	height, round := cs.Height, cs.Round

	// Listen for propose timeout event

	timeoutCh := subscribeToEvent(cs.evsw, "tester", types.EventStringTimeoutPropose(), 1)
	proposalCh := subscribeToEvent(cs.evsw, "tester", types.EventStringCompleteProposal(), 1)

	cs.enterNewRound(height, round)
	cs.startRoutines(3)

	<-proposalCh

	// Check that Proposal, ProposalBlock, ProposalBlockParts are set.
	rs := cs.GetRoundState()
	if rs.Proposal == nil {
		t.Error("rs.Proposal should be set")
	}
	if rs.ProposalBlock == nil {
		t.Error("rs.ProposalBlock should be set")
	}
	if rs.ProposalBlockParts.Total() == 0 {
		t.Error("rs.ProposalBlockParts should be set")
	}

	// if we're a validator, enterPropose should not timeout
	ticker := time.NewTicker(cs.timeoutParams.ensureProposeTimeout())
	select {
	case <-timeoutCh:
		t.Fatal("Expected EnterPropose not to timeout")
	case <-ticker.C:

	}
}

func TestBadProposal(t *testing.T) {
	cs1, vss := randConsensusState(2)
	height, round := cs1.Height, cs1.Round
	cs2 := vss[1]

	proposalCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringCompleteProposal(), 1)
	voteCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringVote(), 1)

	propBlock, _ := cs1.createProposalBlock() //changeProposer(t, cs1, cs2)

	// make the second validator the proposer by incrementing round
	round = round + 1
	incrementRound(vss[1:]...)

	// make the block bad by tampering with statehash
	stateHash := propBlock.AppHash
	if len(stateHash) == 0 {
		stateHash = make([]byte, 32)
	}
	stateHash[0] = byte((stateHash[0] + 1) % 255)
	propBlock.AppHash = stateHash
	propBlockParts := propBlock.MakePartSet()
	proposal := types.NewProposal(cs2.Height, round, propBlockParts.Header(), -1)
	if err := cs2.SignProposal(chainID, proposal); err != nil {
		t.Fatal("failed to sign bad proposal", err)
	}

	// set the proposal block
	cs1.SetProposalAndBlock(proposal, propBlock, propBlockParts, "some peer")

	// start the machine
	startTestRound(cs1, height, round)

	// wait for proposal
	<-proposalCh

	// wait for prevote
	<-voteCh

	validatePrevote(t, cs1, round, vss[0], nil)

	// add bad prevote from cs2 and wait for it
	signAddVoteToFrom(types.VoteTypePrevote, cs1, cs2, propBlock.Hash(), propBlock.MakePartSet().Header())
	<-voteCh

	// wait for precommit
	<-voteCh

	validatePrecommit(t, cs1, round, 0, vss[0], nil, nil)
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs2, propBlock.Hash(), propBlock.MakePartSet().Header())
}

//----------------------------------------------------------------------------------------------------
// FullRoundSuite

// propose, prevote, and precommit a block
func TestFullRound1(t *testing.T) {
	cs, vss := randConsensusState(1)
	height, round := cs.Height, cs.Round

	voteCh := subscribeToEvent(cs.evsw, "tester", types.EventStringVote(), 1)
	propCh := subscribeToEvent(cs.evsw, "tester", types.EventStringCompleteProposal(), 1)
	newRoundCh := subscribeToEvent(cs.evsw, "tester", types.EventStringNewRound(), 1)

	startTestRound(cs, height, round)

	<-newRoundCh

	// grab proposal
	re := <-propCh
	propBlockHash := re.(types.EventDataRoundState).RoundState.(*RoundState).ProposalBlock.Hash()

	<-voteCh // wait for prevote
	validatePrevote(t, cs, round, vss[0], propBlockHash)

	<-voteCh // wait for precommit

	// we're going to roll right into new height
	<-newRoundCh

	validateLastPrecommit(t, cs, vss[0], propBlockHash)
}

// nil is proposed, so prevote and precommit nil
func TestFullRoundNil(t *testing.T) {
	cs, vss := randConsensusState(1)
	height, round := cs.Height, cs.Round

	voteCh := subscribeToEvent(cs.evsw, "tester", types.EventStringVote(), 1)

	cs.enterPrevote(height, round)
	cs.startRoutines(4)

	<-voteCh // prevote
	<-voteCh // precommit

	// should prevote and precommit nil
	validatePrevoteAndPrecommit(t, cs, round, 0, vss[0], nil, nil)
}

// run through propose, prevote, precommit commit with two validators
// where the first validator has to wait for votes from the second
func TestFullRound2(t *testing.T) {
	cs1, vss := randConsensusState(2)
	cs2 := vss[1]
	height, round := cs1.Height, cs1.Round

	voteCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringVote(), 1)
	newBlockCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringNewBlock(), 1)

	// start round and wait for propose and prevote
	startTestRound(cs1, height, round)

	<-voteCh // prevote

	// we should be stuck in limbo waiting for more prevotes

	propBlockHash, propPartsHeader := cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header()

	// prevote arrives from cs2:
	signAddVoteToFrom(types.VoteTypePrevote, cs1, cs2, propBlockHash, propPartsHeader)
	<-voteCh

	<-voteCh //precommit

	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, 0, 0, vss[0], propBlockHash, propBlockHash)

	// we should be stuck in limbo waiting for more precommits

	// precommit arrives from cs2:
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs2, propBlockHash, propPartsHeader)
	<-voteCh

	// wait to finish commit, propose in next height
	<-newBlockCh
}

//------------------------------------------------------------------------------------------
// LockSuite

// two validators, 4 rounds.
// two vals take turns proposing. val1 locks on first one, precommits nil on everything else
func TestLockNoPOL(t *testing.T) {
	cs1, vss := randConsensusState(2)
	cs2 := vss[1]
	height := cs1.Height

	timeoutProposeCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringTimeoutPropose(), 1)
	timeoutWaitCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringTimeoutWait(), 1)
	voteCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringVote(), 1)
	proposalCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringCompleteProposal(), 1)
	newRoundCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringNewRound(), 1)

	/*
		Round1 (cs1, B) // B B // B B2
	*/

	// start round and wait for prevote
	cs1.enterNewRound(height, 0)
	cs1.startRoutines(0)

	re := <-proposalCh
	rs := re.(types.EventDataRoundState).RoundState.(*RoundState)
	theBlockHash := rs.ProposalBlock.Hash()

	<-voteCh // prevote

	// we should now be stuck in limbo forever, waiting for more prevotes
	// prevote arrives from cs2:
	signAddVoteToFrom(types.VoteTypePrevote, cs1, cs2, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header())
	<-voteCh // prevote

	<-voteCh // precommit

	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, 0, 0, vss[0], theBlockHash, theBlockHash)

	// we should now be stuck in limbo forever, waiting for more precommits
	// lets add one for a different block
	// NOTE: in practice we should never get to a point where there are precommits for different blocks at the same round
	hash := make([]byte, len(theBlockHash))
	copy(hash, theBlockHash)
	hash[0] = byte((hash[0] + 1) % 255)
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs2, hash, rs.ProposalBlock.MakePartSet().Header())
	<-voteCh // precommit

	// (note we're entering precommit for a second time this round)
	// but with invalid args. then we enterPrecommitWait, and the timeout to new round
	<-timeoutWaitCh

	///

	<-newRoundCh
	log.Notice("#### ONTO ROUND 1")
	/*
		Round2 (cs1, B) // B B2
	*/

	incrementRound(cs2)

	// now we're on a new round and not the proposer, so wait for timeout
	re = <-timeoutProposeCh
	rs = re.(types.EventDataRoundState).RoundState.(*RoundState)

	if rs.ProposalBlock != nil {
		t.Fatal("Expected proposal block to be nil")
	}

	// wait to finish prevote
	<-voteCh

	// we should have prevoted our locked block
	validatePrevote(t, cs1, 1, vss[0], rs.LockedBlock.Hash())

	// add a conflicting prevote from the other validator
	signAddVoteToFrom(types.VoteTypePrevote, cs1, cs2, hash, rs.ProposalBlock.MakePartSet().Header())
	<-voteCh

	// now we're going to enter prevote again, but with invalid args
	// and then prevote wait, which should timeout. then wait for precommit
	<-timeoutWaitCh

	<-voteCh // precommit

	// the proposed block should still be locked and our precommit added
	// we should precommit nil and be locked on the proposal
	validatePrecommit(t, cs1, 1, 0, vss[0], nil, theBlockHash)

	// add conflicting precommit from cs2
	// NOTE: in practice we should never get to a point where there are precommits for different blocks at the same round
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs2, hash, rs.ProposalBlock.MakePartSet().Header())
	<-voteCh

	// (note we're entering precommit for a second time this round, but with invalid args
	// then we enterPrecommitWait and timeout into NewRound
	<-timeoutWaitCh

	<-newRoundCh
	log.Notice("#### ONTO ROUND 2")
	/*
		Round3 (cs2, _) // B, B2
	*/

	incrementRound(cs2)

	re = <-proposalCh
	rs = re.(types.EventDataRoundState).RoundState.(*RoundState)

	// now we're on a new round and are the proposer
	if !bytes.Equal(rs.ProposalBlock.Hash(), rs.LockedBlock.Hash()) {
		t.Fatalf("Expected proposal block to be locked block. Got %v, Expected %v", rs.ProposalBlock, rs.LockedBlock)
	}

	<-voteCh // prevote

	validatePrevote(t, cs1, 2, vss[0], rs.LockedBlock.Hash())

	signAddVoteToFrom(types.VoteTypePrevote, cs1, cs2, hash, rs.ProposalBlock.MakePartSet().Header())
	<-voteCh

	<-timeoutWaitCh // prevote wait
	<-voteCh        // precommit

	validatePrecommit(t, cs1, 2, 0, vss[0], nil, theBlockHash)                                          // precommit nil but be locked on proposal
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs2, hash, rs.ProposalBlock.MakePartSet().Header()) // NOTE: conflicting precommits at same height
	<-voteCh

	<-timeoutWaitCh

	// before we time out into new round, set next proposal block
	prop, propBlock := decideProposal(cs1, cs2, cs2.Height, cs2.Round+1)
	if prop == nil || propBlock == nil {
		t.Fatal("Failed to create proposal block with cs2")
	}

	incrementRound(cs2)

	<-newRoundCh
	log.Notice("#### ONTO ROUND 3")
	/*
		Round4 (cs2, C) // B C // B C
	*/

	// now we're on a new round and not the proposer
	// so set the proposal block
	cs1.SetProposalAndBlock(prop, propBlock, propBlock.MakePartSet(), "")

	<-proposalCh
	<-voteCh // prevote

	// prevote for locked block (not proposal)
	validatePrevote(t, cs1, 0, vss[0], cs1.LockedBlock.Hash())

	signAddVoteToFrom(types.VoteTypePrevote, cs1, cs2, propBlock.Hash(), propBlock.MakePartSet().Header())
	<-voteCh

	<-timeoutWaitCh
	<-voteCh

	validatePrecommit(t, cs1, 2, 0, vss[0], nil, theBlockHash)                                               // precommit nil but locked on proposal
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs2, propBlock.Hash(), propBlock.MakePartSet().Header()) // NOTE: conflicting precommits at same height
	<-voteCh
}

// 4 vals, one precommits, other 3 polka at next round, so we unlock and precomit the polka
func TestLockPOLRelock(t *testing.T) {
	cs1, vss := randConsensusState(4)
	cs2, cs3, cs4 := vss[1], vss[2], vss[3]

	timeoutProposeCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringTimeoutPropose(), 1)
	timeoutWaitCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringTimeoutWait(), 1)
	proposalCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringCompleteProposal(), 1)
	voteCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringVote(), 1)
	newRoundCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringNewRound(), 1)
	newBlockCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringNewBlock(), 1)

	log.Debug("cs2 last round", "lr", cs2.PrivValidator.LastRound)

	// everything done from perspective of cs1

	/*
		Round1 (cs1, B) // B B B B// B nil B nil

		eg. cs2 and cs4 didn't see the 2/3 prevotes
	*/

	// start round and wait for propose and prevote
	startTestRound(cs1, cs1.Height, 0)

	<-newRoundCh
	re := <-proposalCh
	rs := re.(types.EventDataRoundState).RoundState.(*RoundState)
	theBlockHash := rs.ProposalBlock.Hash()

	<-voteCh // prevote

	signAddVoteToFromMany(types.VoteTypePrevote, cs1, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header(), cs2, cs3, cs4)
	_, _, _ = <-voteCh, <-voteCh, <-voteCh // prevotes

	<-voteCh // our precommit
	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, 0, 0, vss[0], theBlockHash, theBlockHash)

	// add precommits from the rest
	signAddVoteToFromMany(types.VoteTypePrecommit, cs1, nil, types.PartSetHeader{}, cs2, cs4)
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs3, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header())
	_, _, _ = <-voteCh, <-voteCh, <-voteCh // precommits

	// before we timeout to the new round set the new proposal
	prop, propBlock := decideProposal(cs1, cs2, cs2.Height, cs2.Round+1)
	propBlockParts := propBlock.MakePartSet()
	propBlockHash := propBlock.Hash()

	incrementRound(cs2, cs3, cs4)

	// timeout to new round
	<-timeoutWaitCh

	//XXX: this isnt gauranteed to get there before the timeoutPropose ...
	cs1.SetProposalAndBlock(prop, propBlock, propBlockParts, "some peer")

	<-newRoundCh
	log.Notice("### ONTO ROUND 1")

	/*
		Round2 (cs2, C) // B C C C // C C C _)

		cs1 changes lock!
	*/

	// now we're on a new round and not the proposer
	// but we should receive the proposal
	select {
	case <-proposalCh:
	case <-timeoutProposeCh:
		<-proposalCh
	}

	// go to prevote, prevote for locked block (not proposal), move on
	<-voteCh
	validatePrevote(t, cs1, 0, vss[0], theBlockHash)

	// now lets add prevotes from everyone else for the new block
	signAddVoteToFromMany(types.VoteTypePrevote, cs1, propBlockHash, propBlockParts.Header(), cs2, cs3, cs4)
	_, _, _ = <-voteCh, <-voteCh, <-voteCh // prevotes

	// now either we go to PrevoteWait or Precommit
	select {
	case <-timeoutWaitCh: // we're in PrevoteWait, go to Precommit
		<-voteCh
	case <-voteCh: // we went straight to Precommit
	}

	// we should have unlocked and locked on the new block
	validatePrecommit(t, cs1, 1, 1, vss[0], propBlockHash, propBlockHash)

	signAddVoteToFromMany(types.VoteTypePrecommit, cs1, propBlockHash, propBlockParts.Header(), cs2, cs3)
	_, _ = <-voteCh, <-voteCh

	be := <-newBlockCh
	b := be.(types.EventDataNewBlock)
	re = <-newRoundCh
	rs = re.(types.EventDataRoundState).RoundState.(*RoundState)
	if rs.Height != 2 {
		t.Fatal("Expected height to increment")
	}

	if !bytes.Equal(b.Block.Hash(), propBlockHash) {
		t.Fatal("Expected new block to be proposal block")
	}
}

// 4 vals, one precommits, other 3 polka at next round, so we unlock and precomit the polka
func TestLockPOLUnlock(t *testing.T) {
	cs1, vss := randConsensusState(4)
	cs2, cs3, cs4 := vss[1], vss[2], vss[3]

	proposalCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringCompleteProposal(), 1)
	timeoutProposeCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringTimeoutPropose(), 1)
	timeoutWaitCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringTimeoutWait(), 1)
	newRoundCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringNewRound(), 1)
	unlockCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringUnlock(), 1)
	voteCh := subscribeToVoter(cs1, cs1.privValidator.Address)

	// everything done from perspective of cs1

	/*
		Round1 (cs1, B) // B B B B // B nil B nil

		eg. didn't see the 2/3 prevotes
	*/

	// start round and wait for propose and prevote
	startTestRound(cs1, cs1.Height, 0)
	<-newRoundCh
	re := <-proposalCh
	rs := re.(types.EventDataRoundState).RoundState.(*RoundState)
	theBlockHash := rs.ProposalBlock.Hash()

	<-voteCh // prevote

	signAddVoteToFromMany(types.VoteTypePrevote, cs1, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header(), cs2, cs3, cs4)

	<-voteCh //precommit

	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, 0, 0, vss[0], theBlockHash, theBlockHash)

	// add precommits from the rest
	signAddVoteToFromMany(types.VoteTypePrecommit, cs1, nil, types.PartSetHeader{}, cs2, cs4)
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs3, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header())

	// before we time out into new round, set next proposal block
	prop, propBlock := decideProposal(cs1, cs2, cs2.Height, cs2.Round+1)
	propBlockParts := propBlock.MakePartSet()

	incrementRound(cs2, cs3, cs4)

	// timeout to new round
	re = <-timeoutWaitCh
	rs = re.(types.EventDataRoundState).RoundState.(*RoundState)
	lockedBlockHash := rs.LockedBlock.Hash()

	//XXX: this isnt gauranteed to get there before the timeoutPropose ...
	cs1.SetProposalAndBlock(prop, propBlock, propBlockParts, "some peer")

	<-newRoundCh
	log.Notice("#### ONTO ROUND 1")
	/*
		Round2 (cs2, C) // B nil nil nil // nil nil nil _

		cs1 unlocks!
	*/

	// now we're on a new round and not the proposer,
	// but we should receive the proposal
	select {
	case <-proposalCh:
	case <-timeoutProposeCh:
		<-proposalCh
	}

	// go to prevote, prevote for locked block (not proposal)
	<-voteCh
	validatePrevote(t, cs1, 0, vss[0], lockedBlockHash)
	// now lets add prevotes from everyone else for nil (a polka!)
	signAddVoteToFromMany(types.VoteTypePrevote, cs1, nil, types.PartSetHeader{}, cs2, cs3, cs4)

	// the polka makes us unlock and precommit nil
	<-unlockCh
	<-voteCh // precommit

	// we should have unlocked and committed nil
	// NOTE: since we don't relock on nil, the lock round is 0
	validatePrecommit(t, cs1, 1, 0, vss[0], nil, nil)

	signAddVoteToFromMany(types.VoteTypePrecommit, cs1, nil, types.PartSetHeader{}, cs2, cs3)
	<-newRoundCh
}

// 4 vals
// a polka at round 1 but we miss it
// then a polka at round 2 that we lock on
// then we see the polka from round 1 but shouldn't unlock
func TestLockPOLSafety1(t *testing.T) {
	cs1, vss := randConsensusState(4)
	cs2, cs3, cs4 := vss[1], vss[2], vss[3]

	proposalCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringCompleteProposal(), 1)
	timeoutProposeCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringTimeoutPropose(), 1)
	timeoutWaitCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringTimeoutWait(), 1)
	newRoundCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringNewRound(), 1)
	voteCh := subscribeToVoter(cs1, cs1.privValidator.Address)

	// start round and wait for propose and prevote
	startTestRound(cs1, cs1.Height, 0)
	<-newRoundCh
	re := <-proposalCh
	rs := re.(types.EventDataRoundState).RoundState.(*RoundState)
	propBlock := rs.ProposalBlock

	<-voteCh // prevote

	validatePrevote(t, cs1, 0, vss[0], propBlock.Hash())

	// the others sign a polka but we don't see it
	prevotes := signVoteMany(types.VoteTypePrevote, propBlock.Hash(), propBlock.MakePartSet().Header(), cs2, cs3, cs4)

	// before we time out into new round, set next proposer
	// and next proposal block
	/*
		_, v1 := cs1.Validators.GetByAddress(vss[0].Address)
		v1.VotingPower = 1
		if updated := cs1.Validators.Update(v1); !updated {
			t.Fatal("failed to update validator")
		}*/

	log.Warn("old prop", "hash", fmt.Sprintf("%X", propBlock.Hash()))

	// we do see them precommit nil
	signAddVoteToFromMany(types.VoteTypePrecommit, cs1, nil, types.PartSetHeader{}, cs2, cs3, cs4)

	prop, propBlock := decideProposal(cs1, cs2, cs2.Height, cs2.Round+1)
	propBlockHash := propBlock.Hash()
	propBlockParts := propBlock.MakePartSet()

	incrementRound(cs2, cs3, cs4)

	//XXX: this isnt gauranteed to get there before the timeoutPropose ...
	cs1.SetProposalAndBlock(prop, propBlock, propBlockParts, "some peer")

	<-newRoundCh
	log.Notice("### ONTO ROUND 1")
	/*Round2
	// we timeout and prevote our lock
	// a polka happened but we didn't see it!
	*/

	// now we're on a new round and not the proposer,
	// but we should receive the proposal
	select {
	case re = <-proposalCh:
	case <-timeoutProposeCh:
		re = <-proposalCh
	}

	rs = re.(types.EventDataRoundState).RoundState.(*RoundState)

	if rs.LockedBlock != nil {
		t.Fatal("we should not be locked!")
	}
	log.Warn("new prop", "hash", fmt.Sprintf("%X", propBlockHash))
	// go to prevote, prevote for proposal block
	<-voteCh
	validatePrevote(t, cs1, 1, vss[0], propBlockHash)

	// now we see the others prevote for it, so we should lock on it
	signAddVoteToFromMany(types.VoteTypePrevote, cs1, propBlockHash, propBlockParts.Header(), cs2, cs3, cs4)

	<-voteCh // precommit

	// we should have precommitted
	validatePrecommit(t, cs1, 1, 1, vss[0], propBlockHash, propBlockHash)

	signAddVoteToFromMany(types.VoteTypePrecommit, cs1, nil, types.PartSetHeader{}, cs2, cs3)

	<-timeoutWaitCh

	incrementRound(cs2, cs3, cs4)

	<-newRoundCh

	log.Notice("### ONTO ROUND 2")
	/*Round3
	we see the polka from round 1 but we shouldn't unlock!
	*/

	// timeout of propose
	<-timeoutProposeCh

	// finish prevote
	<-voteCh

	// we should prevote what we're locked on
	validatePrevote(t, cs1, 2, vss[0], propBlockHash)

	newStepCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringNewRoundStep(), 1)

	// add prevotes from the earlier round
	addVoteToFromMany(cs1, prevotes, cs2, cs3, cs4)

	log.Warn("Done adding prevotes!")

	ensureNoNewStep(newStepCh)
}

// 4 vals.
// polka P0 at R0, P1 at R1, and P2 at R2,
// we lock on P0 at R0, don't see P1, and unlock using P2 at R2
// then we should make sure we don't lock using P1

// What we want:
// dont see P0, lock on P1 at R1, dont unlock using P0 at R2
func TestLockPOLSafety2(t *testing.T) {
	cs1, vss := randConsensusState(4)
	cs2, cs3, cs4 := vss[1], vss[2], vss[3]

	proposalCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringCompleteProposal(), 1)
	timeoutProposeCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringTimeoutPropose(), 1)
	timeoutWaitCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringTimeoutWait(), 1)
	newRoundCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringNewRound(), 1)
	unlockCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringUnlock(), 1)
	voteCh := subscribeToVoter(cs1, cs1.privValidator.Address)

	// the block for R0: gets polkad but we miss it
	// (even though we signed it, shhh)
	_, propBlock0 := decideProposal(cs1, vss[0], cs1.Height, cs1.Round)
	propBlockHash0 := propBlock0.Hash()
	propBlockParts0 := propBlock0.MakePartSet()

	// the others sign a polka but we don't see it
	prevotes := signVoteMany(types.VoteTypePrevote, propBlockHash0, propBlockParts0.Header(), cs2, cs3, cs4)

	// the block for round 1
	prop1, propBlock1 := decideProposal(cs1, cs2, cs2.Height, cs2.Round+1)
	propBlockHash1 := propBlock1.Hash()
	propBlockParts1 := propBlock1.MakePartSet()

	incrementRound(cs2, cs3, cs4)

	cs1.updateRoundStep(0, RoundStepPrecommitWait)

	log.Notice("### ONTO Round 1")
	// jump in at round 1
	height := cs1.Height
	startTestRound(cs1, height, 1)
	<-newRoundCh

	cs1.SetProposalAndBlock(prop1, propBlock1, propBlockParts1, "some peer")
	<-proposalCh

	<-voteCh // prevote

	signAddVoteToFromMany(types.VoteTypePrevote, cs1, propBlockHash1, propBlockParts1.Header(), cs2, cs3, cs4)

	<-voteCh // precommit
	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, 1, 1, vss[0], propBlockHash1, propBlockHash1)

	// add precommits from the rest
	signAddVoteToFromMany(types.VoteTypePrecommit, cs1, nil, types.PartSetHeader{}, cs2, cs4)
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs3, propBlockHash1, propBlockParts1.Header())

	incrementRound(cs2, cs3, cs4)

	// timeout of precommit wait to new round
	<-timeoutWaitCh

	// in round 2 we see the polkad block from round 0
	newProp := types.NewProposal(height, 2, propBlockParts0.Header(), 0)
	if err := cs3.SignProposal(chainID, newProp); err != nil {
		t.Fatal(err)
	}
	cs1.SetProposalAndBlock(newProp, propBlock0, propBlockParts0, "some peer")
	addVoteToFromMany(cs1, prevotes, cs2, cs3, cs4) // add the pol votes

	<-newRoundCh
	log.Notice("### ONTO Round 2")
	/*Round2
	// now we see the polka from round 1, but we shouldnt unlock
	*/

	select {
	case <-timeoutProposeCh:
		<-proposalCh
	case <-proposalCh:
	}

	select {
	case <-unlockCh:
		t.Fatal("validator unlocked using an old polka")
	case <-voteCh:
		// prevote our locked block
	}
	validatePrevote(t, cs1, 2, vss[0], propBlockHash1)

}

//------------------------------------------------------------------------------------------
// SlashingSuite
// TODO: Slashing

/*
func TestSlashingPrevotes(t *testing.T) {
	cs1, vss := randConsensusState(2)
	cs2 := vss[1]


	proposalCh := subscribeToEvent(cs1.evsw,"tester",types.EventStringCompleteProposal() , 1)
	timeoutWaitCh := subscribeToEvent(cs1.evsw,"tester",types.EventStringTimeoutWait() , 1)
	newRoundCh := subscribeToEvent(cs1.evsw,"tester",types.EventStringNewRound() , 1)
	voteCh := subscribeToVoter(cs1, cs1.privValidator.Address)

	// start round and wait for propose and prevote
	startTestRound(cs1, cs1.Height, 0)
	<-newRoundCh
	re := <-proposalCh
	<-voteCh // prevote

	rs := re.(types.EventDataRoundState).RoundState.(*RoundState)

	// we should now be stuck in limbo forever, waiting for more prevotes
	// add one for a different block should cause us to go into prevote wait
	hash := cs1.ProposalBlock.Hash()
	hash[0] = byte(hash[0]+1) % 255
	signAddVoteToFrom(types.VoteTypePrevote, cs1, cs2, hash, rs.ProposalBlockParts.Header())

	<-timeoutWaitCh

	// NOTE: we have to send the vote for different block first so we don't just go into precommit round right
	// away and ignore more prevotes (and thus fail to slash!)

	// add the conflicting vote
	signAddVoteToFrom(types.VoteTypePrevote, cs1, cs2, rs.ProposalBlock.Hash(), rs.ProposalBlockParts.Header())

	// XXX: Check for existence of Dupeout info
}

func TestSlashingPrecommits(t *testing.T) {
	cs1, vss := randConsensusState(2)
	cs2 := vss[1]


	proposalCh := subscribeToEvent(cs1.evsw,"tester",types.EventStringCompleteProposal() , 1)
	timeoutWaitCh := subscribeToEvent(cs1.evsw,"tester",types.EventStringTimeoutWait() , 1)
	newRoundCh := subscribeToEvent(cs1.evsw,"tester",types.EventStringNewRound() , 1)
	voteCh := subscribeToVoter(cs1, cs1.privValidator.Address)

	// start round and wait for propose and prevote
	startTestRound(cs1, cs1.Height, 0)
	<-newRoundCh
	re := <-proposalCh
	<-voteCh // prevote

	// add prevote from cs2
	signAddVoteToFrom(types.VoteTypePrevote, cs1, cs2, rs.ProposalBlock.Hash(), rs.ProposalBlockParts.Header())

	<-voteCh // precommit

	// we should now be stuck in limbo forever, waiting for more prevotes
	// add one for a different block should cause us to go into prevote wait
	hash := rs.ProposalBlock.Hash()
	hash[0] = byte(hash[0]+1) % 255
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs2, hash, rs.ProposalBlockParts.Header())

	// NOTE: we have to send the vote for different block first so we don't just go into precommit round right
	// away and ignore more prevotes (and thus fail to slash!)

	// add precommit from cs2
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs2, rs.ProposalBlock.Hash(), rs.ProposalBlockParts.Header())

	// XXX: Check for existence of Dupeout info
}
*/

//------------------------------------------------------------------------------------------
// CatchupSuite

//------------------------------------------------------------------------------------------
// HaltSuite

// 4 vals.
// we receive a final precommit after going into next round, but others might have gone to commit already!
func TestHalt1(t *testing.T) {
	cs1, vss := randConsensusState(4)
	cs2, cs3, cs4 := vss[1], vss[2], vss[3]

	proposalCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringCompleteProposal(), 1)
	timeoutWaitCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringTimeoutWait(), 1)
	newRoundCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringNewRound(), 1)
	newBlockCh := subscribeToEvent(cs1.evsw, "tester", types.EventStringNewBlock(), 1)
	voteCh := subscribeToVoter(cs1, cs1.privValidator.Address)

	// start round and wait for propose and prevote
	startTestRound(cs1, cs1.Height, 0)
	<-newRoundCh
	re := <-proposalCh
	rs := re.(types.EventDataRoundState).RoundState.(*RoundState)
	propBlock := rs.ProposalBlock
	propBlockParts := propBlock.MakePartSet()

	<-voteCh // prevote

	signAddVoteToFromMany(types.VoteTypePrevote, cs1, propBlock.Hash(), propBlockParts.Header(), cs3, cs4)
	<-voteCh // precommit

	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, 0, 0, vss[0], propBlock.Hash(), propBlock.Hash())

	// add precommits from the rest
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs2, nil, types.PartSetHeader{}) // didnt receive proposal
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs3, propBlock.Hash(), propBlockParts.Header())
	// we receive this later, but cs3 might receive it earlier and with ours will go to commit!
	precommit4 := signVote(cs4, types.VoteTypePrecommit, propBlock.Hash(), propBlockParts.Header())

	incrementRound(cs2, cs3, cs4)

	// timeout to new round
	<-timeoutWaitCh
	re = <-newRoundCh
	rs = re.(types.EventDataRoundState).RoundState.(*RoundState)

	log.Notice("### ONTO ROUND 1")
	/*Round2
	// we timeout and prevote our lock
	// a polka happened but we didn't see it!
	*/

	// go to prevote, prevote for locked block
	<-voteCh // prevote
	validatePrevote(t, cs1, 0, vss[0], rs.LockedBlock.Hash())

	// now we receive the precommit from the previous round
	addVoteToFrom(cs1, cs4, precommit4)

	// receiving that precommit should take us straight to commit
	<-newBlockCh
	re = <-newRoundCh
	rs = re.(types.EventDataRoundState).RoundState.(*RoundState)

	if rs.Height != 2 {
		t.Fatal("expected height to increment")
	}
}
