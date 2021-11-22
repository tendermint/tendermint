package consensus

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/crypto/tmhash"
	cstypes "github.com/tendermint/tendermint/internal/consensus/types"
	p2pmock "github.com/tendermint/tendermint/internal/p2p/mock"
	"github.com/tendermint/tendermint/libs/log"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

/*

ProposeSuite
x * TestProposerSelection0 - round robin ordering, round 0
x * TestProposerSelection2 - round robin ordering, round 2++
x * TestEnterProposeNoValidator - timeout into prevote round
x * TestEnterPropose - finish propose without timing out (we have the proposal)
x * TestBadProposal - 2 vals, bad proposal (bad block state hash), should prevote and precommit nil
x * TestOversizedBlock - block with too many txs should be rejected
FullRoundSuite
x * TestFullRound1 - 1 val, full successful round
x * TestFullRoundNil - 1 val, full round of nil
x * TestFullRound2 - 2 vals, both required for full round
LockSuite
x * TestStateLock_NoPOL - 2 vals, 4 rounds. one val locked, precommits nil every round except first.
x * TestStateLock_POLUpdateLock - 4 vals, one precommits,
other 3 polka at next round, so we unlock and precomit the polka
x * TestStateLock_POLRelock - 4 vals, polka in round 1 and polka in round 2.
Ensure validator updates locked round.
x_*_TestStateLock_POLDoesNotUnlock 4 vals, one precommits, other 3 polka nil at
next round, so we precommit nil but maintain lock
x * TestStateLock_MissingProposalWhenPOLSeenDoesNotUpdateLock - 4 vals, 1 misses proposal but sees POL.
x * TestStateLock_MissingProposalWhenPOLSeenDoesNotUnlock - 4 vals, 1 misses proposal but sees POL.
x * TestStateLock_POLSafety1 - 4 vals. We shouldn't change lock based on polka at earlier round
x * TestStateLock_POLSafety2 - 4 vals. After unlocking, we shouldn't relock based on polka at earlier round
x_*_TestState_PrevotePOLFromPreviousRound 4 vals, prevote a proposal if a POL was seen for it in a previous round.
  * TestNetworkLock - once +1/3 precommits, network should be locked
  * TestNetworkLockPOL - once +1/3 precommits, the block with more recent polka is committed
SlashingSuite
x * TestStateSlashing_Prevotes - a validator prevoting twice in a round gets slashed
x * TestStateSlashing_Precommits - a validator precomitting twice in a round gets slashed
CatchupSuite
  * TestCatchup - if we might be behind and we've seen any 2/3 prevotes, round skip to new round, precommit, or prevote
HaltSuite
x * TestHalt1 - if we see +2/3 precommits after timing out into new round, we should still commit

*/

//----------------------------------------------------------------------------------------------------
// ProposeSuite

func TestStateProposerSelection0(t *testing.T) {
	config := configSetup(t)

	cs1, vss := randState(config, 4)
	height, round := cs1.Height, cs1.Round

	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)

	startTestRound(cs1, height, round)

	// Wait for new round so proposer is set.
	ensureNewRound(t, newRoundCh, height, round)

	// Commit a block and ensure proposer for the next height is correct.
	prop := cs1.GetRoundState().Validators.GetProposer()
	pv, err := cs1.privValidator.GetPubKey(context.Background())
	require.NoError(t, err)
	address := pv.Address()
	if !bytes.Equal(prop.Address, address) {
		t.Fatalf("expected proposer to be validator %d. Got %X", 0, prop.Address)
	}

	// Wait for complete proposal.
	ensureNewProposal(t, proposalCh, height, round)

	rs := cs1.GetRoundState()
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), types.BlockID{
		Hash:          rs.ProposalBlock.Hash(),
		PartSetHeader: rs.ProposalBlockParts.Header(),
	}, vss[1:]...)

	// Wait for new round so next validator is set.
	ensureNewRound(t, newRoundCh, height+1, 0)

	prop = cs1.GetRoundState().Validators.GetProposer()
	pv1, err := vss[1].GetPubKey(context.Background())
	require.NoError(t, err)
	addr := pv1.Address()
	if !bytes.Equal(prop.Address, addr) {
		panic(fmt.Sprintf("expected proposer to be validator %d. Got %X", 1, prop.Address))
	}
}

// Now let's do it all again, but starting from round 2 instead of 0
func TestStateProposerSelection2(t *testing.T) {
	config := configSetup(t)

	cs1, vss := randState(config, 4) // test needs more work for more than 3 validators
	height := cs1.Height
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)

	// this time we jump in at round 2
	incrementRound(vss[1:]...)
	incrementRound(vss[1:]...)

	var round int32 = 2
	startTestRound(cs1, height, round)

	ensureNewRound(t, newRoundCh, height, round) // wait for the new round

	// everyone just votes nil. we get a new proposer each round
	for i := int32(0); int(i) < len(vss); i++ {
		prop := cs1.GetRoundState().Validators.GetProposer()
		pvk, err := vss[int(i+round)%len(vss)].GetPubKey(context.Background())
		require.NoError(t, err)
		addr := pvk.Address()
		correctProposer := addr
		if !bytes.Equal(prop.Address, correctProposer) {
			panic(fmt.Sprintf(
				"expected RoundState.Validators.GetProposer() to be validator %d. Got %X",
				int(i+2)%len(vss),
				prop.Address))
		}

		signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), types.BlockID{}, vss[1:]...)
		ensureNewRound(t, newRoundCh, height, i+round+1) // wait for the new round event each round
		incrementRound(vss[1:]...)
	}

}

// a non-validator should timeout into the prevote round
func TestStateEnterProposeNoPrivValidator(t *testing.T) {
	config := configSetup(t)

	cs, _ := randState(config, 1)
	cs.SetPrivValidator(nil)
	height, round := cs.Height, cs.Round

	// Listen for propose timeout event
	timeoutCh := subscribe(cs.eventBus, types.EventQueryTimeoutPropose)

	startTestRound(cs, height, round)

	// if we're not a validator, EnterPropose should timeout
	ensureNewTimeout(t, timeoutCh, height, round, cs.config.TimeoutPropose.Nanoseconds())

	if cs.GetRoundState().Proposal != nil {
		t.Error("Expected to make no proposal, since no privValidator")
	}
}

// a validator should not timeout of the prevote round (TODO: unless the block is really big!)
func TestStateEnterProposeYesPrivValidator(t *testing.T) {
	config := configSetup(t)

	cs, _ := randState(config, 1)
	height, round := cs.Height, cs.Round

	// Listen for propose timeout event

	timeoutCh := subscribe(cs.eventBus, types.EventQueryTimeoutPropose)
	proposalCh := subscribe(cs.eventBus, types.EventQueryCompleteProposal)

	cs.enterNewRound(height, round)
	cs.startRoutines(3)

	ensureNewProposal(t, proposalCh, height, round)

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
	ensureNoNewTimeout(t, timeoutCh, cs.config.TimeoutPropose.Nanoseconds())
}

func TestStateBadProposal(t *testing.T) {
	config := configSetup(t)

	cs1, vss := randState(config, 2)
	height, round := cs1.Height, cs1.Round
	vs2 := vss[1]

	partSize := types.BlockPartSizeBytes

	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	voteCh := subscribe(cs1.eventBus, types.EventQueryVote)

	propBlock, _ := cs1.createProposalBlock() // changeProposer(t, cs1, vs2)

	// make the second validator the proposer by incrementing round
	round++
	incrementRound(vss[1:]...)

	// make the block bad by tampering with statehash
	stateHash := propBlock.AppHash
	if len(stateHash) == 0 {
		stateHash = make([]byte, 32)
	}
	stateHash[0] = (stateHash[0] + 1) % 255
	propBlock.AppHash = stateHash
	propBlockParts := propBlock.MakePartSet(partSize)
	blockID := types.BlockID{Hash: propBlock.Hash(), PartSetHeader: propBlockParts.Header()}
	proposal := types.NewProposal(vs2.Height, round, -1, blockID)
	p := proposal.ToProto()
	if err := vs2.SignProposal(context.Background(), config.ChainID(), p); err != nil {
		t.Fatal("failed to sign bad proposal", err)
	}

	proposal.Signature = p.Signature

	// set the proposal block
	if err := cs1.SetProposalAndBlock(proposal, propBlock, propBlockParts, "some peer"); err != nil {
		t.Fatal(err)
	}

	// start the machine
	startTestRound(cs1, height, round)

	// wait for proposal
	ensureProposal(t, proposalCh, height, round, blockID)

	// wait for prevote
	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], nil)

	// add bad prevote from vs2 and wait for it
	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), blockID, vs2)
	ensurePrevote(t, voteCh, height, round)

	// wait for precommit
	ensurePrecommit(t, voteCh, height, round)
	validatePrecommit(t, cs1, round, -1, vss[0], nil, nil)
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), blockID, vs2)
}

func TestStateOversizedBlock(t *testing.T) {
	config := configSetup(t)

	cs1, vss := randState(config, 2)
	cs1.state.ConsensusParams.Block.MaxBytes = 2000
	height, round := cs1.Height, cs1.Round
	vs2 := vss[1]

	partSize := types.BlockPartSizeBytes

	timeoutProposeCh := subscribe(cs1.eventBus, types.EventQueryTimeoutPropose)
	voteCh := subscribe(cs1.eventBus, types.EventQueryVote)

	propBlock, _ := cs1.createProposalBlock()
	propBlock.Data.Txs = []types.Tx{tmrand.Bytes(2001)}
	propBlock.Header.DataHash = propBlock.Data.Hash()

	// make the second validator the proposer by incrementing round
	round++
	incrementRound(vss[1:]...)

	propBlockParts := propBlock.MakePartSet(partSize)
	blockID := types.BlockID{Hash: propBlock.Hash(), PartSetHeader: propBlockParts.Header()}
	proposal := types.NewProposal(height, round, -1, blockID)
	p := proposal.ToProto()
	if err := vs2.SignProposal(context.Background(), config.ChainID(), p); err != nil {
		t.Fatal("failed to sign bad proposal", err)
	}
	proposal.Signature = p.Signature

	totalBytes := 0
	for i := 0; i < int(propBlockParts.Total()); i++ {
		part := propBlockParts.GetPart(i)
		totalBytes += len(part.Bytes)
	}

	if err := cs1.SetProposalAndBlock(proposal, propBlock, propBlockParts, "some peer"); err != nil {
		t.Fatal(err)
	}

	// start the machine
	startTestRound(cs1, height, round)

	t.Log("Block Sizes", "Limit", cs1.state.ConsensusParams.Block.MaxBytes, "Current", totalBytes)

	// c1 should log an error with the block part message as it exceeds the consensus params. The
	// block is not added to cs.ProposalBlock so the node timeouts.
	ensureNewTimeout(t, timeoutProposeCh, height, round, cs1.config.Propose(round).Nanoseconds())

	// and then should send nil prevote and precommit regardless of whether other validators prevote and
	// precommit on it
	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], nil)
	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), blockID, vs2)
	ensurePrevote(t, voteCh, height, round)
	ensurePrecommit(t, voteCh, height, round)
	validatePrecommit(t, cs1, round, -1, vss[0], nil, nil)
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), blockID, vs2)
}

//----------------------------------------------------------------------------------------------------
// FullRoundSuite

// propose, prevote, and precommit a block
func TestStateFullRound1(t *testing.T) {
	config := configSetup(t)

	cs, vss := randState(config, 1)
	height, round := cs.Height, cs.Round

	// NOTE: buffer capacity of 0 ensures we can validate prevote and last commit
	// before consensus can move to the next height (and cause a race condition)
	if err := cs.eventBus.Stop(); err != nil {
		t.Error(err)
	}
	eventBus := types.NewEventBusWithBufferCapacity(0)
	eventBus.SetLogger(log.TestingLogger().With("module", "events"))
	cs.SetEventBus(eventBus)
	if err := eventBus.Start(); err != nil {
		t.Error(err)
	}

	voteCh := subscribeUnBuffered(cs.eventBus, types.EventQueryVote)
	propCh := subscribe(cs.eventBus, types.EventQueryCompleteProposal)
	newRoundCh := subscribe(cs.eventBus, types.EventQueryNewRound)

	// Maybe it would be better to call explicitly startRoutines(4)
	startTestRound(cs, height, round)

	ensureNewRound(t, newRoundCh, height, round)

	ensureNewProposal(t, propCh, height, round)
	propBlockHash := cs.GetRoundState().ProposalBlock.Hash()

	ensurePrevote(t, voteCh, height, round) // wait for prevote
	validatePrevote(t, cs, round, vss[0], propBlockHash)

	ensurePrecommit(t, voteCh, height, round) // wait for precommit

	// we're going to roll right into new height
	ensureNewRound(t, newRoundCh, height+1, 0)

	validateLastPrecommit(t, cs, vss[0], propBlockHash)
}

// nil is proposed, so prevote and precommit nil
func TestStateFullRoundNil(t *testing.T) {
	config := configSetup(t)

	cs, vss := randState(config, 1)
	height, round := cs.Height, cs.Round

	voteCh := subscribeUnBuffered(cs.eventBus, types.EventQueryVote)

	cs.enterPrevote(height, round)
	cs.startRoutines(4)

	ensurePrevote(t, voteCh, height, round)   // prevote
	ensurePrecommit(t, voteCh, height, round) // precommit

	// should prevote and precommit nil
	validatePrevoteAndPrecommit(t, cs, round, -1, vss[0], nil, nil)
}

// run through propose, prevote, precommit commit with two validators
// where the first validator has to wait for votes from the second
func TestStateFullRound2(t *testing.T) {
	config := configSetup(t)

	cs1, vss := randState(config, 2)
	vs2 := vss[1]
	height, round := cs1.Height, cs1.Round

	voteCh := subscribeUnBuffered(cs1.eventBus, types.EventQueryVote)
	newBlockCh := subscribe(cs1.eventBus, types.EventQueryNewBlock)

	// start round and wait for propose and prevote
	startTestRound(cs1, height, round)

	ensurePrevote(t, voteCh, height, round) // prevote

	// we should be stuck in limbo waiting for more prevotes
	rs := cs1.GetRoundState()
	blockID := types.BlockID{Hash: rs.ProposalBlock.Hash(), PartSetHeader: rs.ProposalBlockParts.Header()}

	// prevote arrives from vs2:
	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), blockID, vs2)
	ensurePrevote(t, voteCh, height, round) // prevote

	ensurePrecommit(t, voteCh, height, round) // precommit
	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, 0, 0, vss[0], blockID.Hash, blockID.Hash)

	// we should be stuck in limbo waiting for more precommits

	// precommit arrives from vs2:
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), blockID, vs2)
	ensurePrecommit(t, voteCh, height, round)

	// wait to finish commit, propose in next height
	ensureNewBlock(t, newBlockCh, height)
}

//------------------------------------------------------------------------------------------
// LockSuite

// two validators, 4 rounds.
// two vals take turns proposing. val1 locks on first one, precommits nil on everything else
func TestStateLock_NoPOL(t *testing.T) {
	config := configSetup(t)

	cs1, vss := randState(config, 2)
	vs2 := vss[1]
	height, round := cs1.Height, cs1.Round

	partSize := types.BlockPartSizeBytes

	timeoutProposeCh := subscribe(cs1.eventBus, types.EventQueryTimeoutPropose)
	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	voteCh := subscribeUnBuffered(cs1.eventBus, types.EventQueryVote)
	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)

	/*
		Round1 (cs1, B) // B B // B B2
	*/

	// start round and wait for prevote
	cs1.enterNewRound(height, round)
	cs1.startRoutines(0)

	ensureNewRound(t, newRoundCh, height, round)

	ensureNewProposal(t, proposalCh, height, round)
	roundState := cs1.GetRoundState()
	initialBlockID := types.BlockID{
		Hash:          roundState.ProposalBlock.Hash(),
		PartSetHeader: roundState.ProposalBlockParts.Header(),
	}

	ensurePrevote(t, voteCh, height, round) // prevote

	// we should now be stuck in limbo forever, waiting for more prevotes
	// prevote arrives from vs2:
	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), initialBlockID, vs2)
	ensurePrevote(t, voteCh, height, round) // prevote
	validatePrevote(t, cs1, round, vss[0], initialBlockID.Hash)

	// the proposed block should now be locked and our precommit added
	ensurePrecommit(t, voteCh, height, round)
	validatePrecommit(t, cs1, round, round, vss[0], initialBlockID.Hash, initialBlockID.Hash)

	// we should now be stuck in limbo forever, waiting for more precommits
	// lets add one for a different block
	hash := make([]byte, len(initialBlockID.Hash))
	copy(hash, initialBlockID.Hash)
	hash[0] = (hash[0] + 1) % 255
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), types.BlockID{
		Hash:          hash,
		PartSetHeader: initialBlockID.PartSetHeader,
	}, vs2)
	ensurePrecommit(t, voteCh, height, round) // precommit

	// (note we're entering precommit for a second time this round)
	// but with invalid args. then we enterPrecommitWait, and the timeout to new round
	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Precommit(round).Nanoseconds())

	///

	round++ // moving to the next round
	ensureNewRound(t, newRoundCh, height, round)
	t.Log("#### ONTO ROUND 1")
	/*
		Round2 (cs1, B) // B B2
	*/

	incrementRound(vs2)

	// now we're on a new round and not the proposer, so wait for timeout
	ensureNewTimeout(t, timeoutProposeCh, height, round, cs1.config.Propose(round).Nanoseconds())

	rs := cs1.GetRoundState()

	if rs.ProposalBlock != nil {
		t.Fatal("Expected proposal block to be nil")
	}

	// we should have prevoted nil since we did not see a proposal in the round.
	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], nil)

	// add a conflicting prevote from the other validator
	conflictingBlockID := types.BlockID{Hash: hash, PartSetHeader: rs.LockedBlock.MakePartSet(partSize).Header()}
	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), conflictingBlockID, vs2)
	ensurePrevote(t, voteCh, height, round)

	// now we're going to enter prevote again, but with invalid args
	// and then prevote wait, which should timeout. then wait for precommit
	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Prevote(round).Nanoseconds())

	// the proposed block should still be locked block.
	// we should precommit nil and be locked on the proposal.
	ensurePrecommit(t, voteCh, height, round)
	validatePrecommit(t, cs1, round, 0, vss[0], nil, initialBlockID.Hash)

	// add conflicting precommit from vs2
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), conflictingBlockID, vs2)
	ensurePrecommit(t, voteCh, height, round)

	// (note we're entering precommit for a second time this round, but with invalid args
	// then we enterPrecommitWait and timeout into NewRound
	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Precommit(round).Nanoseconds())

	round++ // entering new round
	ensureNewRound(t, newRoundCh, height, round)
	t.Log("#### ONTO ROUND 2")
	/*
		Round3 (vs2, _) // B, B2
	*/

	incrementRound(vs2)

	ensureNewProposal(t, proposalCh, height, round)
	rs = cs1.GetRoundState()

	// now we're on a new round and are the proposer
	if !bytes.Equal(rs.ProposalBlock.Hash(), rs.LockedBlock.Hash()) {
		panic(fmt.Sprintf(
			"Expected proposal block to be locked block. Got %v, Expected %v",
			rs.ProposalBlock,
			rs.LockedBlock))
	}

	ensurePrevote(t, voteCh, height, round) // prevote
	validatePrevote(t, cs1, round, vss[0], rs.LockedBlock.Hash())

	newBlockID := types.BlockID{Hash: hash, PartSetHeader: rs.ProposalBlock.MakePartSet(partSize).Header()}
	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), newBlockID, vs2)
	ensurePrevote(t, voteCh, height, round)

	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Prevote(round).Nanoseconds())
	ensurePrecommit(t, voteCh, height, round) // precommit

	validatePrecommit(t, cs1, round, 0, vss[0], nil, initialBlockID.Hash) // precommit nil but be locked on proposal

	signAddVotes(
		cs1,
		tmproto.PrecommitType,
		config.ChainID(),

		newBlockID,
		vs2) // NOTE: conflicting precommits at same height
	ensurePrecommit(t, voteCh, height, round)

	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Precommit(round).Nanoseconds())

	cs2, _ := randState(config, 2) // needed so generated block is different than locked block
	// before we time out into new round, set next proposal block
	prop, propBlock := decideProposal(t, cs2, vs2, vs2.Height, vs2.Round+1)
	if prop == nil || propBlock == nil {
		t.Fatal("Failed to create proposal block with vs2")
	}
	propBlockID := types.BlockID{
		Hash:          propBlock.Hash(),
		PartSetHeader: propBlock.MakePartSet(partSize).Header(),
	}

	incrementRound(vs2)

	round++ // entering new round
	ensureNewRound(t, newRoundCh, height, round)
	t.Log("#### ONTO ROUND 3")
	/*
		Round4 (vs2, C) // B C // B C
	*/

	// now we're on a new round and not the proposer
	// so set the proposal block
	if err := cs1.SetProposalAndBlock(prop, propBlock, propBlock.MakePartSet(partSize), ""); err != nil {
		t.Fatal(err)
	}

	ensureNewProposal(t, proposalCh, height, round)

	// prevote for nil since we did not see a proposal for our locked block in the round.
	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, 3, vss[0], nil)

	// prevote for proposed block
	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), propBlockID, vs2)
	ensurePrevote(t, voteCh, height, round)

	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Prevote(round).Nanoseconds())
	ensurePrecommit(t, voteCh, height, round)
	validatePrecommit(t, cs1, round, 0, vss[0], nil, initialBlockID.Hash) // precommit nil but locked on proposal

	signAddVotes(
		cs1,
		tmproto.PrecommitType,
		config.ChainID(),

		propBlockID,
		vs2) // NOTE: conflicting precommits at same height
	ensurePrecommit(t, voteCh, height, round)
}

// TestStateLock_POLUpdateLock tests that a validator maintains updates its locked
// block if the following conditions are met within a round:
// 1. The validator received a valid proposal for the block
// 2. The validator received prevotes representing greater than 2/3 of the voting
// power on the network for the block.
func TestStateLock_POLUpdateLock(t *testing.T) {
	config := configSetup(t)

	cs1, vss := randState(config, 4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round

	partSize := types.BlockPartSizeBytes

	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	pv1, err := cs1.privValidator.GetPubKey(context.Background())
	require.NoError(t, err)
	addr := pv1.Address()
	voteCh := subscribeToVoter(cs1, addr)
	lockCh := subscribe(cs1.eventBus, types.EventQueryLock)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)

	/*
		Round 0:
		cs1 creates a proposal for block B.
		Send a prevote for B from each of the validators to cs1.
		Send a precommit for nil from all of the validators to cs1.

		This ensures that cs1 will lock on B in this round but not precommit it.
	*/
	t.Log("### Starting Round 0")

	startTestRound(cs1, height, round)

	ensureNewRound(t, newRoundCh, height, round)
	ensureNewProposal(t, proposalCh, height, round)
	rs := cs1.GetRoundState()
	initialBlockID := types.BlockID{
		Hash:          rs.ProposalBlock.Hash(),
		PartSetHeader: rs.ProposalBlockParts.Header(),
	}

	ensurePrevote(t, voteCh, height, round)

	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), initialBlockID, vs2, vs3, vs4)

	// check that the validator generates a Lock event.
	ensureLock(t, lockCh, height, round)

	// the proposed block should now be locked and our precommit added.
	ensurePrecommit(t, voteCh, height, round)
	validatePrecommit(t, cs1, round, round, vss[0], initialBlockID.Hash, initialBlockID.Hash)

	// add precommits from the rest of the validators.
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), types.BlockID{}, vs2, vs3, vs4)

	// timeout to new round.
	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Precommit(round).Nanoseconds())

	/*
		Round 1:
		Create a block, D and send a proposal for it to cs1
		Send a prevote for D from each of the validators to cs1.
		Send a precommit for nil from all of the validtors to cs1.

		Check that cs1 is now locked on the new block, D and no longer on the old block.
	*/
	t.Log("### Starting Round 1")
	incrementRound(vs2, vs3, vs4)
	round++

	// Generate a new proposal block.
	cs2 := newState(cs1.state, vs2, kvstore.NewApplication())
	propR1, propBlockR1 := decideProposal(t, cs2, vs2, vs2.Height, vs2.Round)
	propBlockR1Parts := propBlockR1.MakePartSet(partSize)
	propBlockR1Hash := propBlockR1.Hash()
	r1BlockID := types.BlockID{
		Hash:          propBlockR1Hash,
		PartSetHeader: propBlockR1Parts.Header(),
	}
	require.NotEqual(t, propBlockR1Hash, initialBlockID.Hash)
	if err := cs1.SetProposalAndBlock(propR1, propBlockR1, propBlockR1Parts, "some peer"); err != nil {
		t.Fatal(err)
	}

	ensureNewRound(t, newRoundCh, height, round)

	// ensure that the validator receives the proposal.
	ensureNewProposal(t, proposalCh, height, round)

	// Prevote our nil since the proposal does not match our locked block.
	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], nil)

	// Add prevotes from the remainder of the validators for the new locked block.
	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), r1BlockID, vs2, vs3, vs4)

	// Check that we lock on a new block.
	ensureLock(t, lockCh, height, round)

	ensurePrecommit(t, voteCh, height, round)

	// We should now be locked on the new block and prevote it since we saw a sufficient amount
	// prevote for the block.
	validatePrecommit(t, cs1, round, round, vss[0], propBlockR1Hash, propBlockR1Hash)
}

// TestStateLock_POLRelock tests that a validator updates its locked round if
// it receives votes representing over 2/3 of the voting power on the network
// for a block that it is already locked in.
func TestStateLock_POLRelock(t *testing.T) {
	config := configSetup(t)

	cs1, vss := randState(config, 4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round

	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	pv1, err := cs1.privValidator.GetPubKey(context.Background())
	require.NoError(t, err)
	addr := pv1.Address()
	voteCh := subscribeToVoter(cs1, addr)
	lockCh := subscribe(cs1.eventBus, types.EventQueryLock)
	relockCh := subscribe(cs1.eventBus, types.EventQueryRelock)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)

	/*
		Round 0:
		cs1 creates a proposal for block B.
		Send a prevote for B from each of the validators to cs1.
		Send a precommit for nil from all of the validators to cs1.

		This ensures that cs1 will lock on B in this round but not precommit it.
	*/
	t.Log("### Starting Round 0")

	startTestRound(cs1, height, round)

	ensureNewRound(t, newRoundCh, height, round)
	ensureNewProposal(t, proposalCh, height, round)
	rs := cs1.GetRoundState()
	theBlock := rs.ProposalBlock
	theBlockParts := rs.ProposalBlockParts
	blockID := types.BlockID{
		Hash:          rs.ProposalBlock.Hash(),
		PartSetHeader: rs.ProposalBlockParts.Header(),
	}

	ensurePrevote(t, voteCh, height, round)

	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), blockID, vs2, vs3, vs4)

	// check that the validator generates a Lock event.
	ensureLock(t, lockCh, height, round)

	// the proposed block should now be locked and our precommit added.
	ensurePrecommit(t, voteCh, height, round)
	validatePrecommit(t, cs1, round, round, vss[0], blockID.Hash, blockID.Hash)

	// add precommits from the rest of the validators.
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), types.BlockID{}, vs2, vs3, vs4)

	// timeout to new round.
	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Precommit(round).Nanoseconds())

	/*
		Round 1:
		Create a proposal for block B, the same block from round 1.
		Send a prevote for B from each of the validators to cs1.
		Send a precommit for nil from all of the validtors to cs1.

		Check that cs1 updates its 'locked round' value to the current round.
	*/
	t.Log("### Starting Round 1")
	incrementRound(vs2, vs3, vs4)
	round++
	propR1 := types.NewProposal(height, round, cs1.ValidRound, blockID)
	p := propR1.ToProto()
	if err := vs2.SignProposal(context.Background(), cs1.state.ChainID, p); err != nil {
		t.Fatalf("error signing proposal: %s", err)
	}
	propR1.Signature = p.Signature
	if err := cs1.SetProposalAndBlock(propR1, theBlock, theBlockParts, ""); err != nil {
		t.Fatal(err)
	}

	ensureNewRound(t, newRoundCh, height, round)

	// ensure that the validator receives the proposal.
	ensureNewProposal(t, proposalCh, height, round)

	// Prevote our locked block since it matches the propsal seen in this round.
	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], blockID.Hash)

	// Add prevotes from the remainder of the validators for the locked block.
	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), blockID, vs2, vs3, vs4)

	// Check that we relock.
	ensureRelock(t, relockCh, height, round)

	ensurePrecommit(t, voteCh, height, round)

	// We should now be locked on the same block but with an updated locked round.
	validatePrecommit(t, cs1, round, round, vss[0], blockID.Hash, blockID.Hash)
}

// TestStateLock_PrevoteNilWhenLockedAndMissProposal tests that a validator prevotes nil
// if it is locked on a block and misses the proposal in a round.
func TestStateLock_PrevoteNilWhenLockedAndMissProposal(t *testing.T) {
	config := configSetup(t)

	cs1, vss := randState(config, 4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round

	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	pv1, err := cs1.privValidator.GetPubKey(context.Background())
	require.NoError(t, err)
	addr := pv1.Address()
	voteCh := subscribeToVoter(cs1, addr)
	lockCh := subscribe(cs1.eventBus, types.EventQueryLock)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)

	/*
		Round 0:
		cs1 creates a proposal for block B.
		Send a prevote for B from each of the validators to cs1.
		Send a precommit for nil from all of the validators to cs1.

		This ensures that cs1 will lock on B in this round but not precommit it.
	*/
	t.Log("### Starting Round 0")

	startTestRound(cs1, height, round)

	ensureNewRound(t, newRoundCh, height, round)
	ensureNewProposal(t, proposalCh, height, round)
	rs := cs1.GetRoundState()
	blockID := types.BlockID{
		Hash:          rs.ProposalBlock.Hash(),
		PartSetHeader: rs.ProposalBlockParts.Header(),
	}

	ensurePrevote(t, voteCh, height, round)

	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), blockID, vs2, vs3, vs4)

	// check that the validator generates a Lock event.
	ensureLock(t, lockCh, height, round)

	// the proposed block should now be locked and our precommit added.
	ensurePrecommit(t, voteCh, height, round)
	validatePrecommit(t, cs1, round, round, vss[0], blockID.Hash, blockID.Hash)

	// add precommits from the rest of the validators.
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), types.BlockID{}, vs2, vs3, vs4)

	// timeout to new round.
	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Precommit(round).Nanoseconds())

	/*
		Round 1:
		Send a prevote for nil from each of the validators to cs1.
		Send a precommit for nil from all of the validtors to cs1.

		Check that cs1 prevotes nil instead of its locked block, but ensure
		that it maintains its locked block.
	*/
	t.Log("### Starting Round 1")
	incrementRound(vs2, vs3, vs4)
	round++

	ensureNewRound(t, newRoundCh, height, round)

	// Prevote our nil.
	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], nil)

	// Add prevotes from the remainder of the validators nil.
	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), types.BlockID{}, vs2, vs3, vs4)
	ensurePrecommit(t, voteCh, height, round)
	// We should now be locked on the same block but with an updated locked round.
	validatePrecommit(t, cs1, round, 0, vss[0], nil, blockID.Hash)
}

// TestStateLock_PrevoteNilWhenLockedAndMissProposal tests that a validator prevotes nil
// if it is locked on a block and misses the proposal in a round.
func TestStateLock_PrevoteNilWhenLockedAndDifferentProposal(t *testing.T) {
	config := configSetup(t)
	/*
		All of the assertions in this test occur on the `cs1` validator.
		The test sends signed votes from the other validators to cs1 and
		cs1's state is then examined to verify that it now matches the expected
		state.
	*/

	cs1, vss := randState(config, 4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round

	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	pv1, err := cs1.privValidator.GetPubKey(context.Background())
	require.NoError(t, err)
	addr := pv1.Address()
	voteCh := subscribeToVoter(cs1, addr)
	lockCh := subscribe(cs1.eventBus, types.EventQueryLock)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)

	/*
		Round 0:
		cs1 creates a proposal for block B.
		Send a prevote for B from each of the validators to cs1.
		Send a precommit for nil from all of the validators to cs1.

		This ensures that cs1 will lock on B in this round but not precommit it.
	*/
	t.Log("### Starting Round 0")
	startTestRound(cs1, height, round)

	ensureNewRound(t, newRoundCh, height, round)
	ensureNewProposal(t, proposalCh, height, round)
	rs := cs1.GetRoundState()
	blockID := types.BlockID{
		Hash:          rs.ProposalBlock.Hash(),
		PartSetHeader: rs.ProposalBlockParts.Header(),
	}

	ensurePrevote(t, voteCh, height, round)

	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), blockID, vs2, vs3, vs4)

	// check that the validator generates a Lock event.
	ensureLock(t, lockCh, height, round)

	// the proposed block should now be locked and our precommit added.
	ensurePrecommit(t, voteCh, height, round)
	validatePrecommit(t, cs1, round, round, vss[0], blockID.Hash, blockID.Hash)

	// add precommits from the rest of the validators.
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), types.BlockID{}, vs2, vs3, vs4)

	// timeout to new round.
	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Precommit(round).Nanoseconds())

	/*
		Round 1:
		Create a proposal for a new block.
		Send a prevote for nil from each of the validators to cs1.
		Send a precommit for nil from all of the validtors to cs1.

		Check that cs1 prevotes nil instead of its locked block, but ensure
		that it maintains its locked block.
	*/
	t.Log("### Starting Round 1")
	incrementRound(vs2, vs3, vs4)
	round++
	cs2 := newState(cs1.state, vs2, kvstore.NewApplication())
	propR1, propBlockR1 := decideProposal(t, cs2, vs2, vs2.Height, vs2.Round)
	propBlockR1Parts := propBlockR1.MakePartSet(types.BlockPartSizeBytes)
	propBlockR1Hash := propBlockR1.Hash()
	require.NotEqual(t, propBlockR1Hash, blockID.Hash)
	if err := cs1.SetProposalAndBlock(propR1, propBlockR1, propBlockR1Parts, "some peer"); err != nil {
		t.Fatal(err)
	}

	ensureNewRound(t, newRoundCh, height, round)
	ensureNewProposal(t, proposalCh, height, round)

	// Prevote our nil.
	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], nil)

	// Add prevotes from the remainder of the validators for nil.
	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), types.BlockID{}, vs2, vs3, vs4)

	// We should now be locked on the same block but prevote nil.
	ensurePrecommit(t, voteCh, height, round)
	validatePrecommit(t, cs1, round, 0, vss[0], nil, blockID.Hash)
}

// TestStateLock_POLDoesNotUnlock tests that a validator maintains its locked block
// despite receiving +2/3 nil prevotes and nil precommits from other validators.
// Tendermint used to 'unlock' its locked block when greater than 2/3 prevotes
// for a nil block were seen. This behavior has been removed and this test ensures
// that it has been completely removed.
func TestStateLock_POLDoesNotUnlock(t *testing.T) {
	config := configSetup(t)
	/*
		All of the assertions in this test occur on the `cs1` validator.
		The test sends signed votes from the other validators to cs1 and
		cs1's state is then examined to verify that it now matches the expected
		state.
	*/

	cs1, vss := randState(config, 4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round

	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	lockCh := subscribe(cs1.eventBus, types.EventQueryLock)
	pv1, err := cs1.privValidator.GetPubKey(context.Background())
	require.NoError(t, err)
	addr := pv1.Address()
	voteCh := subscribeToVoter(cs1, addr)

	/*
		Round 0:
		Create a block, B
		Send a prevote for B from each of the validators to `cs1`.
		Send a precommit for B from one of the validtors to `cs1`.

		This ensures that cs1 will lock on B in this round.
	*/
	t.Log("#### ONTO ROUND 0")

	// start round and wait for propose and prevote
	startTestRound(cs1, height, round)
	ensureNewRound(t, newRoundCh, height, round)

	ensureNewProposal(t, proposalCh, height, round)
	rs := cs1.GetRoundState()
	blockID := types.BlockID{
		Hash:          rs.ProposalBlock.Hash(),
		PartSetHeader: rs.ProposalBlockParts.Header(),
	}

	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], blockID.Hash)

	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), blockID, vs2, vs3, vs4)

	// the validator should have locked a block in this round.
	ensureLock(t, lockCh, height, round)

	ensurePrecommit(t, voteCh, height, round)
	// the proposed block should now be locked and our should be for this locked block.

	validatePrecommit(t, cs1, round, round, vss[0], blockID.Hash, blockID.Hash)

	// Add precommits from the other validators.
	// We only issue 1/2 Precommits for the block in this round.
	// This ensures that the validator being tested does not commit the block.
	// We do not want the validator to commit the block because we want the test
	// test to proceeds to the next consensus round.
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), types.BlockID{}, vs2, vs4)
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), blockID, vs3)

	// timeout to new round
	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Precommit(round).Nanoseconds())

	/*
		Round 1:
		Send a prevote for nil from >2/3 of the validators to `cs1`.
		Check that cs1 maintains its lock on B but precommits nil.
		Send a precommit for nil from >2/3 of the validators to `cs1`.
	*/
	t.Log("#### ONTO ROUND 1")
	round++
	incrementRound(vs2, vs3, vs4)
	cs2 := newState(cs1.state, vs2, kvstore.NewApplication())
	prop, propBlock := decideProposal(t, cs2, vs2, vs2.Height, vs2.Round)
	propBlockParts := propBlock.MakePartSet(types.BlockPartSizeBytes)
	require.NotEqual(t, propBlock.Hash(), blockID.Hash)
	if err := cs1.SetProposalAndBlock(prop, propBlock, propBlockParts, ""); err != nil {
		t.Fatal(err)
	}

	ensureNewRound(t, newRoundCh, height, round)

	ensureNewProposal(t, proposalCh, height, round)

	// Prevote for nil since the proposed block does not match our locked block.
	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], nil)

	// add >2/3 prevotes for nil from all other validators
	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), types.BlockID{}, vs2, vs3, vs4)

	ensurePrecommit(t, voteCh, height, round)

	// verify that we haven't update our locked block since the first round
	validatePrecommit(t, cs1, round, 0, vss[0], nil, blockID.Hash)

	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), types.BlockID{}, vs2, vs3, vs4)
	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Precommit(round).Nanoseconds())

	/*
		Round 2:
		The validator cs1 saw >2/3 precommits for nil in the previous round.
		Send the validator >2/3 prevotes for nil and ensure that it did not
		unlock its block at the end of the previous round.
	*/
	t.Log("#### ONTO ROUND 2")
	round++
	incrementRound(vs2, vs3, vs4)
	cs3 := newState(cs1.state, vs2, kvstore.NewApplication())
	prop, propBlock = decideProposal(t, cs3, vs3, vs3.Height, vs3.Round)
	propBlockParts = propBlock.MakePartSet(types.BlockPartSizeBytes)
	if err := cs1.SetProposalAndBlock(prop, propBlock, propBlockParts, ""); err != nil {
		t.Fatal(err)
	}

	ensureNewRound(t, newRoundCh, height, round)

	ensureNewProposal(t, proposalCh, height, round)

	// Prevote for nil since the proposal does not match our locked block.
	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], nil)

	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), types.BlockID{}, vs2, vs3, vs4)

	ensurePrecommit(t, voteCh, height, round)

	// verify that we haven't update our locked block since the first round
	validatePrecommit(t, cs1, round, 0, vss[0], nil, blockID.Hash)

}

// TestStateLock_MissingProposalWhenPOLSeenDoesNotUnlock tests that observing
// a two thirds majority for a block does not cause a validator to upate its lock on the
// new block if a proposal was not seen for that block.
func TestStateLock_MissingProposalWhenPOLSeenDoesNotUpdateLock(t *testing.T) {
	config := configSetup(t)

	cs1, vss := randState(config, 4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round

	partSize := types.BlockPartSizeBytes

	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	pv1, err := cs1.privValidator.GetPubKey(context.Background())
	require.NoError(t, err)
	addr := pv1.Address()
	voteCh := subscribeToVoter(cs1, addr)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	/*
		Round 0:
		cs1 creates a proposal for block B.
		Send a prevote for B from each of the validators to cs1.
		Send a precommit for nil from all of the validators to cs1.

		This ensures that cs1 will lock on B in this round but not precommit it.
	*/
	t.Log("### Starting Round 0")
	startTestRound(cs1, height, round)

	ensureNewRound(t, newRoundCh, height, round)
	ensureNewProposal(t, proposalCh, height, round)
	rs := cs1.GetRoundState()
	firstBlockID := types.BlockID{
		Hash:          rs.ProposalBlock.Hash(),
		PartSetHeader: rs.ProposalBlockParts.Header(),
	}

	ensurePrevote(t, voteCh, height, round) // prevote

	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), firstBlockID, vs2, vs3, vs4)

	ensurePrecommit(t, voteCh, height, round) // our precommit
	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, round, round, vss[0], firstBlockID.Hash, firstBlockID.Hash)

	// add precommits from the rest
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), types.BlockID{}, vs2, vs3, vs4)

	// timeout to new round
	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Precommit(round).Nanoseconds())

	/*
		Round 1:
		Create a new block, D but do not send it to cs1.
		Send a prevote for D from each of the validators to cs1.

		Check that cs1 does not update its locked block to this missed block D.
	*/
	t.Log("### Starting Round 1")
	incrementRound(vs2, vs3, vs4)
	round++
	cs2 := newState(cs1.state, vs2, kvstore.NewApplication())
	prop, propBlock := decideProposal(t, cs2, vs2, vs2.Height, vs2.Round)
	if prop == nil || propBlock == nil {
		t.Fatal("Failed to create proposal block with vs2")
	}
	secondBlockID := types.BlockID{
		Hash:          propBlock.Hash(),
		PartSetHeader: propBlock.MakePartSet(partSize).Header(),
	}
	require.NotEqual(t, secondBlockID.Hash, firstBlockID.Hash)

	ensureNewRound(t, newRoundCh, height, round)

	// prevote for nil since the proposal was not seen.
	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], nil)

	// now lets add prevotes from everyone else for the new block
	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), secondBlockID, vs2, vs3, vs4)

	ensurePrecommit(t, voteCh, height, round)
	validatePrecommit(t, cs1, round, 0, vss[0], nil, firstBlockID.Hash)
}

// TestStateLock_DoesNotLockOnOldProposal tests that observing
// a two thirds majority for a block does not cause a validator to lock on the
// block if a proposal was not seen for that block in the current round, but
// was seen in a previous round.
func TestStateLock_DoesNotLockOnOldProposal(t *testing.T) {
	config := configSetup(t)

	cs1, vss := randState(config, 4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round

	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	pv1, err := cs1.privValidator.GetPubKey(context.Background())
	require.NoError(t, err)
	addr := pv1.Address()
	voteCh := subscribeToVoter(cs1, addr)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	/*
		Round 0:
		cs1 creates a proposal for block B.
		Send a prevote for nil from each of the validators to cs1.
		Send a precommit for nil from all of the validators to cs1.

		This ensures that cs1 will not lock on B.
	*/
	t.Log("### Starting Round 0")
	startTestRound(cs1, height, round)

	ensureNewRound(t, newRoundCh, height, round)
	ensureNewProposal(t, proposalCh, height, round)
	rs := cs1.GetRoundState()
	firstBlockID := types.BlockID{
		Hash:          rs.ProposalBlock.Hash(),
		PartSetHeader: rs.ProposalBlockParts.Header(),
	}

	ensurePrevote(t, voteCh, height, round)

	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), types.BlockID{}, vs2, vs3, vs4)

	// The proposed block should not have been locked.
	ensurePrecommit(t, voteCh, height, round)
	validatePrecommit(t, cs1, round, -1, vss[0], nil, nil)

	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), types.BlockID{}, vs2, vs3, vs4)

	incrementRound(vs2, vs3, vs4)

	// timeout to new round
	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Precommit(round).Nanoseconds())

	/*
		Round 1:
		No proposal new proposal is created.
		Send a prevote for B, the block from round 0, from each of the validators to cs1.
		Send a precommit for nil from all of the validators to cs1.

		cs1 saw a POL for the block it saw in round 0. We ensure that it does not
		lock on this block, since it did not see a proposal for it in this round.
	*/
	t.Log("### Starting Round 1")
	round++
	ensureNewRound(t, newRoundCh, height, round)

	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], nil)

	// All validators prevote for the old block.
	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), firstBlockID, vs2, vs3, vs4)

	// Make sure that cs1 did not lock on the block since it did not receive a proposal for it.
	ensurePrecommit(t, voteCh, height, round)
	validatePrecommit(t, cs1, round, -1, vss[0], nil, nil)
}

// 4 vals
// a polka at round 1 but we miss it
// then a polka at round 2 that we lock on
// then we see the polka from round 1 but shouldn't unlock
func TestStateLock_POLSafety1(t *testing.T) {
	config := configSetup(t)

	cs1, vss := randState(config, 4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round

	partSize := types.BlockPartSizeBytes

	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	timeoutProposeCh := subscribe(cs1.eventBus, types.EventQueryTimeoutPropose)
	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	pv1, err := cs1.privValidator.GetPubKey(context.Background())
	require.NoError(t, err)
	addr := pv1.Address()
	voteCh := subscribeToVoter(cs1, addr)

	// start round and wait for propose and prevote
	startTestRound(cs1, cs1.Height, round)
	ensureNewRound(t, newRoundCh, height, round)

	ensureNewProposal(t, proposalCh, height, round)
	rs := cs1.GetRoundState()
	propBlock := rs.ProposalBlock

	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], propBlock.Hash())
	blockID := types.BlockID{Hash: propBlock.Hash(), PartSetHeader: propBlock.MakePartSet(partSize).Header()}
	// the others sign a polka but we don't see it
	prevotes := signVotes(tmproto.PrevoteType, config.ChainID(),
		blockID,
		vs2, vs3, vs4)

	// we do see them precommit nil
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), types.BlockID{}, vs2, vs3, vs4)

	// cs1 precommit nil
	ensurePrecommit(t, voteCh, height, round)
	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Precommit(round).Nanoseconds())

	t.Log("### ONTO ROUND 1")
	incrementRound(vs2, vs3, vs4)
	round++ // moving to the next round
	cs2 := newState(cs1.state, vs2, kvstore.NewApplication())
	prop, propBlock := decideProposal(t, cs2, vs2, vs2.Height, vs2.Round)
	propBlockParts := propBlock.MakePartSet(partSize)
	r2BlockID := types.BlockID{
		Hash:          propBlock.Hash(),
		PartSetHeader: propBlockParts.Header(),
	}

	ensureNewRound(t, newRoundCh, height, round)

	//XXX: this isnt guaranteed to get there before the timeoutPropose ...
	if err := cs1.SetProposalAndBlock(prop, propBlock, propBlockParts, "some peer"); err != nil {
		t.Fatal(err)
	}
	/*Round2
	// we timeout and prevote our lock
	// a polka happened but we didn't see it!
	*/

	ensureNewProposal(t, proposalCh, height, round)

	rs = cs1.GetRoundState()

	if rs.LockedBlock != nil {
		t.Fatalf("was not expected to be locked on a block")
	}

	// go to prevote, prevote for proposal block
	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], r2BlockID.Hash)

	// now we see the others prevote for it, so we should lock on it
	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), r2BlockID, vs2, vs3, vs4)

	ensurePrecommit(t, voteCh, height, round)
	// we should have precommitted
	validatePrecommit(t, cs1, round, round, vss[0], r2BlockID.Hash, r2BlockID.Hash)

	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), types.BlockID{}, vs2, vs3, vs4)

	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Precommit(round).Nanoseconds())

	incrementRound(vs2, vs3, vs4)
	round++ // moving to the next round

	ensureNewRound(t, newRoundCh, height, round)

	t.Log("### ONTO ROUND 2")
	/*Round3
	we see the polka from round 1 but we shouldn't unlock!
	*/

	// timeout of propose
	ensureNewTimeout(t, timeoutProposeCh, height, round, cs1.config.Propose(round).Nanoseconds())

	// finish prevote
	ensurePrevote(t, voteCh, height, round)
	// we should prevote for nil
	validatePrevote(t, cs1, round, vss[0], nil)

	newStepCh := subscribe(cs1.eventBus, types.EventQueryNewRoundStep)

	// before prevotes from the previous round are added
	// add prevotes from the earlier round
	addVotes(cs1, prevotes...)

	ensureNoNewRoundStep(t, newStepCh)
}

// 4 vals.
// polka P0 at R0, P1 at R1, and P2 at R2,
// we lock on P0 at R0, don't see P1, and unlock using P2 at R2
// then we should make sure we don't lock using P1

// What we want:
// dont see P0, lock on P1 at R1, dont unlock using P0 at R2
func TestStateLock_POLSafety2(t *testing.T) {
	config := configSetup(t)

	cs1, vss := randState(config, 4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round

	partSize := types.BlockPartSizeBytes

	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	pv1, err := cs1.privValidator.GetPubKey(context.Background())
	require.NoError(t, err)
	addr := pv1.Address()
	voteCh := subscribeToVoter(cs1, addr)

	// the block for R0: gets polkad but we miss it
	// (even though we signed it, shhh)
	_, propBlock0 := decideProposal(t, cs1, vss[0], height, round)
	propBlockHash0 := propBlock0.Hash()
	propBlockParts0 := propBlock0.MakePartSet(partSize)
	propBlockID0 := types.BlockID{Hash: propBlockHash0, PartSetHeader: propBlockParts0.Header()}

	// the others sign a polka but we don't see it
	prevotes := signVotes(tmproto.PrevoteType, config.ChainID(), propBlockID0, vs2, vs3, vs4)

	// the block for round 1
	prop1, propBlock1 := decideProposal(t, cs1, vs2, vs2.Height, vs2.Round+1)
	propBlockParts1 := propBlock1.MakePartSet(partSize)
	propBlockID1 := types.BlockID{Hash: propBlock1.Hash(), PartSetHeader: propBlockParts1.Header()}

	incrementRound(vs2, vs3, vs4)

	round++ // moving to the next round
	t.Log("### ONTO Round 1")
	// jump in at round 1
	startTestRound(cs1, height, round)
	ensureNewRound(t, newRoundCh, height, round)

	if err := cs1.SetProposalAndBlock(prop1, propBlock1, propBlockParts1, "some peer"); err != nil {
		t.Fatal(err)
	}
	ensureNewProposal(t, proposalCh, height, round)

	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], propBlockID1.Hash)

	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), propBlockID1, vs2, vs3, vs4)

	ensurePrecommit(t, voteCh, height, round)
	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, round, round, vss[0], propBlockID1.Hash, propBlockID1.Hash)

	// add precommits from the rest
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), types.BlockID{}, vs2, vs4)
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), propBlockID1, vs3)

	incrementRound(vs2, vs3, vs4)

	// timeout of precommit wait to new round
	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Precommit(round).Nanoseconds())

	round++ // moving to the next round
	// in round 2 we see the polkad block from round 0
	newProp := types.NewProposal(height, round, 0, propBlockID0)
	p := newProp.ToProto()
	if err := vs3.SignProposal(context.Background(), config.ChainID(), p); err != nil {
		t.Fatal(err)
	}

	newProp.Signature = p.Signature

	if err := cs1.SetProposalAndBlock(newProp, propBlock0, propBlockParts0, "some peer"); err != nil {
		t.Fatal(err)
	}

	// Add the pol votes
	addVotes(cs1, prevotes...)

	ensureNewRound(t, newRoundCh, height, round)
	t.Log("### ONTO Round 2")
	/*Round2
	// now we see the polka from round 1, but we shouldnt unlock
	*/
	ensureNewProposal(t, proposalCh, height, round)

	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], propBlockID1.Hash)

}

// TestState_PrevotePOLFromPreviousRound tests that a validator will prevote
// for a block if it is locked on a different block but saw a POL for the block
// it is not locked on in a previous round.
func TestState_PrevotePOLFromPreviousRound(t *testing.T) {
	config := configSetup(t)

	cs1, vss := randState(config, 4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round

	partSize := types.BlockPartSizeBytes

	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	pv1, err := cs1.privValidator.GetPubKey(context.Background())
	require.NoError(t, err)
	addr := pv1.Address()
	voteCh := subscribeToVoter(cs1, addr)
	lockCh := subscribe(cs1.eventBus, types.EventQueryLock)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)

	/*
		Round 0:
		cs1 creates a proposal for block B.
		Send a prevote for B from each of the validators to cs1.
		Send a precommit for nil from all of the validators to cs1.

		This ensures that cs1 will lock on B in this round but not precommit it.
	*/
	t.Log("### Starting Round 0")

	startTestRound(cs1, height, round)

	ensureNewRound(t, newRoundCh, height, round)
	ensureNewProposal(t, proposalCh, height, round)
	rs := cs1.GetRoundState()
	r0BlockID := types.BlockID{
		Hash:          rs.ProposalBlock.Hash(),
		PartSetHeader: rs.ProposalBlockParts.Header(),
	}

	ensurePrevote(t, voteCh, height, round)

	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), r0BlockID, vs2, vs3, vs4)

	// check that the validator generates a Lock event.
	ensureLock(t, lockCh, height, round)

	// the proposed block should now be locked and our precommit added.
	ensurePrecommit(t, voteCh, height, round)
	validatePrecommit(t, cs1, round, round, vss[0], r0BlockID.Hash, r0BlockID.Hash)

	// add precommits from the rest of the validators.
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), types.BlockID{}, vs2, vs3, vs4)

	// timeout to new round.
	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Precommit(round).Nanoseconds())

	/*
		Round 1:
		Create a block, D but do not send a proposal for it to cs1.
		Send a prevote for D from each of the validators to cs1 so that cs1 sees a POL.
		Send a precommit for nil from all of the validtors to cs1.

		cs1 has now seen greater than 2/3 of the voting power prevote D in this round
		but cs1 did not see the proposal for D in this round so it will not prevote or precommit it.
	*/
	t.Log("### Starting Round 1")
	incrementRound(vs2, vs3, vs4)
	round++
	// Generate a new proposal block.
	cs2 := newState(cs1.state, vs2, kvstore.NewApplication())
	cs2.ValidRound = 1
	propR1, propBlockR1 := decideProposal(t, cs2, vs2, vs2.Height, round)
	t.Log(propR1.POLRound)
	propBlockR1Parts := propBlockR1.MakePartSet(partSize)
	r1BlockID := types.BlockID{
		Hash:          propBlockR1.Hash(),
		PartSetHeader: propBlockR1Parts.Header(),
	}
	require.NotEqual(t, r1BlockID.Hash, r0BlockID.Hash)

	ensureNewRound(t, newRoundCh, height, round)

	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), r1BlockID, vs2, vs3, vs4)

	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], nil)

	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), types.BlockID{}, vs2, vs3, vs4)

	ensurePrecommit(t, voteCh, height, round)

	// timeout to new round.
	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Precommit(round).Nanoseconds())

	/*
		Create a new proposal for D, the same block from Round 1.
		cs1 already saw greater than 2/3 of the voting power on the network vote for
		D in a previous round, so it should prevote D once it receives a proposal for it.

		cs1 does not need to receive prevotes from other validators before the proposal
		in this round. It will still prevote the block.

		Send cs1 prevotes for nil and check that it still prevotes its locked block
		and not the block that it prevoted.
	*/
	t.Log("### Starting Round 2")
	incrementRound(vs2, vs3, vs4)
	round++
	propR2 := types.NewProposal(height, round, 1, r1BlockID)
	p := propR2.ToProto()
	if err := vs3.SignProposal(context.Background(), cs1.state.ChainID, p); err != nil {
		t.Fatalf("error signing proposal: %s", err)
	}
	propR2.Signature = p.Signature

	// cs1 receives a proposal for D, the block that received a POL in round 1.
	if err := cs1.SetProposalAndBlock(propR2, propBlockR1, propBlockR1Parts, ""); err != nil {
		t.Fatal(err)
	}

	ensureNewRound(t, newRoundCh, height, round)

	ensureNewProposal(t, proposalCh, height, round)

	// We should now prevote this block, despite being locked on the block from
	// round 0.
	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], r1BlockID.Hash)

	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), types.BlockID{}, vs2, vs3, vs4)

	// cs1 did not receive a POL within this round, so it should remain locked
	// on the block from round 0.
	ensurePrecommit(t, voteCh, height, round)
	validatePrecommit(t, cs1, round, 0, vss[0], nil, r0BlockID.Hash)
}

// 4 vals.
// polka P0 at R0 for B0. We lock B0 on P0 at R0.

// What we want:
// P0 proposes B0 at R3.
func TestProposeValidBlock(t *testing.T) {
	config := configSetup(t)

	cs1, vss := randState(config, 4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round

	partSize := types.BlockPartSizeBytes

	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	timeoutProposeCh := subscribe(cs1.eventBus, types.EventQueryTimeoutPropose)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	pv1, err := cs1.privValidator.GetPubKey(context.Background())
	require.NoError(t, err)
	addr := pv1.Address()
	voteCh := subscribeToVoter(cs1, addr)

	// start round and wait for propose and prevote
	startTestRound(cs1, cs1.Height, round)
	ensureNewRound(t, newRoundCh, height, round)

	ensureNewProposal(t, proposalCh, height, round)
	rs := cs1.GetRoundState()
	propBlock := rs.ProposalBlock
	blockID := types.BlockID{
		Hash:          propBlock.Hash(),
		PartSetHeader: propBlock.MakePartSet(partSize).Header(),
	}

	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], blockID.Hash)

	// the others sign a polka
	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), blockID, vs2, vs3, vs4)

	ensurePrecommit(t, voteCh, height, round)
	// we should have precommitted the proposed block in this round.

	validatePrecommit(t, cs1, round, round, vss[0], blockID.Hash, blockID.Hash)

	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), types.BlockID{}, vs2, vs3, vs4)

	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Precommit(round).Nanoseconds())

	incrementRound(vs2, vs3, vs4)
	round++ // moving to the next round

	ensureNewRound(t, newRoundCh, height, round)
	t.Log("### ONTO ROUND 1")

	// timeout of propose
	ensureNewTimeout(t, timeoutProposeCh, height, round, cs1.config.Propose(round).Nanoseconds())

	// We did not see a valid proposal within this round, so prevote nil.
	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], nil)

	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), types.BlockID{}, vs2, vs3, vs4)

	ensurePrecommit(t, voteCh, height, round)
	// we should have precommitted nil during this round because we received
	// >2/3 precommits for nil from the other validators.
	validatePrecommit(t, cs1, round, 0, vss[0], nil, blockID.Hash)

	incrementRound(vs2, vs3, vs4)
	incrementRound(vs2, vs3, vs4)

	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), types.BlockID{}, vs2, vs3, vs4)

	round += 2 // increment by multiple rounds

	ensureNewRound(t, newRoundCh, height, round)
	t.Log("### ONTO ROUND 3")

	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Precommit(round).Nanoseconds())

	round++ // moving to the next round

	ensureNewRound(t, newRoundCh, height, round)

	ensureNewProposal(t, proposalCh, height, round)

	rs = cs1.GetRoundState()
	assert.True(t, bytes.Equal(rs.ProposalBlock.Hash(), blockID.Hash))
	assert.True(t, bytes.Equal(rs.ProposalBlock.Hash(), rs.ValidBlock.Hash()))
	assert.True(t, rs.Proposal.POLRound == rs.ValidRound)
	assert.True(t, bytes.Equal(rs.Proposal.BlockID.Hash, rs.ValidBlock.Hash()))
}

// What we want:
// P0 miss to lock B but set valid block to B after receiving delayed prevote.
func TestSetValidBlockOnDelayedPrevote(t *testing.T) {
	config := configSetup(t)

	cs1, vss := randState(config, 4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round

	partSize := types.BlockPartSizeBytes

	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	validBlockCh := subscribe(cs1.eventBus, types.EventQueryValidBlock)
	pv1, err := cs1.privValidator.GetPubKey(context.Background())
	require.NoError(t, err)
	addr := pv1.Address()
	voteCh := subscribeToVoter(cs1, addr)

	// start round and wait for propose and prevote
	startTestRound(cs1, cs1.Height, round)
	ensureNewRound(t, newRoundCh, height, round)

	ensureNewProposal(t, proposalCh, height, round)
	rs := cs1.GetRoundState()
	propBlock := rs.ProposalBlock
	blockID := types.BlockID{
		Hash:          propBlock.Hash(),
		PartSetHeader: propBlock.MakePartSet(partSize).Header(),
	}

	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], blockID.Hash)

	// vs2 send prevote for propBlock
	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), blockID, vs2)

	// vs3 send prevote nil
	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), types.BlockID{}, vs3)

	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Prevote(round).Nanoseconds())

	ensurePrecommit(t, voteCh, height, round)
	// we should have precommitted
	validatePrecommit(t, cs1, round, -1, vss[0], nil, nil)

	rs = cs1.GetRoundState()

	assert.True(t, rs.ValidBlock == nil)
	assert.True(t, rs.ValidBlockParts == nil)
	assert.True(t, rs.ValidRound == -1)

	// vs2 send (delayed) prevote for propBlock
	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), blockID, vs4)

	ensureNewValidBlock(t, validBlockCh, height, round)

	rs = cs1.GetRoundState()

	assert.True(t, bytes.Equal(rs.ValidBlock.Hash(), blockID.Hash))
	assert.True(t, rs.ValidBlockParts.Header().Equals(blockID.PartSetHeader))
	assert.True(t, rs.ValidRound == round)
}

// What we want:
// P0 miss to lock B as Proposal Block is missing, but set valid block to B after
// receiving delayed Block Proposal.
func TestSetValidBlockOnDelayedProposal(t *testing.T) {
	config := configSetup(t)

	cs1, vss := randState(config, 4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round

	partSize := types.BlockPartSizeBytes

	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	timeoutProposeCh := subscribe(cs1.eventBus, types.EventQueryTimeoutPropose)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	validBlockCh := subscribe(cs1.eventBus, types.EventQueryValidBlock)
	pv1, err := cs1.privValidator.GetPubKey(context.Background())
	require.NoError(t, err)
	addr := pv1.Address()
	voteCh := subscribeToVoter(cs1, addr)
	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)

	round++ // move to round in which P0 is not proposer
	incrementRound(vs2, vs3, vs4)

	startTestRound(cs1, cs1.Height, round)
	ensureNewRound(t, newRoundCh, height, round)

	ensureNewTimeout(t, timeoutProposeCh, height, round, cs1.config.Propose(round).Nanoseconds())

	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], nil)

	prop, propBlock := decideProposal(t, cs1, vs2, vs2.Height, vs2.Round+1)
	blockID := types.BlockID{
		Hash:          propBlock.Hash(),
		PartSetHeader: propBlock.MakePartSet(partSize).Header(),
	}

	// vs2, vs3 and vs4 send prevote for propBlock
	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), blockID, vs2, vs3, vs4)
	ensureNewValidBlock(t, validBlockCh, height, round)

	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Prevote(round).Nanoseconds())

	ensurePrecommit(t, voteCh, height, round)
	validatePrecommit(t, cs1, round, -1, vss[0], nil, nil)

	if err := cs1.SetProposalAndBlock(prop, propBlock, propBlock.MakePartSet(partSize), "some peer"); err != nil {
		t.Fatal(err)
	}

	ensureNewProposal(t, proposalCh, height, round)
	rs := cs1.GetRoundState()

	assert.True(t, bytes.Equal(rs.ValidBlock.Hash(), blockID.Hash))
	assert.True(t, rs.ValidBlockParts.Header().Equals(blockID.PartSetHeader))
	assert.True(t, rs.ValidRound == round)
}

// 4 vals, 3 Nil Precommits at P0
// What we want:
// P0 waits for timeoutPrecommit before starting next round
func TestWaitingTimeoutOnNilPolka(t *testing.T) {
	config := configSetup(t)

	cs1, vss := randState(config, 4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round

	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)

	// start round
	startTestRound(cs1, height, round)
	ensureNewRound(t, newRoundCh, height, round)

	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), types.BlockID{}, vs2, vs3, vs4)

	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Precommit(round).Nanoseconds())
	ensureNewRound(t, newRoundCh, height, round+1)
}

// 4 vals, 3 Prevotes for nil from the higher round.
// What we want:
// P0 waits for timeoutPropose in the next round before entering prevote
func TestWaitingTimeoutProposeOnNewRound(t *testing.T) {
	config := configSetup(t)

	cs1, vss := randState(config, 4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round

	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutPropose)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	pv1, err := cs1.privValidator.GetPubKey(context.Background())
	require.NoError(t, err)
	addr := pv1.Address()
	voteCh := subscribeToVoter(cs1, addr)

	// start round
	startTestRound(cs1, height, round)
	ensureNewRound(t, newRoundCh, height, round)

	ensurePrevote(t, voteCh, height, round)

	incrementRound(vss[1:]...)
	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), types.BlockID{}, vs2, vs3, vs4)

	round++ // moving to the next round
	ensureNewRound(t, newRoundCh, height, round)

	rs := cs1.GetRoundState()
	assert.True(t, rs.Step == cstypes.RoundStepPropose) // P0 does not prevote before timeoutPropose expires

	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Propose(round).Nanoseconds())

	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], nil)
}

// 4 vals, 3 Precommits for nil from the higher round.
// What we want:
// P0 jump to higher round, precommit and start precommit wait
func TestRoundSkipOnNilPolkaFromHigherRound(t *testing.T) {
	config := configSetup(t)

	cs1, vss := randState(config, 4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round

	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	pv1, err := cs1.privValidator.GetPubKey(context.Background())
	require.NoError(t, err)
	addr := pv1.Address()
	voteCh := subscribeToVoter(cs1, addr)

	// start round
	startTestRound(cs1, height, round)
	ensureNewRound(t, newRoundCh, height, round)

	ensurePrevote(t, voteCh, height, round)

	incrementRound(vss[1:]...)
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), types.BlockID{}, vs2, vs3, vs4)

	round++ // moving to the next round
	ensureNewRound(t, newRoundCh, height, round)

	ensurePrecommit(t, voteCh, height, round)
	validatePrecommit(t, cs1, round, -1, vss[0], nil, nil)

	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Precommit(round).Nanoseconds())

	round++ // moving to the next round
	ensureNewRound(t, newRoundCh, height, round)
}

// 4 vals, 3 Prevotes for nil in the current round.
// What we want:
// P0 wait for timeoutPropose to expire before sending prevote.
func TestWaitTimeoutProposeOnNilPolkaForTheCurrentRound(t *testing.T) {
	config := configSetup(t)

	cs1, vss := randState(config, 4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, int32(1)

	timeoutProposeCh := subscribe(cs1.eventBus, types.EventQueryTimeoutPropose)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	pv1, err := cs1.privValidator.GetPubKey(context.Background())
	require.NoError(t, err)
	addr := pv1.Address()
	voteCh := subscribeToVoter(cs1, addr)

	// start round in which PO is not proposer
	startTestRound(cs1, height, round)
	ensureNewRound(t, newRoundCh, height, round)

	incrementRound(vss[1:]...)
	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), types.BlockID{}, vs2, vs3, vs4)

	ensureNewTimeout(t, timeoutProposeCh, height, round, cs1.config.Propose(round).Nanoseconds())

	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], nil)
}

// What we want:
// P0 emit NewValidBlock event upon receiving 2/3+ Precommit for B but hasn't received block B yet
func TestEmitNewValidBlockEventOnCommitWithoutBlock(t *testing.T) {
	config := configSetup(t)

	cs1, vss := randState(config, 4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, int32(1)

	incrementRound(vs2, vs3, vs4)

	partSize := types.BlockPartSizeBytes

	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	validBlockCh := subscribe(cs1.eventBus, types.EventQueryValidBlock)

	_, propBlock := decideProposal(t, cs1, vs2, vs2.Height, vs2.Round)
	blockID := types.BlockID{
		Hash:          propBlock.Hash(),
		PartSetHeader: propBlock.MakePartSet(partSize).Header(),
	}

	// start round in which PO is not proposer
	startTestRound(cs1, height, round)
	ensureNewRound(t, newRoundCh, height, round)

	// vs2, vs3 and vs4 send precommit for propBlock
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), blockID, vs2, vs3, vs4)
	ensureNewValidBlock(t, validBlockCh, height, round)

	rs := cs1.GetRoundState()
	assert.True(t, rs.Step == cstypes.RoundStepCommit)
	assert.True(t, rs.ProposalBlock == nil)
	assert.True(t, rs.ProposalBlockParts.Header().Equals(blockID.PartSetHeader))

}

// What we want:
// P0 receives 2/3+ Precommit for B for round 0, while being in round 1. It emits NewValidBlock event.
// After receiving block, it executes block and moves to the next height.
func TestCommitFromPreviousRound(t *testing.T) {
	config := configSetup(t)

	cs1, vss := randState(config, 4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, int32(1)

	partSize := types.BlockPartSizeBytes

	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	validBlockCh := subscribe(cs1.eventBus, types.EventQueryValidBlock)
	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)

	prop, propBlock := decideProposal(t, cs1, vs2, vs2.Height, vs2.Round)
	blockID := types.BlockID{
		Hash:          propBlock.Hash(),
		PartSetHeader: propBlock.MakePartSet(partSize).Header(),
	}

	// start round in which PO is not proposer
	startTestRound(cs1, height, round)
	ensureNewRound(t, newRoundCh, height, round)

	// vs2, vs3 and vs4 send precommit for propBlock for the previous round
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), blockID, vs2, vs3, vs4)

	ensureNewValidBlock(t, validBlockCh, height, round)

	rs := cs1.GetRoundState()
	assert.True(t, rs.Step == cstypes.RoundStepCommit)
	assert.True(t, rs.CommitRound == vs2.Round)
	assert.True(t, rs.ProposalBlock == nil)
	assert.True(t, rs.ProposalBlockParts.Header().Equals(blockID.PartSetHeader))

	if err := cs1.SetProposalAndBlock(prop, propBlock, propBlock.MakePartSet(partSize), "some peer"); err != nil {
		t.Fatal(err)
	}

	ensureNewProposal(t, proposalCh, height, round)
	ensureNewRound(t, newRoundCh, height+1, 0)
}

type fakeTxNotifier struct {
	ch chan struct{}
}

func (n *fakeTxNotifier) TxsAvailable() <-chan struct{} {
	return n.ch
}

func (n *fakeTxNotifier) Notify() {
	n.ch <- struct{}{}
}

// 2 vals precommit votes for a block but node times out waiting for the third. Move to next round
// and third precommit arrives which leads to the commit of that header and the correct
// start of the next round
func TestStartNextHeightCorrectlyAfterTimeout(t *testing.T) {
	config := configSetup(t)

	config.Consensus.SkipTimeoutCommit = false
	cs1, vss := randState(config, 4)
	cs1.txNotifier = &fakeTxNotifier{ch: make(chan struct{})}

	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round

	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	timeoutProposeCh := subscribe(cs1.eventBus, types.EventQueryTimeoutPropose)
	precommitTimeoutCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)

	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	newBlockHeader := subscribe(cs1.eventBus, types.EventQueryNewBlockHeader)
	pv1, err := cs1.privValidator.GetPubKey(context.Background())
	require.NoError(t, err)
	addr := pv1.Address()
	voteCh := subscribeToVoter(cs1, addr)

	// start round and wait for propose and prevote
	startTestRound(cs1, height, round)
	ensureNewRound(t, newRoundCh, height, round)

	ensureNewProposal(t, proposalCh, height, round)
	rs := cs1.GetRoundState()
	blockID := types.BlockID{
		Hash:          rs.ProposalBlock.Hash(),
		PartSetHeader: rs.ProposalBlockParts.Header(),
	}

	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], blockID.Hash)

	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), blockID, vs2, vs3, vs4)

	ensurePrecommit(t, voteCh, height, round)
	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, round, round, vss[0], blockID.Hash, blockID.Hash)

	// add precommits
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), types.BlockID{}, vs2)
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), blockID, vs3)

	// wait till timeout occurs
	ensurePrecommitTimeout(t, precommitTimeoutCh)

	ensureNewRound(t, newRoundCh, height, round+1)

	// majority is now reached
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), blockID, vs4)

	ensureNewBlockHeader(t, newBlockHeader, height, blockID.Hash)

	cs1.txNotifier.(*fakeTxNotifier).Notify()

	ensureNewTimeout(t, timeoutProposeCh, height+1, round, cs1.config.Propose(round).Nanoseconds())
	rs = cs1.GetRoundState()
	assert.False(
		t,
		rs.TriggeredTimeoutPrecommit,
		"triggeredTimeoutPrecommit should be false at the beginning of each round")
}

func TestResetTimeoutPrecommitUponNewHeight(t *testing.T) {
	config := configSetup(t)

	config.Consensus.SkipTimeoutCommit = false
	cs1, vss := randState(config, 4)

	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round

	partSize := types.BlockPartSizeBytes

	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)

	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	newBlockHeader := subscribe(cs1.eventBus, types.EventQueryNewBlockHeader)
	pv1, err := cs1.privValidator.GetPubKey(context.Background())
	require.NoError(t, err)
	addr := pv1.Address()
	voteCh := subscribeToVoter(cs1, addr)

	// start round and wait for propose and prevote
	startTestRound(cs1, height, round)
	ensureNewRound(t, newRoundCh, height, round)

	ensureNewProposal(t, proposalCh, height, round)
	rs := cs1.GetRoundState()
	blockID := types.BlockID{
		Hash:          rs.ProposalBlock.Hash(),
		PartSetHeader: rs.ProposalBlockParts.Header(),
	}

	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], blockID.Hash)

	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), blockID, vs2, vs3, vs4)

	ensurePrecommit(t, voteCh, height, round)
	validatePrecommit(t, cs1, round, round, vss[0], blockID.Hash, blockID.Hash)

	// add precommits
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), types.BlockID{}, vs2)
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), blockID, vs3)
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), blockID, vs4)

	ensureNewBlockHeader(t, newBlockHeader, height, blockID.Hash)

	prop, propBlock := decideProposal(t, cs1, vs2, height+1, 0)
	propBlockParts := propBlock.MakePartSet(partSize)

	if err := cs1.SetProposalAndBlock(prop, propBlock, propBlockParts, "some peer"); err != nil {
		t.Fatal(err)
	}
	ensureNewProposal(t, proposalCh, height+1, 0)

	rs = cs1.GetRoundState()
	assert.False(
		t,
		rs.TriggeredTimeoutPrecommit,
		"triggeredTimeoutPrecommit should be false at the beginning of each height")
}

//------------------------------------------------------------------------------------------
// SlashingSuite
// TODO: Slashing

/*
func TestStateSlashing_Prevotes(t *testing.T) {
	cs1, vss := randState(2)
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
	signAddVotes(tmproto.PrevoteType, hash,config.ChainID(), blockID, rs.ProposalBlock.Hash(),config.ChainID(), rs.ProposalBlockParts.Header(), vs2)

	// XXX: Check for existence of Dupeout info
}

func TestStateSlashing_Precommits(t *testing.T) {
	cs1, vss := randState(2)
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
	signAddVotes(tmproto.PrevoteType, rs.ProposalBlock.Hash(),config.ChainID(), blockID, waiting for more prevotes
	// add one for a different block should cause us to go into prevote wait
	hash := rs.ProposalBlock.Hash()
	hash[0] = byte(hash[0]+1) % 255
	signAddVotes(tmproto.PrecommitType, hash, config.ChainID(), blockID,
		rs.ProposalBlock.Hash(), config.ChainID(), rs.ProposalBlockParts.Header(), vs2)

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
	config := configSetup(t)

	cs1, vss := randState(config, 4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round
	partSize := types.BlockPartSizeBytes

	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	timeoutWaitCh := subscribe(cs1.eventBus, types.EventQueryTimeoutWait)
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	newBlockCh := subscribe(cs1.eventBus, types.EventQueryNewBlock)
	pv1, err := cs1.privValidator.GetPubKey(context.Background())
	require.NoError(t, err)
	addr := pv1.Address()
	voteCh := subscribeToVoter(cs1, addr)

	// start round and wait for propose and prevote
	startTestRound(cs1, height, round)
	ensureNewRound(t, newRoundCh, height, round)

	ensureNewProposal(t, proposalCh, height, round)
	rs := cs1.GetRoundState()
	propBlock := rs.ProposalBlock
	blockID := types.BlockID{
		Hash:          propBlock.Hash(),
		PartSetHeader: propBlock.MakePartSet(partSize).Header(),
	}

	ensurePrevote(t, voteCh, height, round)

	signAddVotes(cs1, tmproto.PrevoteType, config.ChainID(), blockID, vs2, vs3, vs4)

	ensurePrecommit(t, voteCh, height, round)
	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, round, round, vss[0], propBlock.Hash(), propBlock.Hash())

	// add precommits from the rest
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), types.BlockID{}, vs2) // didnt receive proposal
	signAddVotes(cs1, tmproto.PrecommitType, config.ChainID(), blockID, vs3)
	// we receive this later, but vs3 might receive it earlier and with ours will go to commit!
	precommit4 := signVote(vs4, tmproto.PrecommitType, config.ChainID(), blockID)

	incrementRound(vs2, vs3, vs4)

	// timeout to new round
	ensureNewTimeout(t, timeoutWaitCh, height, round, cs1.config.Precommit(round).Nanoseconds())

	round++ // moving to the next round

	ensureNewRound(t, newRoundCh, height, round)

	t.Log("### ONTO ROUND 1")
	/*Round2
	// we timeout and prevote
	// a polka happened but we didn't see it!
	*/

	// prevote for nil since we did not receive a proposal in this round.
	ensurePrevote(t, voteCh, height, round)
	validatePrevote(t, cs1, round, vss[0], nil)

	// now we receive the precommit from the previous round
	addVotes(cs1, precommit4)

	// receiving that precommit should take us straight to commit
	ensureNewBlock(t, newBlockCh, height)

	ensureNewRound(t, newRoundCh, height+1, 0)
}

func TestStateOutputsBlockPartsStats(t *testing.T) {
	config := configSetup(t)

	// create dummy peer
	cs, _ := randState(config, 1)
	peer := p2pmock.NewPeer(nil)

	// 1) new block part
	parts := types.NewPartSetFromData(tmrand.Bytes(100), 10)
	msg := &BlockPartMessage{
		Height: 1,
		Round:  0,
		Part:   parts.GetPart(0),
	}

	cs.ProposalBlockParts = types.NewPartSetFromHeader(parts.Header())
	cs.handleMsg(msgInfo{msg, peer.ID()})

	statsMessage := <-cs.statsMsgQueue
	require.Equal(t, msg, statsMessage.Msg, "")
	require.Equal(t, peer.ID(), statsMessage.PeerID, "")

	// sending the same part from different peer
	cs.handleMsg(msgInfo{msg, "peer2"})

	// sending the part with the same height, but different round
	msg.Round = 1
	cs.handleMsg(msgInfo{msg, peer.ID()})

	// sending the part from the smaller height
	msg.Height = 0
	cs.handleMsg(msgInfo{msg, peer.ID()})

	// sending the part from the bigger height
	msg.Height = 3
	cs.handleMsg(msgInfo{msg, peer.ID()})

	select {
	case <-cs.statsMsgQueue:
		t.Errorf("should not output stats message after receiving the known block part!")
	case <-time.After(50 * time.Millisecond):
	}

}

func TestStateOutputVoteStats(t *testing.T) {
	config := configSetup(t)

	cs, vss := randState(config, 2)
	// create dummy peer
	peer := p2pmock.NewPeer(nil)

	randBytes := tmrand.Bytes(tmhash.Size)
	blockID := types.BlockID{
		Hash: randBytes,
	}

	vote := signVote(vss[1], tmproto.PrecommitType, config.ChainID(), blockID)

	voteMessage := &VoteMessage{vote}
	cs.handleMsg(msgInfo{voteMessage, peer.ID()})

	statsMessage := <-cs.statsMsgQueue
	require.Equal(t, voteMessage, statsMessage.Msg, "")
	require.Equal(t, peer.ID(), statsMessage.PeerID, "")

	// sending the same part from different peer
	cs.handleMsg(msgInfo{&VoteMessage{vote}, "peer2"})

	// sending the vote for the bigger height
	incrementHeight(vss[1])
	vote = signVote(vss[1], tmproto.PrecommitType, config.ChainID(), blockID)

	cs.handleMsg(msgInfo{&VoteMessage{vote}, peer.ID()})

	select {
	case <-cs.statsMsgQueue:
		t.Errorf("should not output stats message after receiving the known vote or vote from bigger height")
	case <-time.After(50 * time.Millisecond):
	}

}

func TestSignSameVoteTwice(t *testing.T) {
	config := configSetup(t)

	_, vss := randState(config, 2)

	randBytes := tmrand.Bytes(tmhash.Size)

	vote := signVote(vss[1],
		tmproto.PrecommitType,
		config.ChainID(),
		types.BlockID{
			Hash:          randBytes,
			PartSetHeader: types.PartSetHeader{Total: 10, Hash: randBytes},
		},
	)

	vote2 := signVote(vss[1],
		tmproto.PrecommitType,
		config.ChainID(),

		types.BlockID{
			Hash:          randBytes,
			PartSetHeader: types.PartSetHeader{Total: 10, Hash: randBytes},
		},
	)

	require.Equal(t, vote, vote2)
}

// subscribe subscribes test client to the given query and returns a channel with cap = 1.
func subscribe(eventBus *types.EventBus, q tmpubsub.Query) <-chan tmpubsub.Message {
	sub, err := eventBus.Subscribe(context.Background(), testSubscriber, q)
	if err != nil {
		panic(fmt.Sprintf("failed to subscribe %s to %v", testSubscriber, q))
	}
	return sub.Out()
}

// subscribe subscribes test client to the given query and returns a channel with cap = 0.
func subscribeUnBuffered(eventBus *types.EventBus, q tmpubsub.Query) <-chan tmpubsub.Message {
	sub, err := eventBus.SubscribeUnbuffered(context.Background(), testSubscriber, q)
	if err != nil {
		panic(fmt.Sprintf("failed to subscribe %s to %v", testSubscriber, q))
	}
	return sub.Out()
}
