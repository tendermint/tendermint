package consensus

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	_ "github.com/tendermint/tendermint/config/tendermint_test"
	//"github.com/tendermint/tendermint/events"
	"github.com/tendermint/tendermint/events"
	"github.com/tendermint/tendermint/types"
)

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

func init() {
	fmt.Println("")
	timeoutPropose = 500 * time.Millisecond
}

func TestProposerSelection0(t *testing.T) {
	cs1, vss := simpleConsensusState(3)    // test needs more work for more than 3 validators
	cs1.newStepCh = make(chan *RoundState) // so it blocks
	height, round := cs1.Height, cs1.Round

	go cs1.EnterNewRound(height, round, false)

	// lets commit a block and ensure proposer for the next height is correct
	prop := cs1.Validators.Proposer()
	if !bytes.Equal(prop.Address, cs1.privValidator.Address) {
		t.Fatalf("expected proposer to be validator %d. Got %X", 0, prop.Address)
	}

	waitFor(t, cs1, height, round, RoundStepPrevote)

	signAddVoteToFromMany(types.VoteTypePrevote, cs1, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header(), vss[1:]...)

	waitFor(t, cs1, height, round, RoundStepPrecommit)

	signAddVoteToFromMany(types.VoteTypePrecommit, cs1, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header(), vss[1:]...)

	waitFor(t, cs1, height, round, RoundStepPrecommit)
	waitFor(t, cs1, height, round+1, RoundStepPropose)

	prop = cs1.Validators.Proposer()
	if !bytes.Equal(prop.Address, vss[1].Address) {
		t.Fatalf("expected proposer to be validator %d. Got %X", 1, prop.Address)
	}
}

// Now let's do it all again, but starting from round 2 instead of 0
func TestProposerSelection2(t *testing.T) {
	cs1, vss := simpleConsensusState(3)    // test needs more work for more than 3 validators
	cs1.newStepCh = make(chan *RoundState) // so it blocks

	// listen for new round
	ch := make(chan struct{})
	evsw := events.NewEventSwitch()
	evsw.OnStart()
	evsw.AddListenerForEvent("tester", types.EventStringNewRound(), func(data types.EventData) {
		ch <- struct{}{}
	})
	cs1.SetFireable(evsw)

	// this time we jump in at round 2
	incrementRound(vss[1:]...)
	incrementRound(vss[1:]...)
	go cs1.EnterNewRound(cs1.Height, 2, false)

	<-ch // wait for the new round

	// everyone just votes nil. we get a new proposer each round
	for i := 0; i < len(vss); i++ {
		prop := cs1.Validators.Proposer()
		if !bytes.Equal(prop.Address, vss[(i+2)%len(vss)].Address) {
			t.Fatalf("expected proposer to be validator %d. Got %X", (i+2)%len(vss), prop.Address)
		}
		go nilRound(t, cs1, vss[1:]...)
		<-ch // wait for the new round event each round

		incrementRound(vss[1:]...)
	}

}

// a non-validator should timeout into the prevote round
func TestEnterProposeNoPrivValidator(t *testing.T) {
	cs, _ := simpleConsensusState(1)
	cs.SetPrivValidator(nil)
	height, round := cs.Height, cs.Round

	// Listen for propose timeout event
	timeoutEventReceived := false
	evsw := events.NewEventSwitch()
	evsw.OnStart()
	evsw.AddListenerForEvent("tester", types.EventStringTimeoutPropose(), func(data types.EventData) {
		timeoutEventReceived = true
	})
	cs.SetFireable(evsw)

	// starts a go routine for EnterPropose
	go cs.EnterNewRound(height, round, false)

	// Wait until the prevote step
	waitFor(t, cs, height, round, RoundStepPrevote)

	// if we're not a validator, EnterPropose should timeout
	if timeoutEventReceived == false {
		t.Fatal("Expected EnterPropose to timeout")
	}
	if cs.GetRoundState().Proposal != nil {
		t.Error("Expected to make no proposal, since no privValidator")
	}
}

// a validator should not timeout of the prevote round (TODO: unless the block is really big!)
func TestEnterProposeYesPrivValidator(t *testing.T) {
	cs, _ := simpleConsensusState(1)
	height, round := cs.Height, cs.Round

	// Listen for propose timeout event
	timeoutEventReceived := false
	evsw := events.NewEventSwitch()
	evsw.OnStart()
	evsw.AddListenerForEvent("tester", types.EventStringTimeoutPropose(), func(data types.EventData) {
		timeoutEventReceived = true
	})
	cs.SetFireable(evsw)

	// starts a go routine for the round
	go cs.EnterNewRound(height, round, false)

	// Wait until the prevote step
	waitFor(t, cs, height, round, RoundStepPrevote)

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

	// if we're a validator, EnterPropose should not timeout
	if timeoutEventReceived == true {
		t.Fatal("Expected EnterPropose not to timeout")
	}
}

func TestBadProposal(t *testing.T) {
	cs1, vss := simpleConsensusState(2)
	cs1.newStepCh = make(chan *RoundState) // so it blocks
	height, round := cs1.Height, cs1.Round
	cs2 := vss[1]

	timeoutChan := make(chan struct{})
	evsw := events.NewEventSwitch()
	evsw.OnStart()
	evsw.AddListenerForEvent("tester", types.EventStringTimeoutPropose(), func(data types.EventData) {
		timeoutChan <- struct{}{}
	})
	evsw.AddListenerForEvent("tester", types.EventStringTimeoutWait(), func(data types.EventData) {
		timeoutChan <- struct{}{}
	})
	cs1.SetFireable(evsw)

	// make the second validator the proposer
	propBlock := changeProposer(t, cs1, cs2)

	// make the block bad by tampering with statehash
	stateHash := propBlock.AppHash
	if len(stateHash) == 0 {
		stateHash = make([]byte, 32)
	}
	stateHash[0] = byte((stateHash[0] + 1) % 255)
	propBlock.AppHash = stateHash
	propBlockParts := propBlock.MakePartSet()
	proposal := types.NewProposal(cs2.Height, cs2.Round, propBlockParts.Header(), -1)
	if err := cs2.SignProposal(chainID, proposal); err != nil {
		t.Fatal("failed to sign bad proposal", err)
	}

	// start round
	go cs1.EnterNewRound(height, round, false)

	// now we're on a new round and not the proposer
	waitFor(t, cs1, height, round, RoundStepPropose)
	// so set the proposal block (and fix voting power)
	cs1.mtx.Lock()
	cs1.Proposal, cs1.ProposalBlock, cs1.ProposalBlockParts = proposal, propBlock, propBlockParts
	fixVotingPower(t, cs1, vss[1].Address)
	cs1.mtx.Unlock()
	// and wait for timeout
	<-timeoutChan

	// go to prevote, prevote for nil (proposal is bad)
	waitFor(t, cs1, height, round, RoundStepPrevote)

	validatePrevote(t, cs1, round, vss[0], nil)

	// add bad prevote from cs2. we should precommit nil
	signAddVoteToFrom(types.VoteTypePrevote, cs1, cs2, propBlock.Hash(), propBlock.MakePartSet().Header())

	waitFor(t, cs1, height, round, RoundStepPrevoteWait)
	<-timeoutChan
	waitFor(t, cs1, height, round, RoundStepPrecommit)

	validatePrecommit(t, cs1, round, 0, vss[0], nil, nil)
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs2, propBlock.Hash(), propBlock.MakePartSet().Header())
}

//----------------------------------------------------------------------------------------------------
// FullRoundSuite

// propose, prevote, and precommit a block
func TestFullRound1(t *testing.T) {
	cs, vss := simpleConsensusState(1)
	height, round := cs.Height, cs.Round

	// starts a go routine for EnterPropose
	go cs.EnterNewRound(height, round, false)

	// wait to finish propose and prevote
	waitFor(t, cs, height, round, RoundStepPrevote)

	// we should now be in precommit
	// verify our prevote is there
	cs.mtx.Lock()
	propBlockHash := cs.ProposalBlock.Hash()
	cs.mtx.Unlock()

	// Wait until Precommit
	waitFor(t, cs, height, round, RoundStepPrecommit)

	// the proposed block should be prevoted, precommitted, and locked
	validatePrevoteAndPrecommit(t, cs, round, round, vss[0], propBlockHash, propBlockHash)
}

/*
// nil is proposed, so prevote and precommit nil
func TestFullRoundNil(t *testing.T) {
	cs, vss := simpleConsensusState(1)
	height, round := cs.Height, cs.Round

	// TODO: This is not easy to test now because we need receiveRoutine to start things off
	// and we want to not be the proposer but still vote ....

	// Skip the propose step
	cs.EnterPrevote(height, round, true)

	// Wait until Precommit
	waitFor(t, cs, height, round, RoundStepPrecommit)

	// should prevote and precommit nil
	validatePrevoteAndPrecommit(t, cs, round, 0, vss[0], nil, nil)
}
*/

// run through propose, prevote, precommit commit with two validators
// where the first validator has to wait for votes from the second
func TestFullRound2(t *testing.T) {
	cs1, vss := simpleConsensusState(2)
	cs2 := vss[1]
	cs1.newStepCh = make(chan *RoundState) // so it blocks
	height, round := cs1.Height, cs1.Round

	// start round and wait for propose and prevote
	go cs1.EnterNewRound(height, round, false)
	waitFor(t, cs1, height, round, RoundStepPrevote)

	// we should now be stuck in limbo forever, waiting for more prevotes
	ensureNoNewStep(t, cs1)

	propBlockHash, propPartsHeader := cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header()

	// prevote arrives from cs2:
	signAddVoteToFrom(types.VoteTypePrevote, cs1, cs2, propBlockHash, propPartsHeader)

	// wait to finish precommit
	waitFor(t, cs1, cs1.Height, 0, RoundStepPrecommit)

	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, 0, 0, vss[0], propBlockHash, propBlockHash)

	// we should now be stuck in limbo forever, waiting for more precommits
	ensureNoNewStep(t, cs1)

	// precommit arrives from cs2:
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs2, propBlockHash, propPartsHeader)

	// wait to finish commit, propose in next height
	waitFor(t, cs1, height+1, 0, RoundStepNewHeight)
}

//------------------------------------------------------------------------------------------
// LockSuite

// two validators, 4 rounds.
// val1 proposes the first 2 rounds, and is locked in the first.
// val2 proposes the next two. val1 should precommit nil on all (except first where he locks)
func TestLockNoPOL(t *testing.T) {
	cs1, vss := simpleConsensusState(2)
	cs2 := vss[1]
	cs1.newStepCh = make(chan *RoundState) // so it blocks
	height := cs1.Height

	timeoutChan := make(chan struct{})
	evsw := events.NewEventSwitch()
	evsw.OnStart()
	evsw.AddListenerForEvent("tester", types.EventStringTimeoutPropose(), func(data types.EventData) {
		timeoutChan <- struct{}{}
	})
	evsw.AddListenerForEvent("tester", types.EventStringTimeoutWait(), func(data types.EventData) {
		timeoutChan <- struct{}{}
	})
	cs1.SetFireable(evsw)

	/*
		Round1 (cs1, B) // B B // B B2
	*/

	// start round and wait for prevote
	go cs1.EnterNewRound(height, 0, false)
	waitFor(t, cs1, height, 0, RoundStepPrevote)

	// we should now be stuck in limbo forever, waiting for more prevotes
	// prevote arrives from cs2:
	signAddVoteToFrom(types.VoteTypePrevote, cs1, cs2, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header())

	cs1.mtx.Lock() // XXX: sigh
	theBlockHash := cs1.ProposalBlock.Hash()
	cs1.mtx.Unlock()

	// wait to finish precommit
	waitFor(t, cs1, height, 0, RoundStepPrecommit)

	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, 0, 0, vss[0], theBlockHash, theBlockHash)

	// we should now be stuck in limbo forever, waiting for more precommits
	// lets add one for a different block
	// NOTE: in practice we should never get to a point where there are precommits for different blocks at the same round
	hash := cs1.ProposalBlock.Hash()
	hash[0] = byte((hash[0] + 1) % 255)
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs2, hash, cs1.ProposalBlockParts.Header())

	// (note we're entering precommit for a second time this round)
	// but with invalid args. then we EnterPrecommitWait, and the timeout to new round
	waitFor(t, cs1, height, 0, RoundStepPrecommitWait)
	<-timeoutChan

	log.Info("#### ONTO ROUND 2")
	/*
		Round2 (cs1, B) // B B2
	*/

	incrementRound(cs2)

	// now we're on a new round and not the proposer, so wait for timeout
	waitFor(t, cs1, height, 1, RoundStepPropose)
	<-timeoutChan

	if cs1.ProposalBlock != nil {
		t.Fatal("Expected proposal block to be nil")
	}

	// wait to finish prevote
	waitFor(t, cs1, height, 1, RoundStepPrevote)

	// we should have prevoted our locked block
	validatePrevote(t, cs1, 1, vss[0], cs1.LockedBlock.Hash())

	// add a conflicting prevote from the other validator
	signAddVoteToFrom(types.VoteTypePrevote, cs1, cs2, hash, cs1.ProposalBlockParts.Header())

	// now we're going to enter prevote again, but with invalid args
	// and then prevote wait, which should timeout. then wait for precommit
	waitFor(t, cs1, height, 1, RoundStepPrevoteWait)
	<-timeoutChan
	waitFor(t, cs1, height, 1, RoundStepPrecommit)

	// the proposed block should still be locked and our precommit added
	// we should precommit nil and be locked on the proposal
	validatePrecommit(t, cs1, 1, 0, vss[0], nil, theBlockHash)

	// add conflicting precommit from cs2
	// NOTE: in practice we should never get to a point where there are precommits for different blocks at the same round
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs2, hash, cs1.ProposalBlockParts.Header())

	// (note we're entering precommit for a second time this round, but with invalid args
	// then we EnterPrecommitWait and timeout into NewRound
	waitFor(t, cs1, height, 1, RoundStepPrecommitWait)
	<-timeoutChan

	log.Info("#### ONTO ROUND 3")
	/*
		Round3 (cs2, _) // B, B2
	*/

	incrementRound(cs2)

	waitFor(t, cs1, height, 2, RoundStepPropose)

	// now we're on a new round and are the proposer
	if cs1.ProposalBlock != cs1.LockedBlock {
		t.Fatalf("Expected proposal block to be locked block. Got %v, Expected %v", cs1.ProposalBlock, cs1.LockedBlock)
	}

	// go to prevote, prevote for locked block
	waitFor(t, cs1, height, 2, RoundStepPrevote)

	validatePrevote(t, cs1, 0, vss[0], cs1.LockedBlock.Hash())

	// TODO: quick fastforward to new round, set proposer
	signAddVoteToFrom(types.VoteTypePrevote, cs1, cs2, hash, cs1.ProposalBlockParts.Header())

	waitFor(t, cs1, height, 2, RoundStepPrevoteWait)
	<-timeoutChan
	waitFor(t, cs1, height, 2, RoundStepPrecommit)

	validatePrecommit(t, cs1, 2, 0, vss[0], nil, theBlockHash)                                  // precommit nil but be locked on proposal
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs2, hash, cs1.ProposalBlockParts.Header()) // NOTE: conflicting precommits at same height

	waitFor(t, cs1, height, 2, RoundStepPrecommitWait)

	// before we time out into new round, set next proposal block
	prop, propBlock := decideProposal(cs1, cs2, cs2.Height, cs2.Round+1)
	if prop == nil || propBlock == nil {
		t.Fatal("Failed to create proposal block with cs2")
	}

	incrementRound(cs2)

	<-timeoutChan

	log.Info("#### ONTO ROUND 4")
	/*
		Round4 (cs2, C) // B C // B C
	*/

	// now we're on a new round and not the proposer
	// so set the proposal block
	cs1.mtx.Lock()
	cs1.Proposal, cs1.ProposalBlock = prop, propBlock
	cs1.mtx.Unlock()

	// wait for the proposal go ahead
	waitFor(t, cs1, height, 3, RoundStepPropose)

	//log.Debug("waiting for timeout")
	// and wait for timeout
	// go func() { <-timeoutChan }()

	log.Debug("waiting for prevote")
	// go to prevote, prevote for locked block (not proposal)
	waitFor(t, cs1, height, 3, RoundStepPrevote)

	validatePrevote(t, cs1, 0, vss[0], cs1.LockedBlock.Hash())

	signAddVoteToFrom(types.VoteTypePrevote, cs1, cs2, propBlock.Hash(), propBlock.MakePartSet().Header())

	waitFor(t, cs1, height, 3, RoundStepPrevoteWait)
	<-timeoutChan
	waitFor(t, cs1, height, 3, RoundStepPrecommit)

	validatePrecommit(t, cs1, 2, 0, vss[0], nil, theBlockHash)                                               // precommit nil but locked on proposal
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs2, propBlock.Hash(), propBlock.MakePartSet().Header()) // NOTE: conflicting precommits at same height
}

// 4 vals, one precommits, other 3 polka at next round, so we unlock and precomit the polka
func TestLockPOLRelock(t *testing.T) {
	cs1, vss := simpleConsensusState(4)
	cs2, cs3, cs4 := vss[1], vss[2], vss[3]
	cs1.newStepCh = make(chan *RoundState) // so it blocks

	timeoutChan := make(chan *types.EventDataRoundState)
	voteChan := make(chan *types.EventDataVote)
	evsw := events.NewEventSwitch()
	evsw.OnStart()
	evsw.AddListenerForEvent("tester", types.EventStringTimeoutPropose(), func(data types.EventData) {
		timeoutChan <- data.(*types.EventDataRoundState)
	})
	evsw.AddListenerForEvent("tester", types.EventStringTimeoutWait(), func(data types.EventData) {
		timeoutChan <- data.(*types.EventDataRoundState)
	})
	evsw.AddListenerForEvent("tester", types.EventStringVote(), func(data types.EventData) {
		vote := data.(*types.EventDataVote)
		// we only fire for our own votes
		if bytes.Equal(cs1.privValidator.Address, vote.Address) {
			voteChan <- vote
		}
	})
	cs1.SetFireable(evsw)

	// everything done from perspective of cs1

	/*
		Round1 (cs1, B) // B B B B// B nil B nil

		eg. cs2 and cs4 didn't see the 2/3 prevotes
	*/

	// start round and wait for propose and prevote
	go cs1.EnterNewRound(cs1.Height, 0, false)
	_, _, _ = <-cs1.NewStepCh(), <-voteChan, <-cs1.NewStepCh()

	theBlockHash := cs1.ProposalBlock.Hash()

	// wait to finish precommit after prevotes done
	// we do this in a go routine with another channel since otherwise
	// we may get deadlock with EnterPrecommit waiting to send on newStepCh and the final
	// signAddVoteToFrom waiting for the cs.mtx.Lock
	donePrecommit := make(chan struct{})
	go func() {
		<-voteChan
		<-cs1.NewStepCh()
		donePrecommit <- struct{}{}
	}()
	signAddVoteToFromMany(types.VoteTypePrevote, cs1, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header(), cs2, cs3, cs4)
	<-donePrecommit

	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, 0, 0, vss[0], theBlockHash, theBlockHash)

	donePrecommitWait := make(chan struct{})
	go func() {
		// (note we're entering precommit for a second time this round)
		// but with invalid args. then we EnterPrecommitWait, twice (?)
		<-cs1.NewStepCh()
		donePrecommitWait <- struct{}{}
	}()
	// add precommits from the rest
	signAddVoteToFromMany(types.VoteTypePrecommit, cs1, nil, types.PartSetHeader{}, cs2, cs4)
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs3, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header())
	<-donePrecommitWait

	// before we time out into new round, set next proposer
	// and next proposal block
	_, v1 := cs1.Validators.GetByAddress(vss[0].Address)
	v1.VotingPower = 1
	if updated := cs1.Validators.Update(v1); !updated {
		t.Fatal("failed to update validator")
	}

	prop, propBlock := decideProposal(cs1, cs2, cs2.Height, cs2.Round+1)

	incrementRound(cs2, cs3, cs4)

	// timeout to new round
	te := <-timeoutChan
	if te.Step != RoundStepPrecommitWait.String() {
		t.Fatalf("expected to timeout of precommit into new round. got %v", te.Step)
	}

	log.Info("### ONTO ROUND 2")

	/*
		Round2 (cs2, C) // B C C C // C C C _)

		cs1 changes lock!
	*/

	// now we're on a new round and not the proposer
	// so set the proposal block
	cs1.mtx.Lock()
	propBlockHash, propBlockParts := propBlock.Hash(), propBlock.MakePartSet()
	cs1.Proposal, cs1.ProposalBlock, cs1.ProposalBlockParts = prop, propBlock, propBlockParts
	cs1.mtx.Unlock()
	<-cs1.NewStepCh()

	// go to prevote, prevote for locked block (not proposal), move on
	_, _ = <-voteChan, <-cs1.NewStepCh()
	validatePrevote(t, cs1, 0, vss[0], theBlockHash)

	donePrecommit = make(chan struct{})
	go func() {
		//  we need this go routine because if we go into PrevoteWait it has to pull on newStepCh
		// before the final vote will get added (because it holds the mutex).
		select {
		case <-cs1.NewStepCh(): // we're in PrevoteWait, go to Precommit
			<-voteChan
		case <-voteChan: // we went straight to Precommit
		}
		donePrecommit <- struct{}{}
	}()
	// now lets add prevotes from everyone else for the new block
	signAddVoteToFromMany(types.VoteTypePrevote, cs1, propBlockHash, propBlockParts.Header(), cs2, cs3, cs4)
	<-donePrecommit

	// we should have unlocked and locked on the new block
	validatePrecommit(t, cs1, 1, 1, vss[0], propBlockHash, propBlockHash)

	donePrecommitWait = make(chan struct{})
	go func() {
		// (note we're entering precommit for a second time this round)
		// but with invalid args. then we EnterPrecommitWait,
		<-cs1.NewStepCh()
		donePrecommitWait <- struct{}{}
	}()
	signAddVoteToFromMany(types.VoteTypePrecommit, cs1, propBlockHash, propBlockParts.Header(), cs2, cs3)
	<-donePrecommitWait

	<-cs1.NewStepCh()
	rs := <-cs1.NewStepCh()
	if rs.Height != 2 {
		t.Fatal("Expected height to increment")
	}

	if hash, _, ok := rs.LastCommit.TwoThirdsMajority(); !ok || !bytes.Equal(hash, propBlockHash) {
		t.Fatal("Expected block to get committed")
	}
}

// 4 vals, one precommits, other 3 polka at next round, so we unlock and precomit the polka
func TestLockPOLUnlock(t *testing.T) {
	cs1, vss := simpleConsensusState(4)
	cs2, cs3, cs4 := vss[1], vss[2], vss[3]
	cs1.newStepCh = make(chan *RoundState) // so it blocks

	timeoutChan := make(chan *types.EventDataRoundState)
	voteChan := make(chan *types.EventDataVote)
	evsw := events.NewEventSwitch()
	evsw.OnStart()
	evsw.AddListenerForEvent("tester", types.EventStringTimeoutPropose(), func(data types.EventData) {
		timeoutChan <- data.(*types.EventDataRoundState)
	})
	evsw.AddListenerForEvent("tester", types.EventStringTimeoutWait(), func(data types.EventData) {
		timeoutChan <- data.(*types.EventDataRoundState)
	})
	evsw.AddListenerForEvent("tester", types.EventStringVote(), func(data types.EventData) {
		vote := data.(*types.EventDataVote)
		// we only fire for our own votes
		if bytes.Equal(cs1.privValidator.Address, vote.Address) {
			voteChan <- vote
		}
	})
	cs1.SetFireable(evsw)

	// everything done from perspective of cs1

	/*
		Round1 (cs1, B) // B B B B // B nil B nil

		eg. didn't see the 2/3 prevotes
	*/

	// start round and wait for propose and prevote
	go cs1.EnterNewRound(cs1.Height, 0, false)
	_, _, _ = <-cs1.NewStepCh(), <-voteChan, <-cs1.NewStepCh()

	theBlockHash := cs1.ProposalBlock.Hash()

	donePrecommit := make(chan struct{})
	go func() {
		<-voteChan
		<-cs1.NewStepCh()
		donePrecommit <- struct{}{}
	}()
	signAddVoteToFromMany(types.VoteTypePrevote, cs1, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header(), cs2, cs3, cs4)
	<-donePrecommit

	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, 0, 0, vss[0], theBlockHash, theBlockHash)

	donePrecommitWait := make(chan struct{})
	go func() {
		<-cs1.NewStepCh()
		donePrecommitWait <- struct{}{}
	}()
	// add precommits from the rest
	signAddVoteToFromMany(types.VoteTypePrecommit, cs1, nil, types.PartSetHeader{}, cs2, cs4)
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs3, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header())
	<-donePrecommitWait

	// before we time out into new round, set next proposer
	// and next proposal block
	_, v1 := cs1.Validators.GetByAddress(vss[0].Address)
	v1.VotingPower = 1
	if updated := cs1.Validators.Update(v1); !updated {
		t.Fatal("failed to update validator")
	}

	prop, propBlock := decideProposal(cs1, cs2, cs2.Height, cs2.Round+1)

	incrementRound(cs2, cs3, cs4)

	// timeout to new round
	<-timeoutChan

	log.Info("#### ONTO ROUND 2")
	/*
		Round2 (cs2, C) // B nil nil nil // nil nil nil _

		cs1 unlocks!
	*/

	// now we're on a new round and not the proposer,
	// so set the proposal block
	cs1.mtx.Lock()
	cs1.Proposal, cs1.ProposalBlock, cs1.ProposalBlockParts = prop, propBlock, propBlock.MakePartSet()
	lockedBlockHash := cs1.LockedBlock.Hash()
	cs1.mtx.Unlock()
	<-cs1.NewStepCh()

	// go to prevote, prevote for locked block (not proposal)
	_, _ = <-voteChan, <-cs1.NewStepCh()
	validatePrevote(t, cs1, 0, vss[0], lockedBlockHash)

	donePrecommit = make(chan struct{})
	go func() {
		select {
		case <-cs1.NewStepCh(): // we're in PrevoteWait, go to Precommit
			<-voteChan
		case <-voteChan: // we went straight to Precommit
		}
		donePrecommit <- struct{}{}
	}()
	// now lets add prevotes from everyone else for the new block
	signAddVoteToFromMany(types.VoteTypePrevote, cs1, nil, types.PartSetHeader{}, cs2, cs3, cs4)
	<-donePrecommit

	// we should have unlocked
	// NOTE: we don't lock on nil, so LockedRound is still 0
	validatePrecommit(t, cs1, 1, 0, vss[0], nil, nil)

	donePrecommitWait = make(chan struct{})
	go func() {
		// the votes will bring us to new round right away
		// we should timeout of it
		_, _, _ = <-cs1.NewStepCh(), <-cs1.NewStepCh(), <-timeoutChan
		donePrecommitWait <- struct{}{}
	}()
	signAddVoteToFromMany(types.VoteTypePrecommit, cs1, nil, types.PartSetHeader{}, cs2, cs3)
	<-donePrecommitWait
}

// 4 vals
// a polka at round 1 but we miss it
// then a polka at round 2 that we lock on
// then we see the polka from round 1 but shouldn't unlock
func TestLockPOLSafety1(t *testing.T) {
	cs1, vss := simpleConsensusState(4)
	cs2, cs3, cs4 := vss[1], vss[2], vss[3]
	cs1.newStepCh = make(chan *RoundState) // so it blocks

	timeoutChan := make(chan *types.EventDataRoundState)
	voteChan := make(chan *types.EventDataVote)
	evsw := events.NewEventSwitch()
	evsw.OnStart()
	evsw.AddListenerForEvent("tester", types.EventStringTimeoutPropose(), func(data types.EventData) {
		timeoutChan <- data.(*types.EventDataRoundState)
	})
	evsw.AddListenerForEvent("tester", types.EventStringTimeoutWait(), func(data types.EventData) {
		timeoutChan <- data.(*types.EventDataRoundState)
	})
	evsw.AddListenerForEvent("tester", types.EventStringVote(), func(data types.EventData) {
		vote := data.(*types.EventDataVote)
		// we only fire for our own votes
		if bytes.Equal(cs1.privValidator.Address, vote.Address) {
			voteChan <- vote
		}
	})
	cs1.SetFireable(evsw)

	// start round and wait for propose and prevote
	go cs1.EnterNewRound(cs1.Height, 0, false)
	_, _, _ = <-cs1.NewStepCh(), <-voteChan, <-cs1.NewStepCh()

	propBlock := cs1.ProposalBlock

	validatePrevote(t, cs1, 0, vss[0], cs1.ProposalBlock.Hash())

	// the others sign a polka but we don't see it
	prevotes := signVoteMany(types.VoteTypePrevote, propBlock.Hash(), propBlock.MakePartSet().Header(), cs2, cs3, cs4)

	// before we time out into new round, set next proposer
	// and next proposal block
	_, v1 := cs1.Validators.GetByAddress(vss[0].Address)
	v1.VotingPower = 1
	if updated := cs1.Validators.Update(v1); !updated {
		t.Fatal("failed to update validator")
	}

	log.Warn("old prop", "hash", fmt.Sprintf("%X", propBlock.Hash()))

	// we do see them precommit nil
	signAddVoteToFromMany(types.VoteTypePrecommit, cs1, nil, types.PartSetHeader{}, cs2, cs3, cs4)

	prop, propBlock := decideProposal(cs1, cs2, cs2.Height, cs2.Round+1)

	incrementRound(cs2, cs3, cs4)

	log.Info("### ONTO ROUND 2")
	/*Round2
	// we timeout and prevote our lock
	// a polka happened but we didn't see it!
	*/

	// now we're on a new round and not the proposer,
	// so set proposal
	cs1.mtx.Lock()
	propBlockHash, propBlockParts := propBlock.Hash(), propBlock.MakePartSet()
	cs1.Proposal, cs1.ProposalBlock, cs1.ProposalBlockParts = prop, propBlock, propBlockParts
	cs1.mtx.Unlock()
	<-cs1.NewStepCh()

	if cs1.LockedBlock != nil {
		t.Fatal("we should not be locked!")
	}
	log.Warn("new prop", "hash", fmt.Sprintf("%X", propBlockHash))
	// go to prevote, prevote for proposal block
	_, _ = <-voteChan, <-cs1.NewStepCh()
	validatePrevote(t, cs1, 1, vss[0], propBlockHash)

	// now we see the others prevote for it, so we should lock on it
	donePrecommit := make(chan struct{})
	go func() {
		select {
		case <-cs1.NewStepCh(): // we're in PrevoteWait, go to Precommit
			<-voteChan
		case <-voteChan: // we went straight to Precommit
		}
		<-cs1.NewStepCh()
		donePrecommit <- struct{}{}
	}()
	// now lets add prevotes from everyone else for nil
	signAddVoteToFromMany(types.VoteTypePrevote, cs1, propBlockHash, propBlockParts.Header(), cs2, cs3, cs4)
	<-donePrecommit

	// we should have precommitted
	validatePrecommit(t, cs1, 1, 1, vss[0], propBlockHash, propBlockHash)

	// now we see precommits for nil
	donePrecommitWait := make(chan struct{})
	go func() {
		// the votes will bring us to new round
		// we should timeut of it and go to prevote
		<-cs1.NewStepCh()
		<-timeoutChan
		donePrecommitWait <- struct{}{}
	}()
	signAddVoteToFromMany(types.VoteTypePrecommit, cs1, nil, types.PartSetHeader{}, cs2, cs3)
	<-donePrecommitWait

	incrementRound(cs2, cs3, cs4)

	log.Info("### ONTO ROUND 3")
	/*Round3
	we see the polka from round 1 but we shouldn't unlock!
	*/

	// timeout of propose
	_, _ = <-cs1.NewStepCh(), <-timeoutChan

	// finish prevote
	_, _ = <-voteChan, <-cs1.NewStepCh()

	// we should prevote what we're locked on
	validatePrevote(t, cs1, 2, vss[0], propBlockHash)

	// add prevotes from the earlier round
	addVoteToFromMany(cs1, prevotes, cs2, cs3, cs4)

	log.Warn("Done adding prevotes!")

	ensureNoNewStep(t, cs1)
}

// 4 vals.
// polka P1 at R1, P2 at R2, and P3 at R3,
// we lock on P1 at R1, don't see P2, and unlock using P3 at R3
// then we should make sure we don't lock using P2
func TestLockPOLSafety2(t *testing.T) {
	cs1, vss := simpleConsensusState(4)
	cs2, cs3, cs4 := vss[1], vss[2], vss[3]
	cs1.newStepCh = make(chan *RoundState) // so it blocks

	timeoutChan := make(chan *types.EventDataRoundState)
	voteChan := make(chan *types.EventDataVote)
	evsw := events.NewEventSwitch()
	evsw.OnStart()
	evsw.AddListenerForEvent("tester", types.EventStringTimeoutPropose(), func(data types.EventData) {
		timeoutChan <- data.(*types.EventDataRoundState)
	})
	evsw.AddListenerForEvent("tester", types.EventStringTimeoutWait(), func(data types.EventData) {
		timeoutChan <- data.(*types.EventDataRoundState)
	})
	evsw.AddListenerForEvent("tester", types.EventStringVote(), func(data types.EventData) {
		vote := data.(*types.EventDataVote)
		// we only fire for our own votes
		if bytes.Equal(cs1.privValidator.Address, vote.Address) {
			voteChan <- vote
		}
	})
	cs1.SetFireable(evsw)

	// start round and wait for propose and prevote
	go cs1.EnterNewRound(cs1.Height, 0, false)
	_, _, _ = <-cs1.NewStepCh(), <-voteChan, <-cs1.NewStepCh()

	theBlockHash := cs1.ProposalBlock.Hash()

	donePrecommit := make(chan struct{})
	go func() {
		<-voteChan
		<-cs1.NewStepCh()
		donePrecommit <- struct{}{}
	}()
	signAddVoteToFromMany(types.VoteTypePrevote, cs1, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header(), cs2, cs3, cs4)
	<-donePrecommit

	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, 0, 0, vss[0], theBlockHash, theBlockHash)

	donePrecommitWait := make(chan struct{})
	go func() {
		<-cs1.NewStepCh()
		donePrecommitWait <- struct{}{}
	}()
	// add precommits from the rest
	signAddVoteToFromMany(types.VoteTypePrecommit, cs1, nil, types.PartSetHeader{}, cs2, cs4)
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs3, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header())
	<-donePrecommitWait

	// before we time out into new round, set next proposer
	// and next proposal block
	_, v1 := cs1.Validators.GetByAddress(vss[0].Address)
	v1.VotingPower = 1
	if updated := cs1.Validators.Update(v1); !updated {
		t.Fatal("failed to update validator")
	}

	prop, propBlock := decideProposal(cs1, cs2, cs2.Height, cs2.Round+1)

	incrementRound(cs2, cs3, cs4)

	// timeout to new round
	<-timeoutChan

	log.Info("### ONTO Round 2")
	/*Round2
	// we timeout and prevote our lock
	// a polka happened but we didn't see it!
	*/

	// now we're on a new round and not the proposer, so wait for timeout
	_, _ = <-cs1.NewStepCh(), <-timeoutChan
	// go to prevote, prevote for locked block
	_, _ = <-voteChan, <-cs1.NewStepCh()
	validatePrevote(t, cs1, 0, vss[0], cs1.LockedBlock.Hash())

	// the others sign a polka but we don't see it
	prevotes := signVoteMany(types.VoteTypePrevote, propBlock.Hash(), propBlock.MakePartSet().Header(), cs2, cs3, cs4)

	// once we see prevotes for the next round we'll skip ahead

	incrementRound(cs2, cs3, cs4)

	log.Info("### ONTO Round 3")
	/*Round3
	a polka for nil causes us to unlock
	*/

	// these prevotes will send us straight to precommit at the higher round
	donePrecommit = make(chan struct{})
	go func() {
		select {
		case <-cs1.NewStepCh(): // we're in PrevoteWait, go to Precommit
			<-voteChan
		case <-voteChan: // we went straight to Precommit
		}
		<-cs1.NewStepCh()
		donePrecommit <- struct{}{}
	}()
	// now lets add prevotes from everyone else for nil
	signAddVoteToFromMany(types.VoteTypePrevote, cs1, nil, types.PartSetHeader{}, cs2, cs3, cs4)
	<-donePrecommit

	// we should have unlocked
	// NOTE: we don't lock on nil, so LockedRound is still 0
	validatePrecommit(t, cs1, 2, 0, vss[0], nil, nil)

	donePrecommitWait = make(chan struct{})
	go func() {
		// the votes will bring us to new round right away
		// we should timeut of it and go to prevote
		<-cs1.NewStepCh()
		// set the proposal block to be that which got a polka in R2
		cs1.mtx.Lock()
		cs1.Proposal, cs1.ProposalBlock, cs1.ProposalBlockParts = prop, propBlock, propBlock.MakePartSet()
		cs1.mtx.Unlock()
		// timeout into prevote, finish prevote
		_, _, _ = <-timeoutChan, <-voteChan, <-cs1.NewStepCh()
		donePrecommitWait <- struct{}{}
	}()
	signAddVoteToFromMany(types.VoteTypePrecommit, cs1, nil, types.PartSetHeader{}, cs2, cs3)
	<-donePrecommitWait

	log.Info("### ONTO ROUND 4")
	/*Round4
	we see the polka from R2
	make sure we don't lock because of it!
	*/
	// new round and not proposer
	// (we already timed out and stepped into prevote)

	log.Warn("adding prevotes from round 2")

	addVoteToFromMany(cs1, prevotes, cs2, cs3, cs4)

	log.Warn("Done adding prevotes!")

	// we should prevote it now
	validatePrevote(t, cs1, 3, vss[0], cs1.ProposalBlock.Hash())

	// but we shouldn't precommit it
	precommits := cs1.Votes.Precommits(3)
	vote := precommits.GetByIndex(0)
	if vote != nil {
		t.Fatal("validator precommitted at round 4 based on an old polka")
	}
}

//------------------------------------------------------------------------------------------
// SlashingSuite

func TestSlashingPrevotes(t *testing.T) {
	cs1, vss := simpleConsensusState(2)
	cs2 := vss[1]
	cs1.newStepCh = make(chan *RoundState) // so it blocks

	// start round and wait for propose and prevote
	go cs1.EnterNewRound(cs1.Height, 0, false)
	_, _ = <-cs1.NewStepCh(), <-cs1.NewStepCh()

	// we should now be stuck in limbo forever, waiting for more prevotes
	// add one for a different block should cause us to go into prevote wait
	hash := cs1.ProposalBlock.Hash()
	hash[0] = byte(hash[0]+1) % 255
	signAddVoteToFrom(types.VoteTypePrevote, cs1, cs2, hash, cs1.ProposalBlockParts.Header())

	// pass prevote wait
	<-cs1.NewStepCh()

	// NOTE: we have to send the vote for different block first so we don't just go into precommit round right
	// away and ignore more prevotes (and thus fail to slash!)

	// add the conflicting vote
	signAddVoteToFrom(types.VoteTypePrevote, cs1, cs2, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header())

	// XXX: Check for existence of Dupeout info
}

func TestSlashingPrecommits(t *testing.T) {
	cs1, vss := simpleConsensusState(2)
	cs2 := vss[1]
	cs1.newStepCh = make(chan *RoundState) // so it blocks

	// start round and wait for propose and prevote
	go cs1.EnterNewRound(cs1.Height, 0, false)
	_, _ = <-cs1.NewStepCh(), <-cs1.NewStepCh()

	// add prevote from cs2
	signAddVoteToFrom(types.VoteTypePrevote, cs1, cs2, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header())

	// wait to finish precommit
	<-cs1.NewStepCh()

	// we should now be stuck in limbo forever, waiting for more prevotes
	// add one for a different block should cause us to go into prevote wait
	hash := cs1.ProposalBlock.Hash()
	hash[0] = byte(hash[0]+1) % 255
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs2, hash, cs1.ProposalBlockParts.Header())

	// pass prevote wait
	<-cs1.NewStepCh()

	// NOTE: we have to send the vote for different block first so we don't just go into precommit round right
	// away and ignore more prevotes (and thus fail to slash!)

	// add precommit from cs2
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs2, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header())

	// XXX: Check for existence of Dupeout info
}

//------------------------------------------------------------------------------------------
// CatchupSuite

//------------------------------------------------------------------------------------------
// HaltSuite

// 4 vals.
// we receive a final precommit after going into next round, but others might have gone to commit already!
func TestHalt1(t *testing.T) {
	cs1, vss := simpleConsensusState(4)
	cs2, cs3, cs4 := vss[1], vss[2], vss[3]
	cs1.newStepCh = make(chan *RoundState) // so it blocks

	timeoutChan := make(chan struct{})
	evsw := events.NewEventSwitch()
	evsw.OnStart()
	evsw.AddListenerForEvent("tester", types.EventStringTimeoutWait(), func(data types.EventData) {
		timeoutChan <- struct{}{}
	})
	cs1.SetFireable(evsw)

	// start round and wait for propose and prevote
	go cs1.EnterNewRound(cs1.Height, 0, false)
	_, _ = <-cs1.NewStepCh(), <-cs1.NewStepCh()

	theBlockHash := cs1.ProposalBlock.Hash()

	donePrecommit := make(chan struct{})
	go func() {
		<-cs1.NewStepCh()
		donePrecommit <- struct{}{}
	}()
	signAddVoteToFromMany(types.VoteTypePrevote, cs1, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header(), cs3, cs4)
	<-donePrecommit

	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, 0, 0, vss[0], theBlockHash, theBlockHash)

	donePrecommitWait := make(chan struct{})
	go func() {
		<-cs1.NewStepCh()
		donePrecommitWait <- struct{}{}
	}()
	// add precommits from the rest
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs2, nil, types.PartSetHeader{}) // didnt receive proposal
	signAddVoteToFrom(types.VoteTypePrecommit, cs1, cs3, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header())
	// we receive this later, but cs3 might receive it earlier and with ours will go to commit!
	precommit4 := signVote(cs4, types.VoteTypePrecommit, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header())
	<-donePrecommitWait

	incrementRound(cs2, cs3, cs4)

	// timeout to new round
	<-timeoutChan

	log.Info("### ONTO ROUND 2")
	/*Round2
	// we timeout and prevote our lock
	// a polka happened but we didn't see it!
	*/

	// go to prevote, prevote for locked block
	_, _ = <-cs1.NewStepCh(), <-cs1.NewStepCh()
	validatePrevote(t, cs1, 0, vss[0], cs1.LockedBlock.Hash())

	// now we receive the precommit from the previous round
	addVoteToFrom(cs1, cs4, precommit4)

	// receiving that precommit should take us straight to commit
	ensureNewStep(t, cs1)
	log.Warn("done enter commit!")

	// update to state
	ensureNewStep(t, cs1)

	if cs1.Height != 2 {
		t.Fatal("expected height to increment")
	}
}
