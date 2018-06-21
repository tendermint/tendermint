package consensus

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	cstypes "github.com/tendermint/tendermint/consensus/types"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"
)

func init() {
	config = ResetConfig("consensus_state_test")
}

func ensureProposeTimeout(timeoutPropose int) time.Duration {
	return time.Duration(timeoutPropose*2) * time.Millisecond
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
x * TestFullRound2 - 2 vals, both required for full round
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

func TestStateProposerSelection0(t *testing.T) {
	cs1, vss := randConsensusState(4)
	height, round := cs1.Height, cs1.Round

	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)

	startTestRound(cs1, height, round)

	// wait for new round so proposer is set
	<-newRoundCh

	// lets commit a block and ensure proposer for the next height is correct
	prop := cs1.GetRoundState().Validators.GetProposer()
	if !bytes.Equal(prop.Address, cs1.privValidator.GetAddress()) {
		t.Fatalf("expected proposer to be validator %d. Got %X", 0, prop.Address)
	}

	// wait for complete proposal
	<-proposalCh

	rs := cs1.GetRoundState()
	signAddVotes(cs1, types.VoteTypePrecommit, rs.ProposalBlock.Hash(), rs.ProposalBlockParts.Header(), vss[1:]...)

	// wait for new round so next validator is set
	<-newRoundCh

	prop = cs1.GetRoundState().Validators.GetProposer()
	if !bytes.Equal(prop.Address, vss[1].GetAddress()) {
		panic(cmn.Fmt("expected proposer to be validator %d. Got %X", 1, prop.Address))
	}
}

// Now let's do it all again, but starting from round 2 instead of 0
func TestStateProposerSelection2(t *testing.T) {
	cs1, vss := randConsensusState(4) // test needs more work for more than 3 validators

	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)

	// this time we jump in at round 2
	incrementRound(vss[1:]...)
	incrementRound(vss[1:]...)
	startTestRound(cs1, cs1.Height, 2)

	<-newRoundCh // wait for the new round

	// everyone just votes nil. we get a new proposer each round
	for i := 0; i < len(vss); i++ {
		prop := cs1.GetRoundState().Validators.GetProposer()
		if !bytes.Equal(prop.Address, vss[(i+2)%len(vss)].GetAddress()) {
			panic(cmn.Fmt("expected proposer to be validator %d. Got %X", (i+2)%len(vss), prop.Address))
		}

		rs := cs1.GetRoundState()
		signAddVotes(cs1, types.VoteTypePrecommit, nil, rs.ProposalBlockParts.Header(), vss[1:]...)
		<-newRoundCh // wait for the new round event each round

		incrementRound(vss[1:]...)
	}

}

// a non-validator should timeout into the prevote round
func TestStateEnterProposeNoPrivValidator(t *testing.T) {
	cs, _ := randConsensusState(1)
	cs.SetPrivValidator(nil)
	height, round := cs.Height, cs.Round

	// Listen for propose timeout event
	timeoutCh := subscribe(cs.eventBus, types.EventQueryTimeoutPropose)

	startTestRound(cs, height, round)

	// if we're not a validator, EnterPropose should timeout
	ticker := time.NewTicker(ensureProposeTimeout(cs.config.TimeoutPropose))
	select {
	case <-timeoutCh:
	case <-ticker.C:
		panic("Expected EnterPropose to timeout")

	}

	if cs.GetRoundState().Proposal != nil {
		t.Error("Expected to make no proposal, since no privValidator")
	}
}

// a validator should not timeout of the prevote round (TODO: unless the block is really big!)
func TestStateEnterProposeYesPrivValidator(t *testing.T) {
	cs, _ := randConsensusState(1)
	height, round := cs.Height, cs.Round

	// Listen for propose timeout event

	timeoutCh := subscribe(cs.eventBus, types.EventQueryTimeoutPropose)
	proposalCh := subscribe(cs.eventBus, types.EventQueryCompleteProposal)

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
	ticker := time.NewTicker(ensureProposeTimeout(cs.config.TimeoutPropose))
	select {
	case <-timeoutCh:
		panic("Expected EnterPropose not to timeout")
	case <-ticker.C:

	}
}

func TestStateBadProposal(t *testing.T) {
	cs1, vss := randConsensusState(2)
	height, round := cs1.Height, cs1.Round
	vs2 := vss[1]

	partSize := cs1.state.ConsensusParams.BlockPartSizeBytes

	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	voteCh := subscribe(cs1.eventBus, types.EventQueryVote)

	propBlock, _ := cs1.createProposalBlock() //changeProposer(t, cs1, vs2)

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
	propBlockParts := propBlock.MakePartSet(partSize)
	proposal := types.NewProposal(vs2.Height, round, propBlockParts.Header(), -1, types.BlockID{})
	if err := vs2.SignProposal(config.ChainID(), proposal); err != nil {
		t.Fatal("failed to sign bad proposal", err)
	}

	// set the proposal block
	if err := cs1.SetProposalAndBlock(proposal, propBlock, propBlockParts, "some peer"); err != nil {
		t.Fatal(err)
	}

	// start the machine
	startTestRound(cs1, height, round)

	// wait for proposal
	<-proposalCh

	// wait for prevote
	<-voteCh

	validatePrevote(t, cs1, round, vss[0], nil)

	// add bad prevote from vs2 and wait for it
	signAddVotes(cs1, types.VoteTypePrevote, propBlock.Hash(), propBlock.MakePartSet(partSize).Header(), vs2)
	<-voteCh

	// wait for precommit
	<-voteCh

	validatePrecommit(t, cs1, round, 0, vss[0], nil, nil)
	signAddVotes(cs1, types.VoteTypePrecommit, propBlock.Hash(), propBlock.MakePartSet(partSize).Header(), vs2)
}

//----------------------------------------------------------------------------------------------------
// FullRoundSuite

// propose, prevote, and precommit a block
func TestStateFullRound1(t *testing.T) {
	cs, vss := randConsensusState(1)
	height, round := cs.Height, cs.Round

	// NOTE: buffer capacity of 0 ensures we can validate prevote and last commit
	// before consensus can move to the next height (and cause a race condition)
	cs.eventBus.Stop()
	eventBus := types.NewEventBusWithBufferCapacity(0)
	eventBus.SetLogger(log.TestingLogger().With("module", "events"))
	cs.SetEventBus(eventBus)
	eventBus.Start()

	voteCh := subscribe(cs.eventBus, types.EventQueryVote)
	propCh := subscribe(cs.eventBus, types.EventQueryCompleteProposal)
	newRoundCh := subscribe(cs.eventBus, types.EventQueryNewRound)

	startTestRound(cs, height, round)

	<-newRoundCh

	// grab proposal
	re := <-propCh
	propBlockHash := re.(types.EventDataRoundState).RoundState.(*cstypes.RoundState).ProposalBlock.Hash()

	<-voteCh // wait for prevote
	validatePrevote(t, cs, round, vss[0], propBlockHash)

	<-voteCh // wait for precommit

	// we're going to roll right into new height
	<-newRoundCh

	validateLastPrecommit(t, cs, vss[0], propBlockHash)
}

// nil is proposed, so prevote and precommit nil
func TestStateFullRoundNil(t *testing.T) {
	cs, vss := randConsensusState(1)
	height, round := cs.Height, cs.Round

	voteCh := subscribe(cs.eventBus, types.EventQueryVote)

	cs.enterPrevote(height, round)
	cs.startRoutines(4)

	<-voteCh // prevote
	<-voteCh // precommit

	// should prevote and precommit nil
	validatePrevoteAndPrecommit(t, cs, round, 0, vss[0], nil, nil)
}

// run through propose, prevote, precommit commit with two validators
// where the first validator has to wait for votes from the second
func TestStateFullRound2(t *testing.T) {
	cs1, vss := randConsensusState(2)
	vs2 := vss[1]
	height, round := cs1.Height, cs1.Round

	voteCh := subscribe(cs1.eventBus, types.EventQueryVote)
	newBlockCh := subscribe(cs1.eventBus, types.EventQueryNewBlock)

	// start round and wait for propose and prevote
	startTestRound(cs1, height, round)

	<-voteCh // prevote

	// we should be stuck in limbo waiting for more prevotes
	rs := cs1.GetRoundState()
	propBlockHash, propPartsHeader := rs.ProposalBlock.Hash(), rs.ProposalBlockParts.Header()

	// prevote arrives from vs2:
	signAddVotes(cs1, types.VoteTypePrevote, propBlockHash, propPartsHeader, vs2)
	<-voteCh

	<-voteCh //precommit

	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, 0, 0, vss[0], propBlockHash, propBlockHash)

	// we should be stuck in limbo waiting for more precommits

	// precommit arrives from vs2:
	signAddVotes(cs1, types.VoteTypePrecommit, propBlockHash, propPartsHeader, vs2)
	<-voteCh

	// wait to finish commit, propose in next height
	<-newBlockCh
}

//------------------------------------------------------------------------------------------
// LockSuite

// two validators, 4 rounds.
// two vals take turns proposing. val1 locks on first one, precommits nil on everything else
func TestStateLockNoPOL(t *testing.T) {
	cs1, vss := randConsensusState(2)
	vs2 := vss[1]
	height := cs1.Height

	partSize := cs1.state.ConsensusParams.BlockPartSizeBytes

	timeoutProposeCh := subscribe(cs1.eventBus, types.EventQueryTimeoutPropose)
	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	voteCh := subscribe(cs1.eventBus, types.EventQueryVote)
	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)

	/*
		Round1 (cs1, B) // B B // B B2
	*/

	// start round and wait for prevote
	cs1.enterNewRound(height, 0)
	cs1.startRoutines(0)

	re := <-proposalCh
	rs := re.(types.EventDataRoundState).RoundState.(*cstypes.RoundState)
	theBlockHash := rs.ProposalBlock.Hash()

	<-voteCh // prevote

	// we should now be stuck in limbo forever, waiting for more prevotes
	// prevote arrives from vs2:
	signAddVotes(cs1, types.VoteTypePrevote, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header(), vs2)
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
	signAddVotes(cs1, types.VoteTypePrecommit, hash, rs.ProposalBlock.MakePartSet(partSize).Header(), vs2)
	<-voteCh // precommit

	// (note we're entering precommit for a second time this round)
	// but with invalid args. then we enterPrecommitWait, and the timeout to new round
	<-timeoutWaitCh

	///

	<-newRoundCh
	t.Log("#### ONTO ROUND 1")
	/*
		Round2 (cs1, B) // B B2
	*/

	incrementRound(vs2)

	// now we're on a new round and not the proposer, so wait for timeout
	re = <-timeoutProposeCh
	rs = re.(types.EventDataRoundState).RoundState.(*cstypes.RoundState)

	if rs.ProposalBlock != nil {
		panic("Expected proposal block to be nil")
	}

	// wait to finish prevote
	<-voteCh

	// we should have prevoted our locked block
	validatePrevote(t, cs1, 1, vss[0], rs.LockedBlock.Hash())

	// add a conflicting prevote from the other validator
	signAddVotes(cs1, types.VoteTypePrevote, hash, rs.LockedBlock.MakePartSet(partSize).Header(), vs2)
	<-voteCh

	// now we're going to enter prevote again, but with invalid args
	// and then prevote wait, which should timeout. then wait for precommit
	<-timeoutWaitCh

	<-voteCh // precommit

	// the proposed block should still be locked and our precommit added
	// we should precommit nil and be locked on the proposal
	validatePrecommit(t, cs1, 1, 0, vss[0], nil, theBlockHash)

	// add conflicting precommit from vs2
	// NOTE: in practice we should never get to a point where there are precommits for different blocks at the same round
	signAddVotes(cs1, types.VoteTypePrecommit, hash, rs.LockedBlock.MakePartSet(partSize).Header(), vs2)
	<-voteCh

	// (note we're entering precommit for a second time this round, but with invalid args
	// then we enterPrecommitWait and timeout into NewRound
	<-timeoutWaitCh

	<-newRoundCh
	t.Log("#### ONTO ROUND 2")
	/*
		Round3 (vs2, _) // B, B2
	*/

	incrementRound(vs2)

	re = <-proposalCh
	rs = re.(types.EventDataRoundState).RoundState.(*cstypes.RoundState)

	// now we're on a new round and are the proposer
	if !bytes.Equal(rs.ProposalBlock.Hash(), rs.LockedBlock.Hash()) {
		panic(cmn.Fmt("Expected proposal block to be locked block. Got %v, Expected %v", rs.ProposalBlock, rs.LockedBlock))
	}

	<-voteCh // prevote

	validatePrevote(t, cs1, 2, vss[0], rs.LockedBlock.Hash())

	signAddVotes(cs1, types.VoteTypePrevote, hash, rs.ProposalBlock.MakePartSet(partSize).Header(), vs2)
	<-voteCh

	<-timeoutWaitCh // prevote wait
	<-voteCh        // precommit

	validatePrecommit(t, cs1, 2, 0, vss[0], nil, theBlockHash) // precommit nil but be locked on proposal

	signAddVotes(cs1, types.VoteTypePrecommit, hash, rs.ProposalBlock.MakePartSet(partSize).Header(), vs2) // NOTE: conflicting precommits at same height
	<-voteCh

	<-timeoutWaitCh

	// before we time out into new round, set next proposal block
	prop, propBlock := decideProposal(cs1, vs2, vs2.Height, vs2.Round+1)
	if prop == nil || propBlock == nil {
		t.Fatal("Failed to create proposal block with vs2")
	}

	incrementRound(vs2)

	<-newRoundCh
	t.Log("#### ONTO ROUND 3")
	/*
		Round4 (vs2, C) // B C // B C
	*/

	// now we're on a new round and not the proposer
	// so set the proposal block
	if err := cs1.SetProposalAndBlock(prop, propBlock, propBlock.MakePartSet(partSize), ""); err != nil {
		t.Fatal(err)
	}

	<-proposalCh
	<-voteCh // prevote

	// prevote for locked block (not proposal)
	validatePrevote(t, cs1, 0, vss[0], cs1.LockedBlock.Hash())

	signAddVotes(cs1, types.VoteTypePrevote, propBlock.Hash(), propBlock.MakePartSet(partSize).Header(), vs2)
	<-voteCh

	<-timeoutWaitCh
	<-voteCh

	validatePrecommit(t, cs1, 2, 0, vss[0], nil, theBlockHash) // precommit nil but locked on proposal

	signAddVotes(cs1, types.VoteTypePrecommit, propBlock.Hash(), propBlock.MakePartSet(partSize).Header(), vs2) // NOTE: conflicting precommits at same height
	<-voteCh
}

// 4 vals, one precommits, other 3 polka at next round, so we unlock and precomit the polka
func TestStateLockPOLRelock(t *testing.T) {
	cs1, vss := randConsensusState(4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]

	partSize := cs1.state.ConsensusParams.BlockPartSizeBytes

	timeoutProposeCh := subscribe(cs1.eventBus, types.EventQueryTimeoutPropose)
	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	voteCh := subscribe(cs1.eventBus, types.EventQueryVote)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	newBlockCh := subscribe(cs1.eventBus, types.EventQueryNewBlockHeader)

	// everything done from perspective of cs1

	/*
		Round1 (cs1, B) // B B B B// B nil B nil

		eg. vs2 and vs4 didn't see the 2/3 prevotes
	*/

	// start round and wait for propose and prevote
	startTestRound(cs1, cs1.Height, 0)

	<-newRoundCh
	re := <-proposalCh
	rs := re.(types.EventDataRoundState).RoundState.(*cstypes.RoundState)
	theBlockHash := rs.ProposalBlock.Hash()

	<-voteCh // prevote

	signAddVotes(cs1, types.VoteTypePrevote, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header(), vs2, vs3, vs4)
	// prevotes
	discardFromChan(voteCh, 3)

	<-voteCh // our precommit
	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, 0, 0, vss[0], theBlockHash, theBlockHash)

	// add precommits from the rest
	signAddVotes(cs1, types.VoteTypePrecommit, nil, types.PartSetHeader{}, vs2, vs4)
	signAddVotes(cs1, types.VoteTypePrecommit, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header(), vs3)
	// precommites
	discardFromChan(voteCh, 3)

	// before we timeout to the new round set the new proposal
	prop, propBlock := decideProposal(cs1, vs2, vs2.Height, vs2.Round+1)
	propBlockParts := propBlock.MakePartSet(partSize)
	propBlockHash := propBlock.Hash()

	incrementRound(vs2, vs3, vs4)

	// timeout to new round
	<-timeoutWaitCh

	//XXX: this isnt guaranteed to get there before the timeoutPropose ...
	if err := cs1.SetProposalAndBlock(prop, propBlock, propBlockParts, "some peer"); err != nil {
		t.Fatal(err)
	}

	<-newRoundCh
	t.Log("### ONTO ROUND 1")

	/*
		Round2 (vs2, C) // B C C C // C C C _)

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
	signAddVotes(cs1, types.VoteTypePrevote, propBlockHash, propBlockParts.Header(), vs2, vs3, vs4)
	// prevotes
	discardFromChan(voteCh, 3)

	// now either we go to PrevoteWait or Precommit
	select {
	case <-timeoutWaitCh: // we're in PrevoteWait, go to Precommit
		// XXX: there's no guarantee we see the polka, this might be a precommit for nil,
		// in which case the test fails!
		<-voteCh
	case <-voteCh: // we went straight to Precommit
	}

	// we should have unlocked and locked on the new block
	validatePrecommit(t, cs1, 1, 1, vss[0], propBlockHash, propBlockHash)

	signAddVotes(cs1, types.VoteTypePrecommit, propBlockHash, propBlockParts.Header(), vs2, vs3)
	discardFromChan(voteCh, 2)

	be := <-newBlockCh
	b := be.(types.EventDataNewBlockHeader)
	re = <-newRoundCh
	rs = re.(types.EventDataRoundState).RoundState.(*cstypes.RoundState)
	if rs.Height != 2 {
		panic("Expected height to increment")
	}

	if !bytes.Equal(b.Header.Hash(), propBlockHash) {
		panic("Expected new block to be proposal block")
	}
}

// 4 vals, one precommits, other 3 polka at next round, so we unlock and precomit the polka
func TestStateLockPOLUnlock(t *testing.T) {
	cs1, vss := randConsensusState(4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]

	partSize := cs1.state.ConsensusParams.BlockPartSizeBytes

	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	timeoutProposeCh := subscribe(cs1.eventBus, types.EventQueryTimeoutPropose)
	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	unlockCh := subscribe(cs1.eventBus, types.EventQueryUnlock)
	voteCh := subscribeToVoter(cs1, cs1.privValidator.GetAddress())

	// everything done from perspective of cs1

	/*
		Round1 (cs1, B) // B B B B // B nil B nil

		eg. didn't see the 2/3 prevotes
	*/

	// start round and wait for propose and prevote
	startTestRound(cs1, cs1.Height, 0)
	<-newRoundCh
	re := <-proposalCh
	rs := re.(types.EventDataRoundState).RoundState.(*cstypes.RoundState)
	theBlockHash := rs.ProposalBlock.Hash()

	<-voteCh // prevote

	signAddVotes(cs1, types.VoteTypePrevote, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header(), vs2, vs3, vs4)

	<-voteCh //precommit

	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, 0, 0, vss[0], theBlockHash, theBlockHash)

	rs = cs1.GetRoundState()

	// add precommits from the rest
	signAddVotes(cs1, types.VoteTypePrecommit, nil, types.PartSetHeader{}, vs2, vs4)
	signAddVotes(cs1, types.VoteTypePrecommit, cs1.ProposalBlock.Hash(), cs1.ProposalBlockParts.Header(), vs3)

	// before we time out into new round, set next proposal block
	prop, propBlock := decideProposal(cs1, vs2, vs2.Height, vs2.Round+1)
	propBlockParts := propBlock.MakePartSet(partSize)

	incrementRound(vs2, vs3, vs4)

	// timeout to new round
	re = <-timeoutWaitCh
	rs = re.(types.EventDataRoundState).RoundState.(*cstypes.RoundState)
	lockedBlockHash := rs.LockedBlock.Hash()

	//XXX: this isnt guaranteed to get there before the timeoutPropose ...
	if err := cs1.SetProposalAndBlock(prop, propBlock, propBlockParts, "some peer"); err != nil {
		t.Fatal(err)
	}

	<-newRoundCh
	t.Log("#### ONTO ROUND 1")
	/*
		Round2 (vs2, C) // B nil nil nil // nil nil nil _

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
	signAddVotes(cs1, types.VoteTypePrevote, nil, types.PartSetHeader{}, vs2, vs3, vs4)

	// the polka makes us unlock and precommit nil
	<-unlockCh
	<-voteCh // precommit

	// we should have unlocked and committed nil
	// NOTE: since we don't relock on nil, the lock round is 0
	validatePrecommit(t, cs1, 1, 0, vss[0], nil, nil)

	signAddVotes(cs1, types.VoteTypePrecommit, nil, types.PartSetHeader{}, vs2, vs3)
	<-newRoundCh
}

// 4 vals
// a polka at round 1 but we miss it
// then a polka at round 2 that we lock on
// then we see the polka from round 1 but shouldn't unlock
func TestStateLockPOLSafety1(t *testing.T) {
	cs1, vss := randConsensusState(4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]

	partSize := cs1.state.ConsensusParams.BlockPartSizeBytes

	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	timeoutProposeCh := subscribe(cs1.eventBus, types.EventQueryTimeoutPropose)
	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	voteCh := subscribeToVoter(cs1, cs1.privValidator.GetAddress())

	// start round and wait for propose and prevote
	startTestRound(cs1, cs1.Height, 0)
	<-newRoundCh
	re := <-proposalCh
	rs := re.(types.EventDataRoundState).RoundState.(*cstypes.RoundState)
	propBlock := rs.ProposalBlock

	<-voteCh // prevote

	validatePrevote(t, cs1, 0, vss[0], propBlock.Hash())

	// the others sign a polka but we don't see it
	prevotes := signVotes(types.VoteTypePrevote, propBlock.Hash(), propBlock.MakePartSet(partSize).Header(), vs2, vs3, vs4)

	// before we time out into new round, set next proposer
	// and next proposal block
	/*
		_, v1 := cs1.Validators.GetByAddress(vss[0].Address)
		v1.VotingPower = 1
		if updated := cs1.Validators.Update(v1); !updated {
			panic("failed to update validator")
		}*/

	t.Logf("old prop hash %v", fmt.Sprintf("%X", propBlock.Hash()))

	// we do see them precommit nil
	signAddVotes(cs1, types.VoteTypePrecommit, nil, types.PartSetHeader{}, vs2, vs3, vs4)

	prop, propBlock := decideProposal(cs1, vs2, vs2.Height, vs2.Round+1)
	propBlockHash := propBlock.Hash()
	propBlockParts := propBlock.MakePartSet(partSize)

	incrementRound(vs2, vs3, vs4)

	//XXX: this isnt guaranteed to get there before the timeoutPropose ...
	if err := cs1.SetProposalAndBlock(prop, propBlock, propBlockParts, "some peer"); err != nil {
		t.Fatal(err)
	}

	<-newRoundCh
	t.Log("### ONTO ROUND 1")
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

	rs = re.(types.EventDataRoundState).RoundState.(*cstypes.RoundState)

	if rs.LockedBlock != nil {
		panic("we should not be locked!")
	}
	t.Logf("new prop hash %v", fmt.Sprintf("%X", propBlockHash))
	// go to prevote, prevote for proposal block
	<-voteCh
	validatePrevote(t, cs1, 1, vss[0], propBlockHash)

	// now we see the others prevote for it, so we should lock on it
	signAddVotes(cs1, types.VoteTypePrevote, propBlockHash, propBlockParts.Header(), vs2, vs3, vs4)

	<-voteCh // precommit

	// we should have precommitted
	validatePrecommit(t, cs1, 1, 1, vss[0], propBlockHash, propBlockHash)

	signAddVotes(cs1, types.VoteTypePrecommit, nil, types.PartSetHeader{}, vs2, vs3)

	<-timeoutWaitCh

	incrementRound(vs2, vs3, vs4)

	<-newRoundCh

	t.Log("### ONTO ROUND 2")
	/*Round3
	we see the polka from round 1 but we shouldn't unlock!
	*/

	// timeout of propose
	<-timeoutProposeCh

	// finish prevote
	<-voteCh

	// we should prevote what we're locked on
	validatePrevote(t, cs1, 2, vss[0], propBlockHash)

	newStepCh := subscribe(cs1.eventBus, types.EventQueryNewRoundStep)

	// add prevotes from the earlier round
	addVotes(cs1, prevotes...)

	t.Log("Done adding prevotes!")

	ensureNoNewStep(newStepCh)
}

// 4 vals.
// polka P0 at R0, P1 at R1, and P2 at R2,
// we lock on P0 at R0, don't see P1, and unlock using P2 at R2
// then we should make sure we don't lock using P1

// What we want:
// dont see P0, lock on P1 at R1, dont unlock using P0 at R2
func TestStateLockPOLSafety2(t *testing.T) {
	cs1, vss := randConsensusState(4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]

	partSize := cs1.state.ConsensusParams.BlockPartSizeBytes

	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	timeoutProposeCh := subscribe(cs1.eventBus, types.EventQueryTimeoutPropose)
	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	unlockCh := subscribe(cs1.eventBus, types.EventQueryUnlock)
	voteCh := subscribeToVoter(cs1, cs1.privValidator.GetAddress())

	// the block for R0: gets polkad but we miss it
	// (even though we signed it, shhh)
	_, propBlock0 := decideProposal(cs1, vss[0], cs1.Height, cs1.Round)
	propBlockHash0 := propBlock0.Hash()
	propBlockParts0 := propBlock0.MakePartSet(partSize)

	// the others sign a polka but we don't see it
	prevotes := signVotes(types.VoteTypePrevote, propBlockHash0, propBlockParts0.Header(), vs2, vs3, vs4)

	// the block for round 1
	prop1, propBlock1 := decideProposal(cs1, vs2, vs2.Height, vs2.Round+1)
	propBlockHash1 := propBlock1.Hash()
	propBlockParts1 := propBlock1.MakePartSet(partSize)
	propBlockID1 := types.BlockID{propBlockHash1, propBlockParts1.Header()}

	incrementRound(vs2, vs3, vs4)

	cs1.updateRoundStep(0, cstypes.RoundStepPrecommitWait)

	t.Log("### ONTO Round 1")
	// jump in at round 1
	height := cs1.Height
	startTestRound(cs1, height, 1)
	<-newRoundCh

	if err := cs1.SetProposalAndBlock(prop1, propBlock1, propBlockParts1, "some peer"); err != nil {
		t.Fatal(err)
	}
	<-proposalCh

	<-voteCh // prevote

	signAddVotes(cs1, types.VoteTypePrevote, propBlockHash1, propBlockParts1.Header(), vs2, vs3, vs4)

	<-voteCh // precommit
	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, 1, 1, vss[0], propBlockHash1, propBlockHash1)

	// add precommits from the rest
	signAddVotes(cs1, types.VoteTypePrecommit, nil, types.PartSetHeader{}, vs2, vs4)
	signAddVotes(cs1, types.VoteTypePrecommit, propBlockHash1, propBlockParts1.Header(), vs3)

	incrementRound(vs2, vs3, vs4)

	// timeout of precommit wait to new round
	<-timeoutWaitCh

	// in round 2 we see the polkad block from round 0
	newProp := types.NewProposal(height, 2, propBlockParts0.Header(), 0, propBlockID1)
	if err := vs3.SignProposal(config.ChainID(), newProp); err != nil {
		t.Fatal(err)
	}
	if err := cs1.SetProposalAndBlock(newProp, propBlock0, propBlockParts0, "some peer"); err != nil {
		t.Fatal(err)
	}

	// Add the pol votes
	addVotes(cs1, prevotes...)

	<-newRoundCh
	t.Log("### ONTO Round 2")
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
		panic("validator unlocked using an old polka")
	case <-voteCh:
		// prevote our locked block
	}
	validatePrevote(t, cs1, 2, vss[0], propBlockHash1)

}

//------------------------------------------------------------------------------------------
// SlashingSuite
// TODO: Slashing

/*
func TestStateSlashingPrevotes(t *testing.T) {
	cs1, vss := randConsensusState(2)
	vs2 := vss[1]


	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	voteCh := subscribeToVoter(cs1, cs1.privValidator.GetAddress())

	// start round and wait for propose and prevote
	startTestRound(cs1, cs1.Height, 0)
	<-newRoundCh
	re := <-proposalCh
	<-voteCh // prevote

	rs := re.(types.EventDataRoundState).RoundState.(*cstypes.RoundState)

	// we should now be stuck in limbo forever, waiting for more prevotes
	// add one for a different block should cause us to go into prevote wait
	hash := rs.ProposalBlock.Hash()
	hash[0] = byte(hash[0]+1) % 255
	signAddVotes(cs1, types.VoteTypePrevote, hash, rs.ProposalBlockParts.Header(), vs2)

	<-timeoutWaitCh

	// NOTE: we have to send the vote for different block first so we don't just go into precommit round right
	// away and ignore more prevotes (and thus fail to slash!)

	// add the conflicting vote
	signAddVotes(cs1, types.VoteTypePrevote, rs.ProposalBlock.Hash(), rs.ProposalBlockParts.Header(), vs2)

	// XXX: Check for existence of Dupeout info
}

func TestStateSlashingPrecommits(t *testing.T) {
	cs1, vss := randConsensusState(2)
	vs2 := vss[1]


	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	voteCh := subscribeToVoter(cs1, cs1.privValidator.GetAddress())

	// start round and wait for propose and prevote
	startTestRound(cs1, cs1.Height, 0)
	<-newRoundCh
	re := <-proposalCh
	<-voteCh // prevote

	// add prevote from vs2
	signAddVotes(cs1, types.VoteTypePrevote, rs.ProposalBlock.Hash(), rs.ProposalBlockParts.Header(), vs2)

	<-voteCh // precommit

	// we should now be stuck in limbo forever, waiting for more prevotes
	// add one for a different block should cause us to go into prevote wait
	hash := rs.ProposalBlock.Hash()
	hash[0] = byte(hash[0]+1) % 255
	signAddVotes(cs1, types.VoteTypePrecommit, hash, rs.ProposalBlockParts.Header(), vs2)

	// NOTE: we have to send the vote for different block first so we don't just go into precommit round right
	// away and ignore more prevotes (and thus fail to slash!)

	// add precommit from vs2
	signAddVotes(cs1, types.VoteTypePrecommit, rs.ProposalBlock.Hash(), rs.ProposalBlockParts.Header(), vs2)

	// XXX: Check for existence of Dupeout info
}
*/

//------------------------------------------------------------------------------------------
// CatchupSuite

//------------------------------------------------------------------------------------------
// HaltSuite

// 4 vals.
// we receive a final precommit after going into next round, but others might have gone to commit already!
func TestStateHalt1(t *testing.T) {
	cs1, vss := randConsensusState(4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]

	partSize := cs1.state.ConsensusParams.BlockPartSizeBytes

	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	newBlockCh := subscribe(cs1.eventBus, types.EventQueryNewBlock)
	voteCh := subscribeToVoter(cs1, cs1.privValidator.GetAddress())

	// start round and wait for propose and prevote
	startTestRound(cs1, cs1.Height, 0)
	<-newRoundCh
	re := <-proposalCh
	rs := re.(types.EventDataRoundState).RoundState.(*cstypes.RoundState)
	propBlock := rs.ProposalBlock
	propBlockParts := propBlock.MakePartSet(partSize)

	<-voteCh // prevote

	signAddVotes(cs1, types.VoteTypePrevote, propBlock.Hash(), propBlockParts.Header(), vs3, vs4)
	<-voteCh // precommit

	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, 0, 0, vss[0], propBlock.Hash(), propBlock.Hash())

	// add precommits from the rest
	signAddVotes(cs1, types.VoteTypePrecommit, nil, types.PartSetHeader{}, vs2) // didnt receive proposal
	signAddVotes(cs1, types.VoteTypePrecommit, propBlock.Hash(), propBlockParts.Header(), vs3)
	// we receive this later, but vs3 might receive it earlier and with ours will go to commit!
	precommit4 := signVote(vs4, types.VoteTypePrecommit, propBlock.Hash(), propBlockParts.Header())

	incrementRound(vs2, vs3, vs4)

	// timeout to new round
	<-timeoutWaitCh
	re = <-newRoundCh
	rs = re.(types.EventDataRoundState).RoundState.(*cstypes.RoundState)

	t.Log("### ONTO ROUND 1")
	/*Round2
	// we timeout and prevote our lock
	// a polka happened but we didn't see it!
	*/

	// go to prevote, prevote for locked block
	<-voteCh // prevote
	validatePrevote(t, cs1, 0, vss[0], rs.LockedBlock.Hash())

	// now we receive the precommit from the previous round
	addVotes(cs1, precommit4)

	// receiving that precommit should take us straight to commit
	<-newBlockCh
	re = <-newRoundCh
	rs = re.(types.EventDataRoundState).RoundState.(*cstypes.RoundState)

	if rs.Height != 2 {
		panic("expected height to increment")
	}
}

// subscribe subscribes test client to the given query and returns a channel with cap = 1.
func subscribe(eventBus *types.EventBus, q tmpubsub.Query) <-chan interface{} {
	out := make(chan interface{}, 1)
	err := eventBus.Subscribe(context.Background(), testSubscriber, q, out)
	if err != nil {
		panic(fmt.Sprintf("failed to subscribe %s to %v", testSubscriber, q))
	}
	return out
}

// discardFromChan reads n values from the channel.
func discardFromChan(ch <-chan interface{}, n int) {
	for i := 0; i < n; i++ {
		<-ch
	}
}
