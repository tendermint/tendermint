package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
	. "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	. "github.com/tendermint/tendermint/state"
)

const (
	ProposalCh   = byte(0x20)
	KnownPartsCh = byte(0x21)
	VoteCh       = byte(0x22)

	voteTypeNil   = byte(0x00)
	voteTypeBlock = byte(0x01)

	roundDuration0         = 60 * time.Second   // The first round is 60 seconds long.
	roundDurationDelta     = 15 * time.Second   // Each successive round lasts 15 seconds longer.
	roundDeadlineBare      = float64(1.0 / 3.0) // When the bare vote is due.
	roundDeadlinePrecommit = float64(2.0 / 3.0) // When the precommit vote is due.

	newBlockWaitDuration = roundDuration0 / 3 // The time to wait between commitTime and startTime of next consensus rounds.
	voteRankCutoff       = 2                  // Higher ranks --> do not send votes.
	unsolicitedVoteRate  = 0.01               // Probability of sending a high ranked vote.
)

//-----------------------------------------------------------------------------

// total duration of given round
func calcRoundDuration(round uint16) time.Duration {
	return roundDuration0 + roundDurationDelta*time.Duration(round)
}

// startTime is when round zero started.
func calcRoundStartTime(round uint16, startTime time.Time) time.Time {
	return startTime.Add(roundDuration0*time.Duration(round) +
		roundDurationDelta*(time.Duration((int64(round)*int64(round)-int64(round))/2)))
}

// calculates the current round given startTime of round zero.
// NOTE: round is zero if startTime is in the future.
func calcRound(startTime time.Time) uint16 {
	now := time.Now()
	if now.Before(startTime) {
		return 0
	}
	// Start + D_0 * R + D_delta * (R^2 - R)/2 <= Now; find largest integer R.
	// D_delta * R^2 + (2D_0 - D_delta) * R + 2(Start - Now) <= 0.
	// AR^2 + BR + C <= 0; A = D_delta, B = (2_D0 - D_delta), C = 2(Start - Now).
	// R = Floor((-B + Sqrt(B^2 - 4AC))/2A)
	A := float64(roundDurationDelta)
	B := 2.0*float64(roundDuration0) - float64(roundDurationDelta)
	C := 2.0 * float64(startTime.Sub(now))
	R := math.Floor((-B + math.Sqrt(B*B-4.0*A*C)/(2*A)))
	if math.IsNaN(R) {
		panic("Could not calc round, should not happen")
	}
	if R > math.MaxInt16 {
		Panicf("Could not calc round, round overflow: %v", R)
	}
	if R < 0 {
		return 0
	}
	return uint16(R)
}

// convenience
// NOTE: elapsedRatio can be negative if startTime is in the future.
func calcRoundInfo(startTime time.Time) (round uint16, roundStartTime time.Time, roundDuration time.Duration,
	roundElapsed time.Duration, elapsedRatio float64) {
	round = calcRound(startTime)
	roundStartTime = calcRoundStartTime(round, startTime)
	roundDuration = calcRoundDuration(round)
	roundElapsed = time.Now().Sub(roundStartTime)
	elapsedRatio = float64(roundElapsed) / float64(roundDuration)
	return
}

//-----------------------------------------------------------------------------

type ConsensusAgent struct {
	sw       *p2p.Switch
	swEvents chan interface{}
	quit     chan struct{}
	started  uint32
	stopped  uint32

	conS       *ConsensusState
	blockStore *BlockStore
	mempool    *Mempool
	doActionCh chan RoundAction

	mtx            sync.Mutex
	state          *State
	privValidator  *PrivValidator
	peerStates     map[string]*PeerState
	stagedProposal *BlockPartSet
	stagedState    *State
}

func NewConsensusAgent(sw *p2p.Switch, blockStore *BlockStore, mempool *Mempool, state *State) *ConsensusAgent {
	swEvents := make(chan interface{})
	sw.AddEventListener("ConsensusAgent.swEvents", swEvents)
	conS := NewConsensusState(state)
	conA := &ConsensusAgent{
		sw:       sw,
		swEvents: swEvents,
		quit:     make(chan struct{}),

		conS:       conS,
		blockStore: blockStore,
		mempool:    mempool,
		doActionCh: make(chan RoundAction, 1),

		state:      state,
		peerStates: make(map[string]*PeerState),
	}
	return conA
}

// Sets our private validator account for signing votes.
func (conA *ConsensusAgent) SetPrivValidator(priv *PrivValidator) {
	conA.mtx.Lock()
	defer conA.mtx.Unlock()
	conA.privValidator = priv
}

func (conA *ConsensusAgent) PrivValidator() *PrivValidator {
	conA.mtx.Lock()
	defer conA.mtx.Unlock()
	return conA.privValidator
}

func (conA *ConsensusAgent) Start() {
	if atomic.CompareAndSwapUint32(&conA.started, 0, 1) {
		log.Info("Starting ConsensusAgent")
		go conA.switchEventsRoutine()
		go conA.gossipProposalRoutine()
		go conA.knownPartsRoutine()
		go conA.gossipVoteRoutine()
		go conA.proposeAndVoteRoutine()
	}
}

func (conA *ConsensusAgent) Stop() {
	if atomic.CompareAndSwapUint32(&conA.stopped, 0, 1) {
		log.Info("Stopping ConsensusAgent")
		close(conA.quit)
		close(conA.swEvents)
	}
}

// Handle peer new/done events
func (conA *ConsensusAgent) switchEventsRoutine() {
	for {
		swEvent, ok := <-conA.swEvents
		if !ok {
			break
		}
		switch swEvent.(type) {
		case p2p.SwitchEventNewPeer:
			event := swEvent.(p2p.SwitchEventNewPeer)
			// Create peerState for event.Peer
			conA.mtx.Lock()
			conA.peerStates[event.Peer.Key] = NewPeerState(event.Peer)
			conA.mtx.Unlock()
			// Share our state with event.Peer
			// By sending KnownBlockPartsMessage,
			// we send our height/round + startTime, and known block parts,
			// which is sufficient for the peer to begin interacting with us.
			event.Peer.TrySend(ProposalCh, conA.makeKnownBlockPartsMessage(conA.conS.RoundState()))
		case p2p.SwitchEventDonePeer:
			event := swEvent.(p2p.SwitchEventDonePeer)
			// Delete peerState for event.Peer
			conA.mtx.Lock()
			peerState := conA.peerStates[event.Peer.Key]
			if peerState != nil {
				peerState.Disconnect()
				delete(conA.peerStates, event.Peer.Key)
			}
			conA.mtx.Unlock()
		default:
			log.Warning("Unhandled switch event type")
		}
	}
}

// Like, how large is it and how often can we send it?
func (conA *ConsensusAgent) makeKnownBlockPartsMessage(rs *RoundState) *KnownBlockPartsMessage {
	return &KnownBlockPartsMessage{
		Height:                rs.Height,
		SecondsSinceStartTime: uint32(time.Now().Sub(rs.StartTime).Seconds()),
		BlockPartsBitArray:    rs.Proposal.BitArray(),
	}
}

// NOTE: may return nil, but (nil).Wants*() returns false.
func (conA *ConsensusAgent) getPeerState(peer *p2p.Peer) *PeerState {
	conA.mtx.Lock()
	defer conA.mtx.Unlock()
	return conA.peerStates[peer.Key]
}

func (conA *ConsensusAgent) gossipProposalRoutine() {
OUTER_LOOP:
	for {
		// Get round state
		rs := conA.conS.RoundState()

		// Receive incoming message on ProposalCh
		inMsg, ok := conA.sw.Receive(ProposalCh)
		if !ok {
			break OUTER_LOOP // Client has stopped
		}
		_, msg_ := decodeMessage(inMsg.Bytes)
		log.Info("gossipProposalRoutine received %v", msg_)

		switch msg_.(type) {
		case *BlockPartMessage:
			msg := msg_.(*BlockPartMessage)

			// Add the block part if the height matches.
			if msg.BlockPart.Height == rs.Height &&
				msg.BlockPart.Round == rs.Round {

				// TODO Continue if we've already voted, then no point processing the part.

				// Check that the signature is valid and from proposer.
				if rs.Proposer.Verify(msg.BlockPart.Hash(), msg.BlockPart.Signature) {
					// TODO handle bad peer.
					continue OUTER_LOOP
				}

				// If we are the proposer, then don't do anything else.
				// We're already sending peers our proposal on another routine.
				privValidator := conA.PrivValidator()
				if privValidator != nil && rs.Proposer.Account.Id == privValidator.Id {
					continue OUTER_LOOP
				}

				// Add and process the block part
				added, err := rs.Proposal.AddBlockPart(msg.BlockPart)
				if err == ErrInvalidBlockPartConflict {
					// TODO: Bad validator
				} else if err != nil {
					Panicf("Unexpected blockPartsSet error %v", err)
				}
				if added {
					// If peer wants this part, send peer the part
					// and our new blockParts state.
					kbpMsg := conA.makeKnownBlockPartsMessage(rs)
					partMsg := &BlockPartMessage{BlockPart: msg.BlockPart}
					for _, peer := range conA.sw.Peers().List() {
						peerState := conA.getPeerState(peer)
						if peerState.WantsBlockPart(msg.BlockPart) {
							peer.TrySend(KnownPartsCh, kbpMsg)
							peer.TrySend(ProposalCh, partMsg)
						}
					}

				} else {
					// We failed to process the block part.
					// Either an error, which we handled, or duplicate part.
					continue OUTER_LOOP
				}
			}

		default:
			// Ignore unknown message
			// conA.sw.StopPeerForError(inMsg.MConn.Peer, errInvalidMessage)
		}
	}

	// Cleanup
}

func (conA *ConsensusAgent) knownPartsRoutine() {
OUTER_LOOP:
	for {
		// Receive incoming message on ProposalCh
		inMsg, ok := conA.sw.Receive(KnownPartsCh)
		if !ok {
			break OUTER_LOOP // Client has stopped
		}
		_, msg_ := decodeMessage(inMsg.Bytes)
		log.Info("knownPartsRoutine received %v", msg_)

		msg, ok := msg_.(*KnownBlockPartsMessage)
		if !ok {
			// Ignore unknown message type
			// conA.sw.StopPeerForError(inMsg.MConn.Peer, errInvalidMessage)
			continue OUTER_LOOP
		}
		peerState := conA.getPeerState(inMsg.MConn.Peer)
		if !peerState.IsConnected() {
			// Peer disconnected before we were able to process.
			continue OUTER_LOOP
		}
		peerState.ApplyKnownBlockPartsMessage(msg)
	}

	// Cleanup
}

// Signs a vote document and broadcasts it.
// hash can be nil to vote "nil"
func (conA *ConsensusAgent) signAndVote(vote *Vote) error {
	privValidator := conA.PrivValidator()
	if privValidator != nil {
		err := privValidator.SignVote(vote)
		if err != nil {
			return err
		}
		msg := p2p.TypedMessage{msgTypeVote, vote}
		conA.sw.Broadcast(VoteCh, msg)
	}
	return nil
}

func (conA *ConsensusAgent) stageProposal(proposal *BlockPartSet) error {
	// Already staged?
	conA.mtx.Lock()
	if conA.stagedProposal == proposal {
		conA.mtx.Unlock()
		return nil
	} else {
		conA.mtx.Unlock()
	}

	if !proposal.IsComplete() {
		return errors.New("Incomplete proposal BlockPartSet")
	}
	block := proposal.Block()

	// Basic validation is done in state.CommitBlock().
	//err := block.ValidateBasic()
	//if err != nil {
	//	return err
	//}

	// Create a copy of the state for staging
	conA.mtx.Lock()
	stateCopy := conA.state.Copy() // Deep copy the state before staging.
	conA.mtx.Unlock()

	// Commit block onto the copied state.
	err := stateCopy.CommitBlock(block)
	if err != nil {
		return err
	}

	// Looks good!
	conA.mtx.Lock()
	conA.stagedProposal = proposal
	conA.stagedState = stateCopy
	conA.mtx.Unlock()
	return nil
}

// Constructs an unsigned proposal
func (conA *ConsensusAgent) constructProposal(rs *RoundState) (*BlockPartSet, error) {
	// TODO: make use of state returned from MakeProposal()
	proposalBlock, _ := conA.mempool.MakeProposal()
	proposal := proposalBlock.ToBlockPartSet()
	return proposal, nil
}

// Vote for (or against) the proposal for this round.
// Call during transition from RoundStepProposal to RoundStepVote.
// We may not have received a full proposal.
func (conA *ConsensusAgent) voteProposal(rs *RoundState) error {
	// If we're locked, must vote that.
	locked := conA.conS.LockedProposal()
	if locked != nil {
		block := locked.Block()
		err := conA.signAndVote(&Vote{
			Height: rs.Height,
			Round:  rs.Round,
			Type:   VoteTypeBare,
			Hash:   block.Hash(),
		})
		return err
	}
	// Stage proposal
	err := conA.stageProposal(rs.Proposal)
	if err != nil {
		// Vote for nil, whatever the error.
		err := conA.signAndVote(&Vote{
			Height: rs.Height,
			Round:  rs.Round,
			Type:   VoteTypeBare,
			Hash:   nil,
		})
		return err
	}
	// Vote for block.
	err = conA.signAndVote(&Vote{
		Height: rs.Height,
		Round:  rs.Round,
		Type:   VoteTypeBare,
		Hash:   rs.Proposal.Block().Hash(),
	})
	return err
}

// Precommit proposal if we see enough votes for it.
// Call during transition from RoundStepVote to RoundStepPrecommit.
func (conA *ConsensusAgent) precommitProposal(rs *RoundState) error {
	// If we see a 2/3 majority for votes for a block, precommit.

	// TODO: maybe could use commitTime here and avg it with later commitTime?
	if hash, _, ok := rs.RoundBareVotes.TwoThirdsMajority(); ok {
		if len(hash) == 0 {
			// 2/3 majority voted for nil.
			return nil
		} else {
			// 2/3 majority voted for a block.

			// If proposal is invalid or unknown, do nothing.
			// See note on ZombieValidators to see why.
			if conA.stageProposal(rs.Proposal) != nil {
				return nil
			}

			// Lock this proposal.
			// NOTE: we're unlocking any prior locks.
			conA.conS.LockProposal(rs.Proposal)

			// Send precommit vote.
			err := conA.signAndVote(&Vote{
				Height: rs.Height,
				Round:  rs.Round,
				Type:   VoteTypePrecommit,
				Hash:   hash,
			})
			return err
		}
	} else {
		// If we haven't seen enough votes, do nothing.
		return nil
	}
}

// Commit or unlock.
// Call after RoundStepPrecommit, after round has completely expired.
func (conA *ConsensusAgent) commitOrUnlockProposal(rs *RoundState) (commitTime time.Time, err error) {
	// If there exists a 2/3 majority of precommits.
	// Validate the block and commit.
	if hash, commitTime, ok := rs.RoundPrecommits.TwoThirdsMajority(); ok {

		// If the proposal is invalid or we don't have it,
		// do not commit.
		// TODO If we were just late to receive the block, when
		// do we actually get it? Document it.
		if conA.stageProposal(rs.Proposal) != nil {
			return time.Time{}, nil
		}
		// TODO: Remove?
		conA.conS.LockProposal(rs.Proposal)
		// Vote commit.
		err := conA.signAndVote(&Vote{
			Height: rs.Height,
			Round:  rs.Round,
			Type:   VoteTypePrecommit,
			Hash:   hash,
		})
		if err != nil {
			return time.Time{}, err
		}
		// Commit block.
		conA.commitProposal(rs.Proposal, commitTime)
		return commitTime, nil
	} else {
		// Otherwise, if a 1/3 majority if a block that isn't our locked one exists, unlock.
		locked := conA.conS.LockedProposal()
		if locked != nil {
			for _, hashOrNil := range rs.RoundPrecommits.OneThirdMajority() {
				if hashOrNil == nil {
					continue
				}
				if !bytes.Equal(hashOrNil, locked.Block().Hash()) {
					// Unlock our lock.
					conA.conS.LockProposal(nil)
				}
			}
		}
		return time.Time{}, nil
	}
}

func (conA *ConsensusAgent) commitProposal(proposal *BlockPartSet, commitTime time.Time) error {
	conA.mtx.Lock()
	defer conA.mtx.Unlock()

	if conA.stagedProposal != proposal {
		panic("Unexpected stagedProposal.") // Shouldn't happen.
	}

	// Save to blockStore
	block, blockParts := proposal.Block(), proposal.BlockParts()
	err := conA.blockStore.SaveBlockParts(block.Height, blockParts)
	if err != nil {
		return err
	}

	// What was staged becomes committed.
	conA.state = conA.stagedState
	conA.state.Save(commitTime)
	conA.conS.Update(conA.state)
	conA.stagedProposal = nil
	conA.stagedState = nil
	conA.mempool.ResetForBlockAndState(block, conA.state)

	return nil
}

// Given a RoundState where we are the proposer,
// broadcast rs.proposal to all the peers.
func (conA *ConsensusAgent) shareProposal(rs *RoundState) {
	privValidator := conA.PrivValidator()
	proposal := rs.Proposal
	if privValidator == nil || proposal == nil {
		return
	}
	privValidator.SignProposal(rs.Round, proposal)
	blockParts := proposal.BlockParts()
	peers := conA.sw.Peers().List()
	if len(peers) == 0 {
		log.Warning("Could not propose: no peers")
		return
	}
	numBlockParts := uint16(len(blockParts))
	kbpMsg := conA.makeKnownBlockPartsMessage(rs)
	for i, peer := range peers {
		peerState := conA.getPeerState(peer)
		if !peerState.IsConnected() {
			continue // Peer was disconnected.
		}
		startIndex := uint16((i * len(blockParts)) / len(peers))
		// Create a function that when called,
		// starts sending block parts to peer.
		cb := func(peer *p2p.Peer, startIndex uint16) func() {
			return func() {
				// TODO: if the clocks are off a bit,
				// peer may receive this before the round flips.
				peer.Send(KnownPartsCh, kbpMsg)
				for i := uint16(0); i < numBlockParts; i++ {
					part := blockParts[(startIndex+i)%numBlockParts]
					// Ensure round hasn't expired on our end.
					currentRS := conA.conS.RoundState()
					if currentRS != rs {
						return
					}
					// If peer wants the block:
					if peerState.WantsBlockPart(part) {
						partMsg := &BlockPartMessage{BlockPart: part}
						peer.Send(ProposalCh, partMsg)
					}
				}
			}
		}(peer, startIndex)
		// Call immediately or schedule cb for when peer is ready.
		peerState.SetRoundCallback(rs.Height, rs.Round, cb)
	}
}

func (conA *ConsensusAgent) gossipVoteRoutine() {
OUTER_LOOP:
	for {
		// Get round state
		rs := conA.conS.RoundState()

		// Receive incoming message on VoteCh
		inMsg, ok := conA.sw.Receive(VoteCh)
		if !ok {
			break // Client has stopped
		}
		type_, msg_ := decodeMessage(inMsg.Bytes)
		log.Info("gossipVoteRoutine received %v", msg_)

		switch msg_.(type) {
		case *Vote:
			vote := msg_.(*Vote)

			if vote.Height != rs.Height || vote.Round != rs.Round {
				continue OUTER_LOOP
			}

			added, rank, err := rs.AddVote(vote, inMsg.MConn.Peer.Key)
			// Send peer VoteRankMessage if needed
			if type_ == msgTypeVoteAskRank {
				msg := &VoteRankMessage{
					ValidatorId: vote.SignerId,
					Rank:        rank,
				}
				inMsg.MConn.Peer.TrySend(VoteCh, msg)
			}
			// Process vote
			if !added {
				log.Info("Error adding vote %v", err)
			}
			switch err {
			case ErrVoteInvalidAccount, ErrVoteInvalidSignature:
				// TODO: Handle bad peer.
			case ErrVoteConflictingSignature, ErrVoteInvalidHash:
				// TODO: Handle bad validator.
			case nil:
				break
			//case ErrVoteUnexpectedPhase: Shouldn't happen.
			default:
				Panicf("Unexpected error from .AddVote(): %v", err)
			}
			if !added {
				continue
			}

			// Gossip vote.
			for _, peer := range conA.sw.Peers().List() {
				peerState := conA.getPeerState(peer)
				wantsVote, unsolicited := peerState.WantsVote(vote)
				if wantsVote {
					if unsolicited {
						// If we're sending an unsolicited vote,
						// ask for the rank so we know whether it's good.
						msg := p2p.TypedMessage{msgTypeVoteAskRank, vote}
						peer.TrySend(VoteCh, msg)
					} else {
						msg := p2p.TypedMessage{msgTypeVote, vote}
						peer.TrySend(VoteCh, msg)
					}
				}
			}

		case *VoteRankMessage:
			msg := msg_.(*VoteRankMessage)

			peerState := conA.getPeerState(inMsg.MConn.Peer)
			if !peerState.IsConnected() {
				// Peer disconnected before we were able to process.
				continue OUTER_LOOP
			}
			peerState.ApplyVoteRankMessage(msg)

		default:
			// Ignore unknown message
			// conA.sw.StopPeerForError(inMsg.MConn.Peer, errInvalidMessage)
		}
	}

	// Cleanup
}

type RoundAction struct {
	Height   uint32 // The block height for which consensus is reaching for.
	Round    uint16 // The round number at given height.
	XnToStep uint8  // Transition to this step. Action depends on this value.
}

// Source of all round state transitions and votes.
// It can be preemptively woken up via  amessage to
// doActionCh.
func (conA *ConsensusAgent) proposeAndVoteRoutine() {

	// Figure out when to wake up next (in the absence of other events)
	setAlarm := func() {
		if len(conA.doActionCh) > 0 {
			return // Already going to wake up later.
		}

		// Figure out which height/round/step we're at,
		// then schedule an action for when it is due.
		rs := conA.conS.RoundState()
		_, _, roundDuration, _, elapsedRatio := calcRoundInfo(rs.StartTime)
		switch rs.Step() {
		case RoundStepStart:
			// It's a new RoundState.
			if elapsedRatio < 0 {
				// startTime is in the future.
				time.Sleep(time.Duration(-1.0*elapsedRatio) * roundDuration)
			}
			conA.doActionCh <- RoundAction{rs.Height, rs.Round, RoundStepProposal}
		case RoundStepProposal:
			// Wake up when it's time to vote.
			time.Sleep(time.Duration(roundDeadlineBare-elapsedRatio) * roundDuration)
			conA.doActionCh <- RoundAction{rs.Height, rs.Round, RoundStepBareVotes}
		case RoundStepBareVotes:
			// Wake up when it's time to precommit.
			time.Sleep(time.Duration(roundDeadlinePrecommit-elapsedRatio) * roundDuration)
			conA.doActionCh <- RoundAction{rs.Height, rs.Round, RoundStepPrecommits}
		case RoundStepPrecommits:
			// Wake up when the round is over.
			time.Sleep(time.Duration(1.0-elapsedRatio) * roundDuration)
			conA.doActionCh <- RoundAction{rs.Height, rs.Round, RoundStepCommitOrUnlock}
		case RoundStepCommitOrUnlock:
			// This shouldn't happen.
			// Before setAlarm() got called,
			// logic should have created a new RoundState for the next round.
			panic("Should not happen")
		}
	}

	for {
		func() {
			roundAction := <-conA.doActionCh
			// Always set the alarm after any processing below.
			defer setAlarm()

			// We only consider acting on given height and round.
			height := roundAction.Height
			round := roundAction.Round
			// We only consider transitioning to given step.
			step := roundAction.XnToStep
			// This is the current state.
			rs := conA.conS.RoundState()
			if height != rs.Height || round != rs.Round {
				return // Not relevant.
			}

			if step == RoundStepProposal && rs.Step() == RoundStepStart {
				// Propose a block if I am the proposer.
				privValidator := conA.PrivValidator()
				if privValidator != nil && rs.Proposer.Account.Id == privValidator.Id {
					// If we're already locked on a proposal, use that.
					proposal := conA.conS.LockedProposal()
					if proposal != nil {
						// Otherwise, construct a new proposal.
						var err error
						proposal, err = conA.constructProposal(rs)
						if err != nil {
							log.Error("Error attempting to construct a proposal: %v", err)
							return // Pretend like we weren't the proposer. Shrug.
						}
					}
					// Set proposal for roundState, so we vote correctly subsequently.
					rs.Proposal = proposal
					// Share the parts.
					// We send all parts to all of our peers, but everyone receives parts
					// starting at a different index, wrapping around back to 0.
					conA.shareProposal(rs)
				}
			} else if step == RoundStepBareVotes && rs.Step() <= RoundStepProposal {
				err := conA.voteProposal(rs)
				if err != nil {
					log.Info("Error attempting to vote for proposal: %v", err)
				}
			} else if step == RoundStepPrecommits && rs.Step() <= RoundStepBareVotes {
				err := conA.precommitProposal(rs)
				if err != nil {
					log.Info("Error attempting to precommit for proposal: %v", err)
				}
			} else if step == RoundStepCommitOrUnlock && rs.Step() <= RoundStepPrecommits {
				commitTime, err := conA.commitOrUnlockProposal(rs)
				if err != nil {
					log.Info("Error attempting to commit or update for proposal: %v", err)
				}

				if !commitTime.IsZero() {
					// We already set up ConsensusState for the next height
					// (it happens in the call to conA.commitProposal).
				} else {
					// Round is over. This is a special case.
					// Prepare a new RoundState for the next state.
					conA.conS.SetupRound(rs.Round + 1)
					return // setAlarm() takes care of the rest.
				}
			} else {
				return // Action is not relevant.
			}

			// Transition to new step.
			rs.SetStep(step)
		}()
	}
}

//-----------------------------------------------------------------------------

var (
	ErrPeerStateHeightRegression = errors.New("Error peer state height regression")
	ErrPeerStateInvalidStartTime = errors.New("Error peer state invalid startTime")
)

// TODO: voteRanks should purge bygone validators.
type PeerState struct {
	mtx                sync.Mutex
	connected          bool
	peer               *p2p.Peer
	height             uint32
	startTime          time.Time // Derived from offset seconds.
	blockPartsBitArray []byte
	voteRanks          map[uint64]uint8
	cbHeight           uint32
	cbRound            uint16
	cbFunc             func()
}

func NewPeerState(peer *p2p.Peer) *PeerState {
	return &PeerState{
		connected: true,
		peer:      peer,
		height:    0,
		voteRanks: make(map[uint64]uint8),
	}
}

func (ps *PeerState) IsConnected() bool {
	if ps == nil {
		return false
	}
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return ps.connected
}

func (ps *PeerState) Disconnect() {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	ps.connected = false
}

func (ps *PeerState) WantsBlockPart(part *BlockPart) bool {
	if ps == nil {
		return false
	}
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	if !ps.connected {
		return false
	}
	// Only wants the part if peer's current height and round matches.
	if ps.height == part.Height {
		round := calcRound(ps.startTime)
		// NOTE: validators want to receive remaining block parts
		// even after it had voted bare or precommit.
		// Ergo, we do not check for which step the peer is in.
		if round == part.Round {
			// Only wants the part if it doesn't already have it.
			if ps.blockPartsBitArray[part.Index/8]&byte(1<<(part.Index%8)) == 0 {
				return true
			}
		}
	}
	return false
}

func (ps *PeerState) WantsVote(vote *Vote) (wants bool, unsolicited bool) {
	if ps == nil {
		return false, false
	}
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	if !ps.connected {
		return false, false
	}
	// Only wants the vote if peer's current height and round matches.
	if ps.height == vote.Height {
		round, _, _, _, elapsedRatio := calcRoundInfo(ps.startTime)
		if round == vote.Round {
			if vote.Type == VoteTypeBare && elapsedRatio > roundDeadlineBare {
				return false, false
			}
			if vote.Type == VoteTypePrecommit && elapsedRatio > roundDeadlinePrecommit {
				return false, false
			} else {
				// continue on ...
			}
		} else {
			return false, false
		}
	} else {
		return false, false
	}
	// Only wants the vote if voteRank is low.
	if ps.voteRanks[vote.SignerId] > voteRankCutoff {
		// Sometimes, send unsolicited votes to see if peer wants it.
		if rand.Float32() < unsolicitedVoteRate {
			return true, true
		} else {
			// Rank too high. Do not send vote.
			return false, false
		}
	}
	return true, false
}

func (ps *PeerState) ApplyKnownBlockPartsMessage(msg *KnownBlockPartsMessage) error {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	// TODO: Sanity check len(BlockParts)
	if msg.Height < ps.height {
		return ErrPeerStateHeightRegression
	}
	if msg.Height == ps.height {
		if len(ps.blockPartsBitArray) == 0 {
			ps.blockPartsBitArray = msg.BlockPartsBitArray
		} else if len(msg.BlockPartsBitArray) > 0 {
			if len(ps.blockPartsBitArray) != len(msg.BlockPartsBitArray) {
				// TODO: If the peer received a part from
				// a proposer who signed a bad (or conflicting) part,
				// just about anything can happen with the new blockPartsBitArray.
				// In those cases it's alright to ignore the peer for the round,
				// and try to induce nil votes for that round.
				return nil
			} else {
				// TODO: Same as above. If previously known parts disappear,
				// something is fishy.
				// For now, just copy over known parts.
				for i, byt := range msg.BlockPartsBitArray {
					ps.blockPartsBitArray[i] |= byt
				}
			}
		}
	} else {
		// TODO: handle peer connection latency estimation.
		newStartTime := time.Now().Add(-1 * time.Duration(msg.SecondsSinceStartTime) * time.Second)
		// Ensure that the new height's start time is sufficiently after the last startTime.
		// TODO: there should be some time between rounds.
		if !newStartTime.After(ps.startTime) {
			return ErrPeerStateInvalidStartTime
		}
		ps.startTime = newStartTime
		ps.height = msg.Height
		ps.blockPartsBitArray = msg.BlockPartsBitArray
		// Call callback if height+round matches.
		peerRound := calcRound(ps.startTime)
		if ps.cbFunc != nil && ps.cbHeight == ps.height && ps.cbRound == peerRound {
			go ps.cbFunc()
			ps.cbFunc = nil
		}
	}
	return nil
}

func (ps *PeerState) ApplyVoteRankMessage(msg *VoteRankMessage) error {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	ps.voteRanks[msg.ValidatorId] = msg.Rank
	return nil
}

// Sets a single round callback, to be called when the height+round comes around.
// If the height+round is current, calls "go f()" immediately.
// Otherwise, does nothing.
func (ps *PeerState) SetRoundCallback(height uint32, round uint16, f func()) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	if ps.height < height {
		ps.cbHeight = height
		ps.cbRound = round
		ps.cbFunc = f
		// Wait until the height of the peerState changes.
		// We'll call cbFunc then.
		return
	} else if ps.height == height {
		peerRound := calcRound(ps.startTime)
		if peerRound < round {
			// Set a timer to call the cbFunc when the time comes.
			go func() {
				roundStart := calcRoundStartTime(round, ps.startTime)
				time.Sleep(roundStart.Sub(time.Now()))
				// If peer height is still good
				ps.mtx.Lock()
				peerHeight := ps.height
				ps.mtx.Unlock()
				if peerHeight == height {
					f()
				}
			}()
		} else if peerRound == round {
			go f()
		} else {
			return
		}
	} else {
		return
	}
}

//-----------------------------------------------------------------------------
// Messages

const (
	msgTypeUnknown         = byte(0x00)
	msgTypeBlockPart       = byte(0x10)
	msgTypeKnownBlockParts = byte(0x11)
	msgTypeVote            = byte(0x20)
	msgTypeVoteAskRank     = byte(0x21)
	msgTypeVoteRank        = byte(0x22)
)

// TODO: check for unnecessary extra bytes at the end.
func decodeMessage(bz []byte) (msgType byte, msg interface{}) {
	n, err := new(int64), new(error)
	// log.Debug("decoding msg bytes: %X", bz)
	msgType = bz[0]
	switch msgType {
	case msgTypeBlockPart:
		msg = readBlockPartMessage(bytes.NewReader(bz[1:]), n, err)
	case msgTypeKnownBlockParts:
		msg = readKnownBlockPartsMessage(bytes.NewReader(bz[1:]), n, err)
	case msgTypeVote:
		msg = ReadVote(bytes.NewReader(bz[1:]), n, err)
	case msgTypeVoteAskRank:
		msg = ReadVote(bytes.NewReader(bz[1:]), n, err)
	case msgTypeVoteRank:
		msg = readVoteRankMessage(bytes.NewReader(bz[1:]), n, err)
	default:
		msg = nil
	}
	return
}

//-------------------------------------

type BlockPartMessage struct {
	BlockPart *BlockPart
}

func readBlockPartMessage(r io.Reader, n *int64, err *error) *BlockPartMessage {
	return &BlockPartMessage{
		BlockPart: ReadBlockPart(r, n, err),
	}
}

func (m *BlockPartMessage) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, msgTypeBlockPart, &n, &err)
	WriteBinary(w, m.BlockPart, &n, &err)
	return
}

func (m *BlockPartMessage) String() string {
	return fmt.Sprintf("[BlockPartMessage %v]", m.BlockPart)
}

//-------------------------------------

type KnownBlockPartsMessage struct {
	Height                uint32
	SecondsSinceStartTime uint32
	BlockPartsBitArray    []byte
}

func readKnownBlockPartsMessage(r io.Reader, n *int64, err *error) *KnownBlockPartsMessage {
	return &KnownBlockPartsMessage{
		Height:                ReadUInt32(r, n, err),
		SecondsSinceStartTime: ReadUInt32(r, n, err),
		BlockPartsBitArray:    ReadByteSlice(r, n, err),
	}
}

func (m *KnownBlockPartsMessage) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, msgTypeKnownBlockParts, &n, &err)
	WriteUInt32(w, m.Height, &n, &err)
	WriteUInt32(w, m.SecondsSinceStartTime, &n, &err)
	WriteByteSlice(w, m.BlockPartsBitArray, &n, &err)
	return
}

func (m *KnownBlockPartsMessage) String() string {
	return fmt.Sprintf("[KnownBlockPartsMessage H:%v SSST:%v, BPBA:%X]",
		m.Height, m.SecondsSinceStartTime, m.BlockPartsBitArray)
}

//-------------------------------------

type VoteRankMessage struct {
	ValidatorId uint64
	Rank        uint8
}

func readVoteRankMessage(r io.Reader, n *int64, err *error) *VoteRankMessage {
	return &VoteRankMessage{
		ValidatorId: ReadUInt64(r, n, err),
		Rank:        ReadUInt8(r, n, err),
	}
}

func (m *VoteRankMessage) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, msgTypeVoteRank, &n, &err)
	WriteUInt64(w, m.ValidatorId, &n, &err)
	WriteUInt8(w, m.Rank, &n, &err)
	return
}

func (m *VoteRankMessage) String() string {
	return fmt.Sprintf("[VoteRankMessage V:%v, R:%v]", m.ValidatorId, m.Rank)
}
