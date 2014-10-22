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
	"github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/state"
)

const (
	StateCh = byte(0x20)
	DataCh  = byte(0x21)
	VoteCh  = byte(0x22)

	peerStateKey = "ConsensusReactor.peerState"

	voteTypeNil   = byte(0x00)
	voteTypeBlock = byte(0x01)

	roundDuration0         = 60 * time.Second   // The first round is 60 seconds long.
	roundDurationDelta     = 15 * time.Second   // Each successive round lasts 15 seconds longer.
	roundDeadlinePrevote   = float64(1.0 / 3.0) // When the prevote is due.
	roundDeadlinePrecommit = float64(2.0 / 3.0) // When the precommit vote is due.

	finalizeDuration        = roundDuration0 / 3    // The time to wait between commitTime and startTime of next consensus rounds.
	peerGossipSleepDuration = 50 * time.Millisecond // Time to sleep if there's nothing to send.
	hasVotesThreshold       = 50                    // After this many new votes we'll send a HasVotesMessage.
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

type RoundAction struct {
	Height uint32          // The block height for which consensus is reaching for.
	Round  uint16          // The round number at given height.
	Action RoundActionType // Action to perform.
}

type ConsensusReactor struct {
	sw      *p2p.Switch
	quit    chan struct{}
	started uint32
	stopped uint32

	conS       *ConsensusState
	doActionCh chan RoundAction
}

func NewConsensusReactor(sw *p2p.Switch, blockStore *BlockStore, mempool *mempool.Mempool, state *state.State) *ConsensusReactor {
	conS := NewConsensusState(state, blockStore, mempool)
	conR := &ConsensusReactor{
		sw:   sw,
		quit: make(chan struct{}),

		conS:       conS,
		doActionCh: make(chan RoundAction, 1),
	}
	return conR
}

// Sets our private validator account for signing votes.
func (conR *ConsensusReactor) SetPrivValidator(priv *PrivValidator) {
	conR.conS.SetPrivValidator(priv)
}

func (conR *ConsensusReactor) Start() {
	if atomic.CompareAndSwapUint32(&conR.started, 0, 1) {
		log.Info("Starting ConsensusReactor")
		go conR.stepTransitionRoutine()
	}
}

func (conR *ConsensusReactor) Stop() {
	if atomic.CompareAndSwapUint32(&conR.stopped, 0, 1) {
		log.Info("Stopping ConsensusReactor")
		close(conR.quit)
	}
}

func (conR *ConsensusReactor) IsStopped() bool {
	return atomic.LoadUint32(&conR.stopped) == 1
}

// Implements Reactor
func (conR *ConsensusReactor) GetChannels() []*p2p.ChannelDescriptor {
	// TODO optimize
	return []*p2p.ChannelDescriptor{
		&p2p.ChannelDescriptor{
			Id:       StateCh,
			Priority: 5,
		},
		&p2p.ChannelDescriptor{
			Id:       DataCh,
			Priority: 5,
		},
		&p2p.ChannelDescriptor{
			Id:       VoteCh,
			Priority: 5,
		},
	}
}

// Implements Reactor
func (conR *ConsensusReactor) AddPeer(peer *p2p.Peer) {
	// Create peerState for peer
	peerState := NewPeerState(peer)
	peer.Data.Set(peerStateKey, peerState)

	// Begin gossip routines for this peer.
	go conR.gossipDataRoutine(peer, peerState)
	go conR.gossipVotesRoutine(peer, peerState)
}

// Implements Reactor
func (conR *ConsensusReactor) RemovePeer(peer *p2p.Peer, reason interface{}) {
	//peer.Data.Get(peerStateKey).(*PeerState).Disconnect()
}

// Implements Reactor
func (conR *ConsensusReactor) Receive(chId byte, peer *p2p.Peer, msgBytes []byte) {

	// Get round state
	rs := conR.conS.GetRoundState()
	ps := peer.Data.Get(peerStateKey).(*PeerState)
	_, msg_ := decodeMessage(msgBytes)
	voteAddCounter := 0
	var err error = nil

	switch chId {
	case StateCh:
		switch msg_.(type) {
		case *NewRoundStepMessage:
			msg := msg_.(*NewRoundStepMessage)
			err = ps.ApplyNewRoundStepMessage(msg)

		case *HasVotesMessage:
			msg := msg_.(*HasVotesMessage)
			err = ps.ApplyHasVotesMessage(msg)

		default:
			// Ignore unknown message
		}

	case DataCh:
		switch msg_.(type) {
		case *Proposal:
			proposal := msg_.(*Proposal)
			ps.SetHasProposal(proposal.Height, proposal.Round)
			err = conR.conS.SetProposal(proposal)

		case *PartMessage:
			msg := msg_.(*PartMessage)
			if msg.Type == partTypeProposalBlock {
				ps.SetHasProposalBlockPart(msg.Height, msg.Round, msg.Part.Index)
				_, err = conR.conS.AddProposalBlockPart(msg.Height, msg.Round, msg.Part)
			} else if msg.Type == partTypeProposalPOL {
				ps.SetHasProposalPOLPart(msg.Height, msg.Round, msg.Part.Index)
				_, err = conR.conS.AddProposalPOLPart(msg.Height, msg.Round, msg.Part)
			} else {
				// Ignore unknown part type
			}

		default:
			// Ignore unknown message
		}

	case VoteCh:
		switch msg_.(type) {
		case *Vote:
			vote := msg_.(*Vote)
			// We can't deal with votes from another height,
			// as they have a different validator set.
			if vote.Height != rs.Height || vote.Height != ps.Height {
				return
			}
			index, val := rs.Validators.GetById(vote.SignerId)
			if val == nil {
				log.Warning("Peer gave us an invalid vote.")
				return
			}
			ps.SetHasVote(rs.Height, rs.Round, vote.Type, uint32(index))
			added, err := conR.conS.AddVote(vote)
			if err != nil {
				log.Warning("Error attempting to add vote: %v", err)
			}
			if added {
				// Maybe send HasVotesMessage
				// TODO optimize. It would be better to just acks for each vote!
				voteAddCounter++
				if voteAddCounter%hasVotesThreshold == 0 {
					msg := &HasVotesMessage{
						Height:     rs.Height,
						Round:      rs.Round,
						Prevotes:   rs.Prevotes.BitArray(),
						Precommits: rs.Precommits.BitArray(),
						Commits:    rs.Commits.BitArray(),
					}
					conR.sw.Broadcast(StateCh, msg)
				}
				// Maybe run RoundActionCommitWait.
				if vote.Type == VoteTypeCommit &&
					rs.Commits.HasTwoThirdsMajority() &&
					rs.Step < RoundStepCommit {
					// NOTE: Do not call RunAction*() methods here directly.
					conR.doActionCh <- RoundAction{rs.Height, rs.Round, RoundActionCommitWait}
				}
			}

		default:
			// Ignore unknown message
		}
	default:
		// Ignore unknown channel
	}

	if err != nil {
		log.Warning("Error in Receive(): %v", err)
	}
}

//--------------------------------------

// Source of all round state transitions (and votes).
func (conR *ConsensusReactor) stepTransitionRoutine() {

	// Schedule the next action by pushing a RoundAction{} to conR.doActionCh
	// when it is due.
	scheduleNextAction := func() {
		rs := conR.conS.GetRoundState()
		_, _, roundDuration, _, elapsedRatio := calcRoundInfo(rs.StartTime)
		go func() {
			switch rs.Step {
			case RoundStepStart:
				// It's a new RoundState.
				if elapsedRatio < 0 {
					// startTime is in the future.
					time.Sleep(time.Duration(-1.0*elapsedRatio) * roundDuration)
				}
				conR.doActionCh <- RoundAction{rs.Height, rs.Round, RoundActionPropose}
			case RoundStepPropose:
				// Wake up when it's time to vote.
				time.Sleep(time.Duration(roundDeadlinePrevote-elapsedRatio) * roundDuration)
				conR.doActionCh <- RoundAction{rs.Height, rs.Round, RoundActionPrevote}
			case RoundStepPrevote:
				// Wake up when it's time to precommit.
				time.Sleep(time.Duration(roundDeadlinePrecommit-elapsedRatio) * roundDuration)
				conR.doActionCh <- RoundAction{rs.Height, rs.Round, RoundActionPrecommit}
			case RoundStepPrecommit:
				// Wake up when the round is over.
				time.Sleep(time.Duration(1.0-elapsedRatio) * roundDuration)
				conR.doActionCh <- RoundAction{rs.Height, rs.Round, RoundActionNextRound}
			case RoundStepCommit:
				panic("Should not happen: RoundStepCommit waits until +2/3 commits.")
			case RoundStepCommitWait:
				// Wake up when it's time to finalize commit.
				if rs.CommitTime.IsZero() {
					panic("RoundStepCommitWait requires rs.CommitTime")
				}
				time.Sleep(rs.CommitTime.Sub(time.Now()) + finalizeDuration)
				conR.doActionCh <- RoundAction{rs.Height, rs.Round, RoundActionFinalize}
			default:
				panic("Should not happen")
			}
		}()
	}

	scheduleNextAction()

	// NOTE: All ConsensusState.RunAction*() calls must come from here.
	// Since only one routine calls them, it is safe to assume that
	// the RoundState Height/Round/Step won't change concurrently.
	// However, other fields like Proposal could change, due to gossip.
ACTION_LOOP:
	for {
		roundAction := <-conR.doActionCh

		height := roundAction.Height
		round := roundAction.Round
		action := roundAction.Action
		rs := conR.conS.GetRoundState()
		broadcastNewRoundStep := func(step RoundStep) {
			// Broadcast NewRoundStepMessage
			msg := &NewRoundStepMessage{
				Height: height,
				Round:  round,
				Step:   step,
				SecondsSinceStartTime: uint32(rs.RoundElapsed().Seconds()),
			}
			conR.sw.Broadcast(StateCh, msg)
		}

		// Continue if action is not relevant
		if height != rs.Height {
			continue
		}
		// If action >= RoundActionCommit, the round doesn't matter.
		if action < RoundActionCommit && round != rs.Round {
			continue
		}

		// Run action
		switch action {
		case RoundActionPropose:
			if rs.Step != RoundStepStart {
				continue ACTION_LOOP
			}
			conR.conS.RunActionPropose(rs.Height, rs.Round)
			broadcastNewRoundStep(RoundStepPropose)
			scheduleNextAction()
			continue ACTION_LOOP

		case RoundActionPrevote:
			if rs.Step >= RoundStepPrevote {
				continue ACTION_LOOP
			}
			hash := conR.conS.RunActionPrevote(rs.Height, rs.Round)
			broadcastNewRoundStep(RoundStepPrevote)
			conR.signAndBroadcastVote(rs, &Vote{
				Height:    rs.Height,
				Round:     rs.Round,
				Type:      VoteTypePrevote,
				BlockHash: hash,
			})
			scheduleNextAction()
			continue ACTION_LOOP

		case RoundActionPrecommit:
			if rs.Step >= RoundStepPrecommit {
				continue ACTION_LOOP
			}
			hash := conR.conS.RunActionPrecommit(rs.Height, rs.Round)
			broadcastNewRoundStep(RoundStepPrecommit)
			if len(hash) > 0 {
				conR.signAndBroadcastVote(rs, &Vote{
					Height:    rs.Height,
					Round:     rs.Round,
					Type:      VoteTypePrecommit,
					BlockHash: hash,
				})
			}
			scheduleNextAction()
			continue ACTION_LOOP

		case RoundActionNextRound:
			if rs.Step >= RoundStepCommit {
				continue ACTION_LOOP
			}
			conR.conS.SetupRound(rs.Round + 1)
			scheduleNextAction()
			continue ACTION_LOOP

		case RoundActionCommit:
			if rs.Step >= RoundStepCommit {
				continue ACTION_LOOP
			}
			// NOTE: Duplicated in RoundActionCommitWait.
			hash := conR.conS.RunActionCommit(rs.Height)
			if len(hash) > 0 {
				broadcastNewRoundStep(RoundStepCommit)
				conR.signAndBroadcastVote(rs, &Vote{
					Height:    rs.Height,
					Round:     rs.Round,
					Type:      VoteTypeCommit,
					BlockHash: hash,
				})
			} else {
				panic("This shouldn't happen")
			}
			// do not schedule next action.
			continue ACTION_LOOP

		case RoundActionCommitWait:
			if rs.Step >= RoundStepCommitWait {
				continue ACTION_LOOP
			}
			// Commit first we haven't already.
			if rs.Step < RoundStepCommit {
				// NOTE: Duplicated in RoundActionCommit.
				hash := conR.conS.RunActionCommit(rs.Height)
				if len(hash) > 0 {
					broadcastNewRoundStep(RoundStepCommit)
					conR.signAndBroadcastVote(rs, &Vote{
						Height:    rs.Height,
						Round:     rs.Round,
						Type:      VoteTypeCommit,
						BlockHash: hash,
					})
				} else {
					panic("This shouldn't happen")
				}
			}
			// Wait for more commit votes.
			conR.conS.RunActionCommitWait(rs.Height)
			scheduleNextAction()
			continue ACTION_LOOP

		case RoundActionFinalize:
			if rs.Step != RoundStepCommitWait {
				panic("This shouldn't happen")
			}
			conR.conS.RunActionFinalize(rs.Height)
			// Height has been incremented, step is now RoundStepStart.
			scheduleNextAction()
			continue ACTION_LOOP

		default:
			panic("Unknown action")
		}

		// For clarity, ensure that all switch cases call "continue"
		panic("Should not happen.")
	}
}

func (conR *ConsensusReactor) signAndBroadcastVote(rs *RoundState, vote *Vote) {
	if rs.PrivValidator != nil {
		rs.PrivValidator.Sign(vote)
		conR.conS.AddVote(vote)
		msg := p2p.TypedMessage{msgTypeVote, vote}
		conR.sw.Broadcast(VoteCh, msg)
	}
}

//--------------------------------------

func (conR *ConsensusReactor) gossipDataRoutine(peer *p2p.Peer, ps *PeerState) {

OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if peer.IsStopped() || conR.IsStopped() {
			log.Info("Stopping gossipDataRoutine for %v.", peer)
			return
		}
		rs := conR.conS.GetRoundState()
		prs := ps.GetRoundState()

		// If ProposalBlockHash matches, send parts?
		// NOTE: if we or peer is at RoundStepCommit*, the round
		// won't necessarily match, but that's OK.
		if rs.ProposalBlock.HashesTo(prs.ProposalBlockHash) {
			if index, ok := rs.ProposalBlockPartSet.BitArray().Sub(
				prs.ProposalBlockBitArray).PickRandom(); ok {
				msg := &PartMessage{
					Height: rs.Height,
					Round:  rs.Round,
					Type:   partTypeProposalBlock,
					Part:   rs.ProposalBlockPartSet.GetPart(uint16(index)),
				}
				peer.Send(DataCh, msg)
				ps.SetHasProposalBlockPart(rs.Height, rs.Round, uint16(index))
				continue OUTER_LOOP
			}
		}

		// If height and round doesn't match, sleep.
		if rs.Height != prs.Height || rs.Round != prs.Round {
			time.Sleep(peerGossipSleepDuration)
			continue OUTER_LOOP
		}

		// Send proposal?
		if rs.Proposal != nil && !prs.Proposal {
			msg := p2p.TypedMessage{msgTypeProposal, rs.Proposal}
			peer.Send(DataCh, msg)
			ps.SetHasProposal(rs.Height, rs.Round)
			continue OUTER_LOOP
		}

		// Send proposal POL part?
		if index, ok := rs.ProposalPOLPartSet.BitArray().Sub(
			prs.ProposalPOLBitArray).PickRandom(); ok {
			msg := &PartMessage{
				Height: rs.Height,
				Round:  rs.Round,
				Type:   partTypeProposalPOL,
				Part:   rs.ProposalPOLPartSet.GetPart(uint16(index)),
			}
			peer.Send(DataCh, msg)
			ps.SetHasProposalPOLPart(rs.Height, rs.Round, uint16(index))
			continue OUTER_LOOP
		}

		// Nothing to do. Sleep.
		time.Sleep(peerGossipSleepDuration)
		continue OUTER_LOOP
	}
}

func (conR *ConsensusReactor) gossipVotesRoutine(peer *p2p.Peer, ps *PeerState) {
OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if peer.IsStopped() || conR.IsStopped() {
			log.Info("Stopping gossipVotesRoutine for %v.", peer)
			return
		}
		rs := conR.conS.GetRoundState()
		prs := ps.GetRoundState()

		// If height doesn't match, sleep.
		if rs.Height != prs.Height {
			time.Sleep(peerGossipSleepDuration)
			continue OUTER_LOOP
		}

		// If there are prevotes to send...
		if prs.Step <= RoundStepPrevote {
			index, ok := rs.Prevotes.BitArray().Sub(prs.Prevotes).PickRandom()
			if ok {
				valId, val := rs.Validators.GetByIndex(uint32(index))
				if val != nil {
					vote := rs.Prevotes.Get(valId)
					msg := p2p.TypedMessage{msgTypeVote, vote}
					peer.Send(VoteCh, msg)
					ps.SetHasVote(rs.Height, rs.Round, VoteTypePrevote, uint32(index))
					if vote.Type == VoteTypeCommit {
						ps.SetHasVote(rs.Height, rs.Round, VoteTypePrecommit, uint32(index))
						ps.SetHasVote(rs.Height, rs.Round, VoteTypeCommit, uint32(index))
					}
					continue OUTER_LOOP
				} else {
					log.Error("index is not a valid validator index")
				}
			}
		}

		// If there are precommits to send...
		if prs.Step <= RoundStepPrecommit {
			index, ok := rs.Precommits.BitArray().Sub(prs.Precommits).PickRandom()
			if ok {
				valId, val := rs.Validators.GetByIndex(uint32(index))
				if val != nil {
					vote := rs.Precommits.Get(valId)
					msg := p2p.TypedMessage{msgTypeVote, vote}
					peer.Send(VoteCh, msg)
					ps.SetHasVote(rs.Height, rs.Round, VoteTypePrecommit, uint32(index))
					if vote.Type == VoteTypeCommit {
						ps.SetHasVote(rs.Height, rs.Round, VoteTypeCommit, uint32(index))
					}
					continue OUTER_LOOP
				} else {
					log.Error("index is not a valid validator index")
				}
			}
		}

		// If there are any commits to send...
		index, ok := rs.Commits.BitArray().Sub(prs.Commits).PickRandom()
		if ok {
			valId, val := rs.Validators.GetByIndex(uint32(index))
			if val != nil {
				vote := rs.Commits.Get(valId)
				msg := p2p.TypedMessage{msgTypeVote, vote}
				peer.Send(VoteCh, msg)
				ps.SetHasVote(rs.Height, rs.Round, VoteTypeCommit, uint32(index))
				continue OUTER_LOOP
			} else {
				log.Error("index is not a valid validator index")
			}
		}

		// We sent nothing. Sleep...
		time.Sleep(peerGossipSleepDuration)
		continue OUTER_LOOP
	}
}

//-----------------------------------------------------------------------------

// Read only when returned by PeerState.GetRoundState().
type PeerRoundState struct {
	Height                uint32    // Height peer is at
	Round                 uint16    // Round peer is at
	Step                  RoundStep // Step peer is at
	StartTime             time.Time // Estimated start of round 0 at this height
	Proposal              bool      // True if peer has proposal for this round
	ProposalBlockHash     []byte    // Block parts merkle root
	ProposalBlockBitArray BitArray  // Block parts bitarray
	ProposalPOLHash       []byte    // POL parts merkle root
	ProposalPOLBitArray   BitArray  // POL parts bitarray
	Prevotes              BitArray  // All votes peer has for this round
	Precommits            BitArray  // All precommits peer has for this round
	Commits               BitArray  // All commits peer has for this height
}

//-----------------------------------------------------------------------------

var (
	ErrPeerStateHeightRegression = errors.New("Error peer state height regression")
	ErrPeerStateInvalidStartTime = errors.New("Error peer state invalid startTime")
)

type PeerState struct {
	mtx sync.Mutex
	PeerRoundState
}

func NewPeerState(peer *p2p.Peer) *PeerState {
	return &PeerState{}
}

// Returns an atomic snapshot of the PeerRoundState.
// There's no point in mutating it since it won't change PeerState.
func (ps *PeerState) GetRoundState() *PeerRoundState {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	prs := ps.PeerRoundState // copy
	return &prs
}

func (ps *PeerState) SetHasProposal(height uint32, round uint16) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height == height && ps.Round == round {
		ps.Proposal = true
	}
}

func (ps *PeerState) SetHasProposalBlockPart(height uint32, round uint16, index uint16) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height == height && ps.Round == round {
		ps.ProposalBlockBitArray.SetIndex(uint(index), true)
	}
}

func (ps *PeerState) SetHasProposalPOLPart(height uint32, round uint16, index uint16) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	if ps.Height == height && ps.Round == round {
		ps.ProposalPOLBitArray.SetIndex(uint(index), true)
	}
}

func (ps *PeerState) SetHasVote(height uint32, round uint16, type_ uint8, index uint32) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	if ps.Height == height && (ps.Round == round || type_ == VoteTypeCommit) {
		switch type_ {
		case VoteTypePrevote:
			ps.Prevotes.SetIndex(uint(index), true)
		case VoteTypePrecommit:
			ps.Precommits.SetIndex(uint(index), true)
		case VoteTypeCommit:
			ps.Commits.SetIndex(uint(index), true)
		default:
			panic("Invalid vote type")
		}
	}
}

func (ps *PeerState) ApplyNewRoundStepMessage(msg *NewRoundStepMessage) error {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	// Set step state
	startTime := time.Now().Add(-1 * time.Duration(msg.SecondsSinceStartTime) * time.Second)
	ps.Height = msg.Height
	ps.Round = msg.Round
	ps.Step = msg.Step
	ps.StartTime = startTime

	// Reset the rest
	ps.Proposal = false
	ps.ProposalBlockHash = nil
	ps.ProposalBlockBitArray = nil
	ps.ProposalPOLHash = nil
	ps.ProposalPOLBitArray = nil
	ps.Prevotes = nil
	ps.Precommits = nil
	if ps.Height != msg.Height {
		ps.Commits = nil
	}
	return nil
}

func (ps *PeerState) ApplyHasVotesMessage(msg *HasVotesMessage) error {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	if ps.Height == msg.Height {
		ps.Commits = ps.Commits.Or(msg.Commits)
		if ps.Round == msg.Round {
			ps.Prevotes = ps.Prevotes.Or(msg.Prevotes)
			ps.Precommits = ps.Precommits.Or(msg.Precommits)
		} else {
			ps.Prevotes = msg.Prevotes
			ps.Precommits = msg.Precommits
		}
	}
	return nil
}

//-----------------------------------------------------------------------------
// Messages

const (
	msgTypeUnknown = byte(0x00)
	// Messages for communicating state changes
	msgTypeNewRoundStep = byte(0x01)
	msgTypeHasVotes     = byte(0x02)
	// Messages of data
	msgTypeProposal = byte(0x11)
	msgTypePart     = byte(0x12) // both block & POL
	msgTypeVote     = byte(0x13)
)

// TODO: check for unnecessary extra bytes at the end.
func decodeMessage(bz []byte) (msgType byte, msg interface{}) {
	n, err := new(int64), new(error)
	// log.Debug("decoding msg bytes: %X", bz)
	msgType = bz[0]
	r := bytes.NewReader(bz[1:])
	switch msgType {
	// Messages for communicating state changes
	case msgTypeNewRoundStep:
		msg = readNewRoundStepMessage(r, n, err)
	case msgTypeHasVotes:
		msg = readHasVotesMessage(r, n, err)
	// Messages of data
	case msgTypeProposal:
		msg = ReadProposal(r, n, err)
	case msgTypePart:
		msg = readPartMessage(r, n, err)
	case msgTypeVote:
		msg = ReadVote(r, n, err)
	default:
		msg = nil
	}
	return
}

//-------------------------------------

type NewRoundStepMessage struct {
	Height                uint32
	Round                 uint16
	Step                  RoundStep
	SecondsSinceStartTime uint32
}

func readNewRoundStepMessage(r io.Reader, n *int64, err *error) *NewRoundStepMessage {
	return &NewRoundStepMessage{
		Height: ReadUInt32(r, n, err),
		Round:  ReadUInt16(r, n, err),
		Step:   RoundStep(ReadUInt8(r, n, err)),
		SecondsSinceStartTime: ReadUInt32(r, n, err),
	}
}

func (m *NewRoundStepMessage) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, msgTypeNewRoundStep, &n, &err)
	WriteUInt32(w, m.Height, &n, &err)
	WriteUInt16(w, m.Round, &n, &err)
	WriteUInt8(w, uint8(m.Step), &n, &err)
	WriteUInt32(w, m.SecondsSinceStartTime, &n, &err)
	return
}

func (m *NewRoundStepMessage) String() string {
	return fmt.Sprintf("[NewRoundStepMessage H:%v R:%v]", m.Height, m.Round)
}

//-------------------------------------

type HasVotesMessage struct {
	Height     uint32
	Round      uint16
	Prevotes   BitArray
	Precommits BitArray
	Commits    BitArray
}

func readHasVotesMessage(r io.Reader, n *int64, err *error) *HasVotesMessage {
	return &HasVotesMessage{
		Height:     ReadUInt32(r, n, err),
		Round:      ReadUInt16(r, n, err),
		Prevotes:   ReadBitArray(r, n, err),
		Precommits: ReadBitArray(r, n, err),
		Commits:    ReadBitArray(r, n, err),
	}
}

func (m *HasVotesMessage) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, msgTypeHasVotes, &n, &err)
	WriteUInt32(w, m.Height, &n, &err)
	WriteUInt16(w, m.Round, &n, &err)
	WriteBinary(w, m.Prevotes, &n, &err)
	WriteBinary(w, m.Precommits, &n, &err)
	WriteBinary(w, m.Commits, &n, &err)
	return
}

func (m *HasVotesMessage) String() string {
	return fmt.Sprintf("[HasVotesMessage H:%v R:%v]", m.Height, m.Round)
}

//-------------------------------------

const (
	partTypeProposalBlock = byte(0x01)
	partTypeProposalPOL   = byte(0x02)
)

type PartMessage struct {
	Height uint32
	Round  uint16
	Type   byte
	Part   *Part
}

func readPartMessage(r io.Reader, n *int64, err *error) *PartMessage {
	return &PartMessage{
		Height: ReadUInt32(r, n, err),
		Round:  ReadUInt16(r, n, err),
		Type:   ReadByte(r, n, err),
		Part:   ReadPart(r, n, err),
	}
}

func (m *PartMessage) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, msgTypePart, &n, &err)
	WriteUInt32(w, m.Height, &n, &err)
	WriteUInt16(w, m.Round, &n, &err)
	WriteByte(w, m.Type, &n, &err)
	WriteBinary(w, m.Part, &n, &err)
	return
}

func (m *PartMessage) String() string {
	return fmt.Sprintf("[PartMessage H:%v R:%v T:%X]", m.Height, m.Round, m.Type)
}
