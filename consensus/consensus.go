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

	. "github.com/tendermint/tendermint/accounts"
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/p2p"
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

// convenience
func calcRoundInfo(startTime time.Time) (round uint16, roundStartTime time.Time, roundDuration time.Duration, roundElapsed time.Duration, elapsedRatio float64) {
	round = calcRound(startTime)
	roundStartTime = calcRoundStartTime(round, startTime)
	roundDuration = calcRoundDuration(round)
	roundElapsed = time.Now().Sub(roundStartTime)
	elapsedRatio = float64(roundElapsed) / float64(roundDuration)
	return
}

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

//-----------------------------------------------------------------------------

type ConsensusManager struct {
	sw       *p2p.Switch
	swEvents chan interface{}
	quit     chan struct{}
	started  uint32
	stopped  uint32

	csc          *ConsensusStateControl
	blockStore   *BlockStore
	accountStore *AccountStore
	mtx          sync.Mutex
	peerStates   map[string]*PeerState
	doActionCh   chan RoundAction
}

func NewConsensusManager(sw *p2p.Switch, csc *ConsensusStateControl, blockStore *BlockStore, accountStore *AccountStore) *ConsensusManager {
	swEvents := make(chan interface{})
	sw.AddEventListener("ConsensusManager.swEvents", swEvents)
	csc.Update(blockStore) // Update csc with new blocks.
	cm := &ConsensusManager{
		sw:           sw,
		swEvents:     swEvents,
		quit:         make(chan struct{}),
		csc:          csc,
		blockStore:   blockStore,
		accountStore: accountStore,
		peerStates:   make(map[string]*PeerState),
		doActionCh:   make(chan RoundAction, 1),
	}
	return cm
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
			event.Peer.TrySend(ProposalCh, cm.makeKnownBlockPartsMessage())
		case p2p.SwitchEventDonePeer:
			event := swEvent.(p2p.SwitchEventDonePeer)
			// Delete peerState for event.Peer
			cm.mtx.Lock()
			delete(cm.peerStates, event.Peer.Key)
			cm.mtx.Unlock()
		default:
			log.Warning("Unhandled switch event type")
		}
	}
}

// Like, how large is it and how often can we send it?
func (cm *ConsensusManager) makeKnownBlockPartsMessage() *KnownBlockPartsMessage {
	rs := cm.csc.RoundState()
	return &KnownBlockPartsMessage{
		Height:                UInt32(rs.Height),
		SecondsSinceStartTime: UInt32(time.Now().Sub(rs.StartTime).Seconds()),
		BlockPartsBitArray:    rs.BlockPartSet.BitArray(),
	}
}

func (cm *ConsensusManager) getPeerState(peer *p2p.Peer) *PeerState {
	cm.mtx.Lock()
	defer cm.mtx.Unlock()
	peerState := cm.peerStates[peer.Key]
	if peerState == nil {
		log.Warning("Wanted peerState for %v but none exists", peer)
	}
	return peerState
}

func (cm *ConsensusManager) gossipProposalRoutine() {
OUTER_LOOP:
	for {
		// Get round state
		rs := cm.csc.RoundState()

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
			if uint32(msg.BlockPart.Height) == rs.Height &&
				uint16(msg.BlockPart.Round) == rs.Round {

				// TODO Continue if we've already voted, then no point processing the part.

				// Add and process the block part
				added, err := rs.BlockPartSet.AddBlockPart(msg.BlockPart)
				if err == ErrInvalidBlockPartConflict {
					// TODO: Bad validator
				} else if err == ErrInvalidBlockPartSignature {
					// TODO: Bad peer
				} else if err != nil {
					Panicf("Unexpected blockPartsSet error %v", err)
				}
				if added {
					// If peer wants this part, send peer the part
					// and our new blockParts state.
					kbpMsg := cm.makeKnownBlockPartsMessage()
					partMsg := &BlockPartMessage{BlockPart: msg.BlockPart}
				PEERS_LOOP:
					for _, peer := range cm.sw.Peers().List() {
						peerState := cm.getPeerState(peer)
						if peerState == nil {
							// Peer disconnected before we were able to process.
							continue PEERS_LOOP
						}
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
		if peerState == nil {
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
	privValidator := cm.csc.PrivValidator()
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

func (cm *ConsensusManager) isProposalValid(rs *RoundState) bool {
	if !rs.BlockPartSet.IsComplete() {
		return false
	}
	err := cm.stageBlock(rs.BlockPartSet)
	if err != nil {
		return false
	}
	return true
}

func (cm *ConsensusManager) constructProposal(rs *RoundState) (*Block, error) {
	// XXX implement
	return nil, nil
}

// Vote for (or against) the proposal for this round.
// Call during transition from RoundStepProposal to RoundStepVote.
// We may not have received a full proposal.
func (cm *ConsensusManager) voteProposal(rs *RoundState) error {
	// If we're locked, must vote that.
	locked := cm.csc.LockedProposal()
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
	// If proposal is invalid
	if !cm.isProposalValid(rs) {
		// Vote for nil.
		err := cm.signAndVote(&Vote{
			Height: rs.Height,
			Round:  rs.Round,
			Type:   VoteTypeBare,
			Hash:   nil,
		})
		return err
	}
	// Vote for block.
	err := cm.signAndVote(&Vote{
		Height: rs.Height,
		Round:  rs.Round,
		Type:   VoteTypeBare,
		Hash:   rs.BlockPartSet.Block().Hash(),
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
			if !cm.isProposalValid(rs) {
				return nil
			}

			// Lock this proposal.
			// NOTE: we're unlocking any prior locks.
			cm.csc.LockProposal(rs.BlockPartSet)

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
	if hash, ok := rs.RoundPrecommits.TwoThirdsMajority(); ok {
		// If there exists a 2/3 majority of precommits.
		// Validate the block and commit.

		// If the proposal is invalid or we don't have it,
		// do not commit.
		// TODO If we were just late to receive the block, when
		// do we actually get it? Document it.
		if !cm.isProposalValid(rs) {
			return nil
		}
		// TODO: Remove?
		cm.csc.LockProposal(rs.BlockPartSet)
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
		cm.commitBlock(rs.BlockPartSet, commitTime)
		return nil
	} else {
		// Otherwise, if a 1/3 majority if a block that isn't our locked one exists, unlock.
		locked := cm.csc.LockedProposal()
		if locked != nil {
			for _, hashOrNil := range rs.RoundPrecommits.OneThirdMajority() {
				if hashOrNil == nil {
					continue
				}
				hash := hashOrNil.([]byte)
				if !bytes.Equal(hash, locked.Block().Hash()) {
					// Unlock our lock.
					cm.csc.LockProposal(nil)
				}
			}
		}
		return nil
	}
}

// After stageBlock(), a call to commitBlock() with the same arguments must succeed.
func (cm *ConsensusManager) stageBlock(blockPartSet *BlockPartSet) error {
	cm.mtx.Lock()
	defer cm.mtx.Unlock()

	block, blockParts := blockPartSet.Block(), blockPartSet.BlockParts()

	err := block.ValidateBasic()
	if err != nil {
		return err
	}
	err = cm.blockStore.StageBlockAndParts(block, blockParts)
	if err != nil {
		return err
	}
	err = cm.csc.StageBlock(block)
	if err != nil {
		return err
	}
	err = cm.accountStore.StageBlock(block)
	if err != nil {
		return err
	}
	// NOTE: more stores may be added here for validation.
	return nil
}

// after stageBlock(), a call to commitBlock() with the same arguments must succeed.
func (cm *ConsensusManager) commitBlock(blockPartSet *BlockPartSet, commitTime time.Time) error {
	cm.mtx.Lock()
	defer cm.mtx.Unlock()

	block, blockParts := blockPartSet.Block(), blockPartSet.BlockParts()

	err := cm.blockStore.SaveBlockParts(uint32(block.Height), blockParts)
	if err != nil {
		return err
	}
	err = cm.csc.CommitBlock(block, commitTime)
	if err != nil {
		return err
	}
	err = cm.accountStore.CommitBlock(block)
	if err != nil {
		return err
	}
	return nil
}

func (cm *ConsensusManager) gossipVoteRoutine() {
OUTER_LOOP:
	for {
		// Get round state
		rs := cm.csc.RoundState()

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
		PEERS_LOOP:
			for _, peer := range cm.sw.Peers().List() {
				peerState := cm.getPeerState(peer)
				if peerState == nil {
					// Peer disconnected before we were able to process.
					continue PEERS_LOOP
				}
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
		rs := cm.csc.RoundState()
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
			rs := cm.csc.RoundState()
			if height != rs.Height || round != rs.Round {
				return // Not relevant.
			}

			if step == RoundStepProposal && rs.Step() == RoundStepStart {
				// Propose a block if I am the proposer.
				privValidator := cm.csc.PrivValidator()
				if privValidator != nil && rs.Proposer.Account.Id == privValidator.Id {
					block, err := cm.constructProposal(rs)
					if err != nil {
						log.Error("Error attempting to construct a proposal: %v", err)
					}
					// XXX propose the block.
					log.Error("XXX use ", block)
					// XXX divide block into parts
					// XXX communicate parts.
					// XXX put this in another function.
					panic("Implement block proposal!")
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
				cm.csc.SetupRound(rs.Round + 1)
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
	peer               *p2p.Peer
	height             uint32
	startTime          time.Time // Derived from offset seconds.
	blockPartsBitArray []byte
	votesWanted        map[uint64]float32
}

func NewPeerState(peer *p2p.Peer) *PeerState {
	return &PeerState{
		peer:        peer,
		height:      0,
		votesWanted: make(map[uint64]float32),
	}
}

func (ps *PeerState) WantsBlockPart(part *BlockPart) bool {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	// Only wants the part if peer's current height and round matches.
	if ps.height == uint32(part.Height) {
		round, _, _, _, elapsedRatio := calcRoundInfo(ps.startTime)
		if round == uint16(part.Round) && elapsedRatio < roundDeadlineBare {
			// Only wants the part if it doesn't already have it.
			if ps.blockPartsBitArray[part.Index/8]&byte(1<<(part.Index%8)) == 0 {
				return true
			}
		}
	}
	return false
}

func (ps *PeerState) WantsVote(vote *Vote) bool {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	// Only wants the vote if votesWanted says so
	if ps.votesWanted[uint64(vote.SignerId)] <= 0 {
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
	if uint32(msg.Height) < ps.height {
		return ErrPeerStateHeightRegression
	}
	if uint32(msg.Height) == ps.height {
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
		ps.height = uint32(msg.Height)
		ps.blockPartsBitArray = msg.BlockPartsBitArray
	}
	return nil
}

func (ps *PeerState) ApplyVoteRankMessage(msg *VoteRankMessage) error {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	// XXX IMPLEMENT
	return nil
}

//-----------------------------------------------------------------------------
// Messages

const (
	msgTypeUnknown         = Byte(0x00)
	msgTypeBlockPart       = Byte(0x10)
	msgTypeKnownBlockParts = Byte(0x11)
	msgTypeVote            = Byte(0x20)
	msgTypeVoteRank        = Byte(0x21)
)

// TODO: check for unnecessary extra bytes at the end.
func decodeMessage(bz ByteSlice) (msg interface{}) {
	// log.Debug("decoding msg bytes: %X", bz)
	switch Byte(bz[0]) {
	case msgTypeBlockPart:
		return readBlockPartMessage(bytes.NewReader(bz[1:]))
	case msgTypeKnownBlockParts:
		return readKnownBlockPartsMessage(bytes.NewReader(bz[1:]))
	case msgTypeVote:
		return ReadVote(bytes.NewReader(bz[1:]))
	case msgTypeVoteRank:
		return readVoteRankMessage(bytes.NewReader(bz[1:]))
	default:
		return nil
	}
}

//-------------------------------------

type BlockPartMessage struct {
	BlockPart *BlockPart
}

func readBlockPartMessage(r io.Reader) *BlockPartMessage {
	return &BlockPartMessage{
		BlockPart: ReadBlockPart(r),
	}
}

func (m *BlockPartMessage) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(msgTypeBlockPart, w, n, err)
	n, err = WriteTo(m.BlockPart, w, n, err)
	return
}

func (m *BlockPartMessage) String() string {
	return fmt.Sprintf("[BlockPartMessage %v]", m.BlockPart)
}

//-------------------------------------

type KnownBlockPartsMessage struct {
	Height                UInt32
	SecondsSinceStartTime UInt32
	BlockPartsBitArray    ByteSlice
}

func readKnownBlockPartsMessage(r io.Reader) *KnownBlockPartsMessage {
	return &KnownBlockPartsMessage{
		Height:                ReadUInt32(r),
		SecondsSinceStartTime: ReadUInt32(r),
		BlockPartsBitArray:    ReadByteSlice(r),
	}
}

func (m *KnownBlockPartsMessage) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(msgTypeKnownBlockParts, w, n, err)
	n, err = WriteTo(m.Height, w, n, err)
	n, err = WriteTo(m.SecondsSinceStartTime, w, n, err)
	n, err = WriteTo(m.BlockPartsBitArray, w, n, err)
	return
}

func (m *KnownBlockPartsMessage) String() string {
	return fmt.Sprintf("[KnownBlockPartsMessage H:%v SSST:%v, BPBA:%X]",
		m.Height, m.SecondsSinceStartTime, m.BlockPartsBitArray)
}

//-------------------------------------

// XXX use this.
type VoteRankMessage struct {
	ValidatorId UInt64
	Rank        UInt8
}

func readVoteRankMessage(r io.Reader) *VoteRankMessage {
	return &VoteRankMessage{
		ValidatorId: ReadUInt64(r),
		Rank:        ReadUInt8(r),
	}
}

func (m *VoteRankMessage) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(msgTypeVoteRank, w, n, err)
	n, err = WriteTo(m.ValidatorId, w, n, err)
	n, err = WriteTo(m.Rank, w, n, err)
	return
}

func (m *VoteRankMessage) String() string {
	return fmt.Sprintf("[VoteRankMessage V:%v, R:%v]", m.ValidatorId, m.Rank)
}
