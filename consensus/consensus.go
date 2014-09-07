package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
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

// calcs the current round given startTime of round zero.
func calcRound(startTime time.Time) uint16 {
	now := time.Now()
	if now.Before(startTime) {
		Panicf("Cannot calc round when startTime is in the future: %v", startTime)
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

type ConsensusManager struct {
	sw       *p2p.Switch
	swEvents chan interface{}
	quit     chan struct{}
	started  uint32
	stopped  uint32

	cs         *ConsensusState
	blockStore *BlockStore
	doActionCh chan RoundAction

	mtx            sync.Mutex
	state          *State
	privValidator  *PrivValidator
	peerStates     map[string]*PeerState
	stagedProposal *BlockPartSet
	stagedState    *State
}

func NewConsensusManager(sw *p2p.Switch, state *State, blockStore *BlockStore) *ConsensusManager {
	swEvents := make(chan interface{})
	sw.AddEventListener("ConsensusManager.swEvents", swEvents)
	cs := NewConsensusState(state)
	cm := &ConsensusManager{
		sw:       sw,
		swEvents: swEvents,
		quit:     make(chan struct{}),

		cs:         cs,
		blockStore: blockStore,
		doActionCh: make(chan RoundAction, 1),

		state:      state,
		peerStates: make(map[string]*PeerState),
	}
	return cm
}

// Sets our private validator account for signing votes.
func (cm *ConsensusManager) SetPrivValidator(priv *PrivValidator) {
	cm.mtx.Lock()
	defer cm.mtx.Unlock()
	cm.privValidator = priv
}

func (cm *ConsensusManager) PrivValidator() *PrivValidator {
	cm.mtx.Lock()
	defer cm.mtx.Unlock()
	return cm.privValidator
}

func (cm *ConsensusManager) Start() {
	if atomic.CompareAndSwapUint32(&cm.started, 0, 1) {
		log.Info("Starting ConsensusManager")
		go cm.switchEventsRoutine()
		go cm.gossipProposalRoutine()
		go cm.knownPartsRoutine()
		go cm.gossipVoteRoutine()
		go cm.proposeAndVoteRoutine()
	}
}

func (cm *ConsensusManager) Stop() {
	if atomic.CompareAndSwapUint32(&cm.stopped, 0, 1) {
		log.Info("Stopping ConsensusManager")
		close(cm.quit)
		close(cm.swEvents)
	}
}

// Handle peer new/done events
func (cm *ConsensusManager) switchEventsRoutine() {
	for {
		swEvent, ok := <-cm.swEvents
		if !ok {
			break
		}
		switch swEvent.(type) {
		case p2p.SwitchEventNewPeer:
			event := swEvent.(p2p.SwitchEventNewPeer)
			// Create peerState for event.Peer
			cm.mtx.Lock()
			cm.peerStates[event.Peer.Key] = NewPeerState(event.Peer)
			cm.mtx.Unlock()
			// Share our state with event.Peer
			// By sending KnownBlockPartsMessage,
			// we send our height/round + startTime, and known block parts,
			// which is sufficient for the peer to begin interacting with us.
			event.Peer.TrySend(ProposalCh, cm.makeKnownBlockPartsMessage(cm.cs.RoundState()))
		case p2p.SwitchEventDonePeer:
			event := swEvent.(p2p.SwitchEventDonePeer)
			// Delete peerState for event.Peer
			cm.mtx.Lock()
			peerState := cm.peerStates[event.Peer.Key]
			if peerState != nil {
				peerState.Disconnect()
				delete(cm.peerStates, event.Peer.Key)
			}
			cm.mtx.Unlock()
		default:
			log.Warning("Unhandled switch event type")
		}
	}
}

// Like, how large is it and how often can we send it?
func (cm *ConsensusManager) makeKnownBlockPartsMessage(rs *RoundState) *KnownBlockPartsMessage {
	return &KnownBlockPartsMessage{
		Height:                rs.Height,
		SecondsSinceStartTime: uint32(time.Now().Sub(rs.StartTime).Seconds()),
		BlockPartsBitArray:    rs.Proposal.BitArray(),
	}
}

// NOTE: may return nil, but (nil).Wants*() returns false.
func (cm *ConsensusManager) getPeerState(peer *p2p.Peer) *PeerState {
	cm.mtx.Lock()
	defer cm.mtx.Unlock()
	return cm.peerStates[peer.Key]
}

func (cm *ConsensusManager) gossipProposalRoutine() {
OUTER_LOOP:
	for {
		// Get round state
		rs := cm.cs.RoundState()

		// Receive incoming message on ProposalCh
		inMsg, ok := cm.sw.Receive(ProposalCh)
		if !ok {
			break OUTER_LOOP // Client has stopped
		}
		msg_ := decodeMessage(inMsg.Bytes)
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
				privValidator := cm.PrivValidator()
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
					kbpMsg := cm.makeKnownBlockPartsMessage(rs)
					partMsg := &BlockPartMessage{BlockPart: msg.BlockPart}
					for _, peer := range cm.sw.Peers().List() {
						peerState := cm.getPeerState(peer)
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
			// cm.sw.StopPeerForError(inMsg.MConn.Peer, errInvalidMessage)
		}
	}

	// Cleanup
}

func (cm *ConsensusManager) knownPartsRoutine() {
OUTER_LOOP:
	for {
		// Receive incoming message on ProposalCh
		inMsg, ok := cm.sw.Receive(KnownPartsCh)
		if !ok {
			break OUTER_LOOP // Client has stopped
		}
		msg_ := decodeMessage(inMsg.Bytes)
		log.Info("knownPartsRoutine received %v", msg_)

		msg, ok := msg_.(*KnownBlockPartsMessage)
		if !ok {
			// Ignore unknown message type
			// cm.sw.StopPeerForError(inMsg.MConn.Peer, errInvalidMessage)
			continue OUTER_LOOP
		}
		peerState := cm.getPeerState(inMsg.MConn.Peer)
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
func (cm *ConsensusManager) signAndVote(vote *Vote) error {
	privValidator := cm.PrivValidator()
	if privValidator != nil {
		err := privValidator.SignVote(vote)
		if err != nil {
			return err
		}
		msg := p2p.TypedMessage{msgTypeVote, vote}
		cm.sw.Broadcast(VoteCh, msg)
	}
	return nil
}

func (cm *ConsensusManager) stageProposal(proposal *BlockPartSet) error {
	// Already staged?
	cm.mtx.Lock()
	if cm.stagedProposal == proposal {
		cm.mtx.Unlock()
		return nil
	} else {
		cm.mtx.Unlock()
	}

	// Basic validation
	if !proposal.IsComplete() {
		return errors.New("Incomplete proposal BlockPartSet")
	}
	block := proposal.Block()
	err := block.ValidateBasic()
	if err != nil {
		return err
	}

	// Create a copy of the state for staging
	cm.mtx.Lock()
	stateCopy := cm.state.Copy() // Deep copy the state before staging.
	cm.mtx.Unlock()

	// Commit block onto the copied state.
	err = stateCopy.CommitBlock(block, block.Header.Time) // NOTE: fake commit time.
	if err != nil {
		return err
	}

	// Looks good!
	cm.mtx.Lock()
	cm.stagedProposal = proposal
	cm.stagedState = stateCopy
	cm.mtx.Unlock()
	return nil
}

// Constructs an unsigned proposal
func (cm *ConsensusManager) constructProposal(rs *RoundState) (*BlockPartSet, error) {
	// XXX implement, first implement mempool
	// proposal := block.ToBlockPartSet()
	return nil, nil
}

// Vote for (or against) the proposal for this round.
// Call during transition from RoundStepProposal to RoundStepVote.
// We may not have received a full proposal.
func (cm *ConsensusManager) voteProposal(rs *RoundState) error {
	// If we're locked, must vote that.
	locked := cm.cs.LockedProposal()
	if locked != nil {
		block := locked.Block()
		err := cm.signAndVote(&Vote{
			Height: rs.Height,
			Round:  rs.Round,
			Type:   VoteTypeBare,
			Hash:   block.Hash(),
		})
		return err
	}
	// Stage proposal
	err := cm.stageProposal(rs.Proposal)
	if err != nil {
		// Vote for nil, whatever the error.
		err := cm.signAndVote(&Vote{
			Height: rs.Height,
			Round:  rs.Round,
			Type:   VoteTypeBare,
			Hash:   nil,
		})
		return err
	}
	// Vote for block.
	err = cm.signAndVote(&Vote{
		Height: rs.Height,
		Round:  rs.Round,
		Type:   VoteTypeBare,
		Hash:   rs.Proposal.Block().Hash(),
	})
	return err
}

// Precommit proposal if we see enough votes for it.
// Call during transition from RoundStepVote to RoundStepPrecommit.
func (cm *ConsensusManager) precommitProposal(rs *RoundState) error {
	// If we see a 2/3 majority for votes for a block, precommit.

	if hash, ok := rs.RoundBareVotes.TwoThirdsMajority(); ok {
		if len(hash) == 0 {
			// 2/3 majority voted for nil.
			return nil
		} else {
			// 2/3 majority voted for a block.

			// If proposal is invalid or unknown, do nothing.
			// See note on ZombieValidators to see why.
			if cm.stageProposal(rs.Proposal) != nil {
				return nil
			}

			// Lock this proposal.
			// NOTE: we're unlocking any prior locks.
			cm.cs.LockProposal(rs.Proposal)

			// Send precommit vote.
			err := cm.signAndVote(&Vote{
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
// Call after RoundStepPrecommit, after round has expired.
func (cm *ConsensusManager) commitOrUnlockProposal(rs *RoundState) error {
	// If there exists a 2/3 majority of precommits.
	// Validate the block and commit.
	if hash, ok := rs.RoundPrecommits.TwoThirdsMajority(); ok {

		// If the proposal is invalid or we don't have it,
		// do not commit.
		// TODO If we were just late to receive the block, when
		// do we actually get it? Document it.
		if cm.stageProposal(rs.Proposal) != nil {
			return nil
		}
		// TODO: Remove?
		cm.cs.LockProposal(rs.Proposal)
		// Vote commit.
		err := cm.signAndVote(&Vote{
			Height: rs.Height,
			Round:  rs.Round,
			Type:   VoteTypePrecommit,
			Hash:   hash,
		})
		if err != nil {
			return err
		}
		// Commit block.
		// XXX use adjusted commit time.
		// If we just use time.Now() we're not converging
		// time differences between nodes, so nodes end up drifting
		// in time.
		commitTime := time.Now()
		cm.commitProposal(rs.Proposal, commitTime)
		return nil
	} else {
		// Otherwise, if a 1/3 majority if a block that isn't our locked one exists, unlock.
		locked := cm.cs.LockedProposal()
		if locked != nil {
			for _, hashOrNil := range rs.RoundPrecommits.OneThirdMajority() {
				if hashOrNil == nil {
					continue
				}
				hash := hashOrNil.([]byte)
				if !bytes.Equal(hash, locked.Block().Hash()) {
					// Unlock our lock.
					cm.cs.LockProposal(nil)
				}
			}
		}
		return nil
	}
}

func (cm *ConsensusManager) commitProposal(proposal *BlockPartSet, commitTime time.Time) error {
	cm.mtx.Lock()
	defer cm.mtx.Unlock()

	if cm.stagedProposal != proposal {
		panic("Unexpected stagedProposal.") // Shouldn't happen.
	}

	// Save to blockStore
	block, blockParts := proposal.Block(), proposal.BlockParts()
	err := cm.blockStore.SaveBlockParts(block.Height, blockParts)
	if err != nil {
		return err
	}

	// What was staged becomes committed.
	cm.state = cm.stagedState
	cm.cs.Update(cm.state)
	cm.stagedProposal = nil
	cm.stagedState = nil

	return nil
}

// Given a RoundState where we are the proposer,
// broadcast rs.proposal to all the peers.
func (cm *ConsensusManager) shareProposal(rs *RoundState) {
	privValidator := cm.PrivValidator()
	proposal := rs.Proposal
	if privValidator == nil || proposal == nil {
		return
	}
	privValidator.SignProposal(rs.Round, proposal)
	blockParts := proposal.BlockParts()
	peers := cm.sw.Peers().List()
	if len(peers) == 0 {
		log.Warning("Could not propose: no peers")
		return
	}
	numBlockParts := uint16(len(blockParts))
	kbpMsg := cm.makeKnownBlockPartsMessage(rs)
	for i, peer := range peers {
		peerState := cm.getPeerState(peer)
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
					currentRS := cm.cs.RoundState()
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

func (cm *ConsensusManager) gossipVoteRoutine() {
OUTER_LOOP:
	for {
		// Get round state
		rs := cm.cs.RoundState()

		// Receive incoming message on VoteCh
		inMsg, ok := cm.sw.Receive(VoteCh)
		if !ok {
			break // Client has stopped
		}
		msg_ := decodeMessage(inMsg.Bytes)
		log.Info("gossipVoteRoutine received %v", msg_)

		switch msg_.(type) {
		case *Vote:
			vote := msg_.(*Vote)

			if vote.Height != rs.Height || vote.Round != rs.Round {
				continue OUTER_LOOP
			}

			added, err := rs.AddVote(vote)
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
			for _, peer := range cm.sw.Peers().List() {
				peerState := cm.getPeerState(peer)
				if peerState.WantsVote(vote) {
					msg := p2p.TypedMessage{msgTypeVote, vote}
					peer.TrySend(VoteCh, msg)
				}
			}

		default:
			// Ignore unknown message
			// cm.sw.StopPeerForError(inMsg.MConn.Peer, errInvalidMessage)
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
func (cm *ConsensusManager) proposeAndVoteRoutine() {

	// Figure out when to wake up next (in the absence of other events)
	setAlarm := func() {
		if len(cm.doActionCh) > 0 {
			return // Already going to wake up later.
		}

		// Figure out which height/round/step we're at,
		// then schedule an action for when it is due.
		rs := cm.cs.RoundState()
		_, _, roundDuration, _, elapsedRatio := calcRoundInfo(rs.StartTime)
		switch rs.Step() {
		case RoundStepStart:
			// It's a new RoundState, immediately wake up and xn to RoundStepProposal.
			cm.doActionCh <- RoundAction{rs.Height, rs.Round, RoundStepProposal}
		case RoundStepProposal:
			// Wake up when it's time to vote.
			time.Sleep(time.Duration(roundDeadlineBare-elapsedRatio) * roundDuration)
			cm.doActionCh <- RoundAction{rs.Height, rs.Round, RoundStepBareVotes}
		case RoundStepBareVotes:
			// Wake up when it's time to precommit.
			time.Sleep(time.Duration(roundDeadlinePrecommit-elapsedRatio) * roundDuration)
			cm.doActionCh <- RoundAction{rs.Height, rs.Round, RoundStepPrecommits}
		case RoundStepPrecommits:
			// Wake up when the round is over.
			time.Sleep(time.Duration(1.0-elapsedRatio) * roundDuration)
			cm.doActionCh <- RoundAction{rs.Height, rs.Round, RoundStepCommitOrUnlock}
		case RoundStepCommitOrUnlock:
			// This shouldn't happen.
			// Before setAlarm() got called,
			// logic should have created a new RoundState for the next round.
			panic("Should not happen")
		}
	}

	for {
		func() {
			roundAction := <-cm.doActionCh
			// Always set the alarm after any processing below.
			defer setAlarm()

			// We only consider acting on given height and round.
			height := roundAction.Height
			round := roundAction.Round
			// We only consider transitioning to given step.
			step := roundAction.XnToStep
			// This is the current state.
			rs := cm.cs.RoundState()
			if height != rs.Height || round != rs.Round {
				return // Not relevant.
			}

			if step == RoundStepProposal && rs.Step() == RoundStepStart {
				// Propose a block if I am the proposer.
				privValidator := cm.PrivValidator()
				if privValidator != nil && rs.Proposer.Account.Id == privValidator.Id {
					// If we're already locked on a proposal, use that.
					proposal := cm.cs.LockedProposal()
					if proposal != nil {
						// Otherwise, construct a new proposal.
						var err error
						proposal, err = cm.constructProposal(rs)
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
					cm.shareProposal(rs)
				}
			} else if step == RoundStepBareVotes && rs.Step() <= RoundStepProposal {
				err := cm.voteProposal(rs)
				if err != nil {
					log.Info("Error attempting to vote for proposal: %v", err)
				}
			} else if step == RoundStepPrecommits && rs.Step() <= RoundStepBareVotes {
				err := cm.precommitProposal(rs)
				if err != nil {
					log.Info("Error attempting to precommit for proposal: %v", err)
				}
			} else if step == RoundStepCommitOrUnlock && rs.Step() <= RoundStepPrecommits {
				err := cm.commitOrUnlockProposal(rs)
				if err != nil {
					log.Info("Error attempting to commit or update for proposal: %v", err)
				}
				// Round is over. This is a special case.
				// Prepare a new RoundState for the next state.
				cm.cs.SetupRound(rs.Round + 1)
				return // setAlarm() takes care of the rest.
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

type PeerState struct {
	mtx                sync.Mutex
	connected          bool
	peer               *p2p.Peer
	height             uint32
	startTime          time.Time // Derived from offset seconds.
	blockPartsBitArray []byte
	votesWanted        map[uint64]float32
	cbHeight           uint32
	cbRound            uint16
	cbFunc             func()
}

func NewPeerState(peer *p2p.Peer) *PeerState {
	return &PeerState{
		connected:   true,
		peer:        peer,
		height:      0,
		votesWanted: make(map[uint64]float32),
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

func (ps *PeerState) WantsVote(vote *Vote) bool {
	if ps == nil {
		return false
	}
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	if !ps.connected {
		return false
	}
	// Only wants the vote if votesWanted says so
	if ps.votesWanted[vote.SignerId] <= 0 {
		// TODO: sometimes, send unsolicited votes to see if peer wants it.
		return false
	}
	// Only wants the vote if peer's current height and round matches.
	if ps.height == vote.Height {
		round, _, _, _, elapsedRatio := calcRoundInfo(ps.startTime)
		if round == vote.Round {
			if vote.Type == VoteTypeBare && elapsedRatio > roundDeadlineBare {
				return false
			}
			if vote.Type == VoteTypePrecommit && elapsedRatio > roundDeadlinePrecommit {
				return false
			}
			return true
		}
	}
	return false
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
	// XXX IMPLEMENT
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
	msgTypeVoteRank        = byte(0x21)
)

// TODO: check for unnecessary extra bytes at the end.
func decodeMessage(bz []byte) (msg interface{}) {
	n, err := new(int64), new(error)
	// log.Debug("decoding msg bytes: %X", bz)
	switch bz[0] {
	case msgTypeBlockPart:
		return readBlockPartMessage(bytes.NewReader(bz[1:]), n, err)
	case msgTypeKnownBlockParts:
		return readKnownBlockPartsMessage(bytes.NewReader(bz[1:]), n, err)
	case msgTypeVote:
		return ReadVote(bytes.NewReader(bz[1:]), n, err)
	case msgTypeVoteRank:
		return readVoteRankMessage(bytes.NewReader(bz[1:]), n, err)
	default:
		return nil
	}
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

// XXX use this.
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
