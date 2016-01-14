package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-events"
	"github.com/tendermint/go-wire"
	bc "github.com/tendermint/tendermint/blockchain"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

// TODO configurable!
var (
	timeoutPropose        = 3000 * time.Millisecond // Maximum duration of RoundStepPropose
	timeoutPrevote0       = 1000 * time.Millisecond // After any +2/3 prevotes received, wait this long for stragglers.
	timeoutPrevoteDelta   = 0500 * time.Millisecond // timeoutPrevoteN is timeoutPrevote0 + timeoutPrevoteDelta*N
	timeoutPrecommit0     = 1000 * time.Millisecond // After any +2/3 precommits received, wait this long for stragglers.
	timeoutPrecommitDelta = 0500 * time.Millisecond // timeoutPrecommitN is timeoutPrecommit0 + timeoutPrecommitDelta*N
	timeoutCommit         = 1000 * time.Millisecond // After +2/3 commits received for committed block, wait this long for stragglers in the next height's RoundStepNewHeight.

)

var (
	ErrInvalidProposalSignature = errors.New("Error invalid proposal signature")
	ErrInvalidProposalPOLRound  = errors.New("Error invalid proposal POL round")
	ErrAddingVote               = errors.New("Error adding vote")
	ErrVoteHeightMismatch       = errors.New("Error vote height mismatch")
)

//-----------------------------------------------------------------------------
// RoundStepType enum type

type RoundStepType uint8 // These must be numeric, ordered.

const (
	RoundStepNewHeight     = RoundStepType(0x01) // Wait til CommitTime + timeoutCommit
	RoundStepNewRound      = RoundStepType(0x02) // Setup new round and go to RoundStepPropose
	RoundStepPropose       = RoundStepType(0x03) // Did propose, gossip proposal
	RoundStepPrevote       = RoundStepType(0x04) // Did prevote, gossip prevotes
	RoundStepPrevoteWait   = RoundStepType(0x05) // Did receive any +2/3 prevotes, start timeout
	RoundStepPrecommit     = RoundStepType(0x06) // Did precommit, gossip precommits
	RoundStepPrecommitWait = RoundStepType(0x07) // Did receive any +2/3 precommits, start timeout
	RoundStepCommit        = RoundStepType(0x08) // Entered commit state machine
	// NOTE: RoundStepNewHeight acts as RoundStepCommitWait.
)

func (rs RoundStepType) String() string {
	switch rs {
	case RoundStepNewHeight:
		return "RoundStepNewHeight"
	case RoundStepNewRound:
		return "RoundStepNewRound"
	case RoundStepPropose:
		return "RoundStepPropose"
	case RoundStepPrevote:
		return "RoundStepPrevote"
	case RoundStepPrevoteWait:
		return "RoundStepPrevoteWait"
	case RoundStepPrecommit:
		return "RoundStepPrecommit"
	case RoundStepPrecommitWait:
		return "RoundStepPrecommitWait"
	case RoundStepCommit:
		return "RoundStepCommit"
	default:
		return "RoundStepUnknown" // Cannot panic.
	}
}

//-----------------------------------------------------------------------------

// Immutable when returned from ConsensusState.GetRoundState()
type RoundState struct {
	Height             int // Height we are working on
	Round              int
	Step               RoundStepType
	StartTime          time.Time
	CommitTime         time.Time // Subjective time when +2/3 precommits for Block at Round were found
	Validators         *types.ValidatorSet
	Proposal           *types.Proposal
	ProposalBlock      *types.Block
	ProposalBlockParts *types.PartSet
	LockedRound        int
	LockedBlock        *types.Block
	LockedBlockParts   *types.PartSet
	Votes              *HeightVoteSet
	CommitRound        int            //
	LastCommit         *types.VoteSet // Last precommits at Height-1
	LastValidators     *types.ValidatorSet
}

func (rs *RoundState) RoundStateEvent() *types.EventDataRoundState {
	edrs := &types.EventDataRoundState{
		Height:     rs.Height,
		Round:      rs.Round,
		Step:       rs.Step.String(),
		RoundState: rs,
	}
	return edrs
}

func (rs *RoundState) String() string {
	return rs.StringIndented("")
}

func (rs *RoundState) StringIndented(indent string) string {
	return fmt.Sprintf(`RoundState{
%s  H:%v R:%v S:%v
%s  StartTime:     %v
%s  CommitTime:    %v
%s  Validators:    %v
%s  Proposal:      %v
%s  ProposalBlock: %v %v
%s  LockedRound:   %v
%s  LockedBlock:   %v %v
%s  Votes:         %v
%s  LastCommit: %v
%s  LastValidators:    %v
%s}`,
		indent, rs.Height, rs.Round, rs.Step,
		indent, rs.StartTime,
		indent, rs.CommitTime,
		indent, rs.Validators.StringIndented(indent+"    "),
		indent, rs.Proposal,
		indent, rs.ProposalBlockParts.StringShort(), rs.ProposalBlock.StringShort(),
		indent, rs.LockedRound,
		indent, rs.LockedBlockParts.StringShort(), rs.LockedBlock.StringShort(),
		indent, rs.Votes.StringIndented(indent+"    "),
		indent, rs.LastCommit.StringShort(),
		indent, rs.LastValidators.StringIndented(indent+"    "),
		indent)
}

func (rs *RoundState) StringShort() string {
	return fmt.Sprintf(`RoundState{H:%v R:%v S:%v ST:%v}`,
		rs.Height, rs.Round, rs.Step, rs.StartTime)
}

//-----------------------------------------------------------------------------

var (
	msgQueueSize       = 1000
	tickTockBufferSize = 10
)

// msgs from the reactor which may update the state
type msgInfo struct {
	msg     ConsensusMessage
	peerKey string
}

// internally generated messages which may update the state
type timeoutInfo struct {
	duration time.Duration
	height   int
	round    int
	step     RoundStepType
}

func (ti *timeoutInfo) String() string {
	return fmt.Sprintf("%v ; %d/%d %v", ti.duration, ti.height, ti.round, ti.step)
}

// Tracks consensus state across block heights and rounds.
type ConsensusState struct {
	QuitService

	proxyAppConn     proxy.AppConn
	proxyAppUpdateCh chan proxy.ResetProxy

	blockStore    *bc.BlockStore
	mempool       *mempl.Mempool
	privValidator *types.PrivValidator

	mtx sync.Mutex
	RoundState
	state *sm.State // State until height-1.

	peerMsgQueue     chan msgInfo     // serializes msgs affecting state (proposals, block parts, votes)
	internalMsgQueue chan msgInfo     // like peerMsgQueue but for our own proposals, parts, votes
	timeoutTicker    *time.Ticker     // ticker for timeouts
	tickChan         chan timeoutInfo // start the timeoutTicker in the timeoutRoutine
	tockChan         chan timeoutInfo // timeouts are relayed on tockChan to the receiveRoutine

	evsw *events.EventSwitch

	nSteps int // used for testing to limit the number of transitions the state makes
}

func NewConsensusState(state *sm.State, proxyAppConn proxy.AppConn, blockStore *bc.BlockStore, mempool *mempl.Mempool) *ConsensusState {
	cs := &ConsensusState{
		proxyAppConn:     proxyAppConn,
		proxyAppUpdateCh: make(chan proxy.ResetProxy, 1),
		blockStore:       blockStore,
		mempool:          mempool,
		peerMsgQueue:     make(chan msgInfo, msgQueueSize),
		internalMsgQueue: make(chan msgInfo, msgQueueSize),
		timeoutTicker:    new(time.Ticker),
		tickChan:         make(chan timeoutInfo, tickTockBufferSize),
		tockChan:         make(chan timeoutInfo, tickTockBufferSize),
	}
	cs.updateToState(state)
	// Don't call scheduleRound0 yet.
	// We do that upon Start().
	cs.reconstructLastCommit(state)
	cs.QuitService = *NewQuitService(log, "ConsensusState", cs)
	return cs
}

//----------------------------------------
// Public interface

func (cs *ConsensusState) ResetProxyApp(proxyApp proxy.AppConn, wg *sync.WaitGroup, done chan struct{}) {
	cs.proxyAppUpdateCh <- proxy.ResetProxy{proxyApp, wg, done}
}

// implements events.Eventable
func (cs *ConsensusState) SetEventSwitch(evsw *events.EventSwitch) {
	cs.evsw = evsw
}

func (cs *ConsensusState) String() string {
	return Fmt("ConsensusState(H:%v R:%v S:%v", cs.Height, cs.Round, cs.Step)
}

func (cs *ConsensusState) GetState() *sm.State {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	return cs.state.Copy()
}

func (cs *ConsensusState) GetRoundState() *RoundState {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	return cs.getRoundState()
}

func (cs *ConsensusState) getRoundState() *RoundState {
	rs := cs.RoundState // copy
	return &rs
}

func (cs *ConsensusState) SetPrivValidator(priv *types.PrivValidator) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	cs.privValidator = priv
}

func (cs *ConsensusState) OnStart() error {
	cs.BaseService.OnStart()

	// first we schedule the round (no go routines)
	// then we start the timeout and receive routines.
	// tickChan is buffered so scheduleRound0 will finish.
	// Then all further access to the RoundState is through the receiveRoutine
	cs.scheduleRound0(cs.Height)
	cs.startRoutines(0)
	return nil
}

// timeoutRoutine: receive requests for timeouts on tickChan and fire timeouts on tockChan
// receiveRoutine: serializes processing of proposoals, block parts, votes; coordinates state transitions
func (cs *ConsensusState) startRoutines(maxSteps int) {
	go cs.timeoutRoutine()
	go cs.receiveRoutine(maxSteps)
}

func (cs *ConsensusState) OnStop() {
	cs.QuitService.OnStop()
}

//------------------------------------------------------------
// Public interface for passing messages into the consensus state,
// possibly causing a state transition
// TODO: should these return anything or let callers just use events?

// May block on send if queue is full.
func (cs *ConsensusState) AddVote(valIndex int, vote *types.Vote, peerKey string) (added bool, address []byte, err error) {
	if peerKey == "" {
		cs.internalMsgQueue <- msgInfo{&VoteMessage{valIndex, vote}, ""}
	} else {
		cs.peerMsgQueue <- msgInfo{&VoteMessage{valIndex, vote}, peerKey}
	}

	// TODO: wait for event?!
	return false, nil, nil
}

// May block on send if queue is full.
func (cs *ConsensusState) SetProposal(proposal *types.Proposal, peerKey string) error {

	if peerKey == "" {
		cs.internalMsgQueue <- msgInfo{&ProposalMessage{proposal}, ""}
	} else {
		cs.peerMsgQueue <- msgInfo{&ProposalMessage{proposal}, peerKey}
	}

	// TODO: wait for event?!
	return nil
}

// May block on send if queue is full.
func (cs *ConsensusState) AddProposalBlockPart(height, round int, part *types.Part, peerKey string) error {

	if peerKey == "" {
		cs.internalMsgQueue <- msgInfo{&BlockPartMessage{height, round, part}, ""}
	} else {
		cs.peerMsgQueue <- msgInfo{&BlockPartMessage{height, round, part}, peerKey}
	}

	// TODO: wait for event?!
	return nil
}

// May block on send if queue is full.
func (cs *ConsensusState) SetProposalAndBlock(proposal *types.Proposal, block *types.Block, parts *types.PartSet, peerKey string) error {
	cs.SetProposal(proposal, peerKey)
	for i := 0; i < parts.Total(); i++ {
		part := parts.GetPart(i)
		cs.AddProposalBlockPart(proposal.Height, proposal.Round, part, peerKey)
	}
	return nil // TODO errors
}

//------------------------------------------------------------
// internal functions for managing the state

func (cs *ConsensusState) updateHeight(height int) {
	cs.Height = height
}

func (cs *ConsensusState) updateRoundStep(round int, step RoundStepType) {
	cs.Round = round
	cs.Step = step
}

// enterNewRound(height, 0) at cs.StartTime.
func (cs *ConsensusState) scheduleRound0(height int) {
	//log.Info("scheduleRound0", "now", time.Now(), "startTime", cs.StartTime)
	sleepDuration := cs.StartTime.Sub(time.Now())
	if sleepDuration < time.Duration(0) {
		sleepDuration = time.Duration(0)
	}
	cs.scheduleTimeout(sleepDuration, height, 0, RoundStepNewHeight)
}

// Attempt to schedule a timeout by sending timeoutInfo on the tickChan.
// The timeoutRoutine is alwaya available to read from tickChan (it won't block).
// The scheduling may fail if the timeoutRoutine has already scheduled a timeout for a later height/round/step.
func (cs *ConsensusState) scheduleTimeout(duration time.Duration, height, round int, step RoundStepType) {
	cs.tickChan <- timeoutInfo{duration, height, round, step}
}

// send a msg into the receiveRoutine regarding our own proposal, block part, or vote
func (cs *ConsensusState) sendInternalMessage(mi msgInfo) {
	select {
	case cs.internalMsgQueue <- mi:
	default:
		// NOTE: using the go-routine means our votes can
		// be processed out of order.
		// TODO: use CList here for strict determinism and
		// attempt push to internalMsgQueue in receiveRoutine
		log.Debug("Internal msg queue is full. Using a go-routine")
		go func() { cs.internalMsgQueue <- mi }()
	}
}

// Reconstruct LastCommit from SeenValidation, which we saved along with the block,
// (which happens even before saving the state)
func (cs *ConsensusState) reconstructLastCommit(state *sm.State) {
	if state.LastBlockHeight == 0 {
		return
	}
	lastPrecommits := types.NewVoteSet(state.LastBlockHeight, 0, types.VoteTypePrecommit, state.LastValidators)
	seenValidation := cs.blockStore.LoadSeenValidation(state.LastBlockHeight)
	for idx, precommit := range seenValidation.Precommits {
		if precommit == nil {
			continue
		}
		added, _, err := lastPrecommits.AddByIndex(idx, precommit)
		if !added || err != nil {
			PanicCrisis(Fmt("Failed to reconstruct LastCommit: %v", err))
		}
	}
	if !lastPrecommits.HasTwoThirdsMajority() {
		PanicSanity("Failed to reconstruct LastCommit: Does not have +2/3 maj")
	}
	cs.LastCommit = lastPrecommits
}

// Updates ConsensusState and increments height to match that of state.
// The round becomes 0 and cs.Step becomes RoundStepNewHeight.
func (cs *ConsensusState) updateToState(state *sm.State) {
	if cs.CommitRound > -1 && 0 < cs.Height && cs.Height != state.LastBlockHeight {
		PanicSanity(Fmt("updateToState() expected state height of %v but found %v",
			cs.Height, state.LastBlockHeight))
	}
	if cs.state != nil && cs.state.LastBlockHeight+1 != cs.Height {
		// This might happen when someone else is mutating cs.state.
		// Someone forgot to pass in state.Copy() somewhere?!
		PanicSanity(Fmt("Inconsistent cs.state.LastBlockHeight+1 %v vs cs.Height %v",
			cs.state.LastBlockHeight+1, cs.Height))
	}

	// If state isn't further out than cs.state, just ignore.
	// This happens when SwitchToConsensus() is called in the reactor.
	// We don't want to reset e.g. the Votes.
	if cs.state != nil && (state.LastBlockHeight <= cs.state.LastBlockHeight) {
		log.Notice("Ignoring updateToState()", "newHeight", state.LastBlockHeight+1, "oldHeight", cs.state.LastBlockHeight+1)
		return
	}

	// Reset fields based on state.
	validators := state.Validators
	height := state.LastBlockHeight + 1 // Next desired block height
	lastPrecommits := (*types.VoteSet)(nil)
	if cs.CommitRound > -1 && cs.Votes != nil {
		if !cs.Votes.Precommits(cs.CommitRound).HasTwoThirdsMajority() {
			PanicSanity("updateToState(state) called but last Precommit round didn't have +2/3")
		}
		lastPrecommits = cs.Votes.Precommits(cs.CommitRound)
	}

	// RoundState fields
	cs.updateHeight(height)
	cs.updateRoundStep(0, RoundStepNewHeight)
	if cs.CommitTime.IsZero() {
		// "Now" makes it easier to sync up dev nodes.
		// We add timeoutCommit to allow transactions
		// to be gathered for the first block.
		// And alternative solution that relies on clocks:
		//  cs.StartTime = state.LastBlockTime.Add(timeoutCommit)
		cs.StartTime = time.Now().Add(timeoutCommit)
	} else {
		cs.StartTime = cs.CommitTime.Add(timeoutCommit)
	}
	cs.CommitTime = time.Time{}
	cs.Validators = validators
	cs.Proposal = nil
	cs.ProposalBlock = nil
	cs.ProposalBlockParts = nil
	cs.LockedRound = 0
	cs.LockedBlock = nil
	cs.LockedBlockParts = nil
	cs.Votes = NewHeightVoteSet(height, validators)
	cs.CommitRound = -1
	cs.LastCommit = lastPrecommits
	cs.LastValidators = state.LastValidators

	cs.state = state

	// Finally, broadcast RoundState
	cs.newStep()
}

func (cs *ConsensusState) newStep() {
	cs.nSteps += 1
	// newStep is called by updateToStep in NewConsensusState before the evsw is set!
	if cs.evsw != nil {
		cs.evsw.FireEvent(types.EventStringNewRoundStep(), cs.RoundStateEvent())
	}
}

//-----------------------------------------
// the main go routines

// the state machine sends on tickChan to start a new timer.
// timers are interupted and replaced by new ticks from later steps
// timeouts of 0 on the tickChan will be immediately relayed to the tockChan
func (cs *ConsensusState) timeoutRoutine() {
	log.Debug("Starting timeout routine")
	var ti timeoutInfo
	for {
		select {
		case newti := <-cs.tickChan:
			log.Debug("Received tick", "old_ti", ti, "new_ti", newti)

			// ignore tickers for old height/round/step
			if newti.height < ti.height {
				continue
			} else if newti.height == ti.height {
				if newti.round < ti.round {
					continue
				} else if newti.round == ti.round {
					if ti.step > 0 && newti.step <= ti.step {
						continue
					}
				}
			}

			ti = newti

			// if the newti has duration == 0, we relay to the tockChan immediately (no timeout)
			if ti.duration == time.Duration(0) {
				go func(t timeoutInfo) { cs.tockChan <- t }(ti)
				continue
			}

			log.Info("Scheduling timeout", "dur", ti.duration, "height", ti.height, "round", ti.round, "step", ti.step)
			cs.timeoutTicker.Stop()
			cs.timeoutTicker = time.NewTicker(ti.duration)
		case <-cs.timeoutTicker.C:
			log.Info("Timed out", "dur", ti.duration, "height", ti.height, "round", ti.round, "step", ti.step)
			cs.timeoutTicker.Stop()
			// go routine here gaurantees timeoutRoutine doesn't block.
			// Determinism comes from playback in the receiveRoutine.
			// We can eliminate it by merging the timeoutRoutine into receiveRoutine
			//  and managing the timeouts ourselves with a millisecond ticker
			go func(t timeoutInfo) { cs.tockChan <- t }(ti)
		case <-cs.Quit:
			return
		}
	}
}

// a nice idea but probably more trouble than its worth
func (cs *ConsensusState) stopTimer() {
	cs.timeoutTicker.Stop()
}

// receiveRoutine handles messages which may cause state transitions.
// it's argument (n) is the number of messages to process before exiting - use 0 to run forever
// It keeps the RoundState and is the only thing that updates it.
// Updates (state transitions) happen on timeouts, complete proposals, and 2/3 majorities
func (cs *ConsensusState) receiveRoutine(maxSteps int) {
	for {
		if maxSteps > 0 {
			if cs.nSteps >= maxSteps {
				log.Warn("reached max steps. exiting receive routine")
				cs.nSteps = 0
				return
			}
		}
		rs := cs.RoundState
		var mi msgInfo

		select {
		case mi = <-cs.peerMsgQueue:
			// handles proposals, block parts, votes
			// may generate internal events (votes, complete proposals, 2/3 majorities)
			cs.handleMsg(mi, rs)
		case mi = <-cs.internalMsgQueue:
			// handles proposals, block parts, votes
			cs.handleMsg(mi, rs)
		case ti := <-cs.tockChan:
			// if the timeout is relevant to the rs
			// go to the next step
			cs.handleTimeout(ti, rs)
		case <-cs.Quit:
			return
		}
	}
}

// state transitions on complete-proposal, 2/3-any, 2/3-one
func (cs *ConsensusState) handleMsg(mi msgInfo, rs RoundState) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	var err error
	msg, peerKey := mi.msg, mi.peerKey
	switch msg := msg.(type) {
	case *ProposalMessage:
		// will not cause transition.
		// once proposal is set, we can receive block parts
		err = cs.setProposal(msg.Proposal)
	case *BlockPartMessage:
		// if the proposal is complete, we'll enterPrevote or tryFinalizeCommit
		// if we're the only validator, the enterPrevote may take us through to the next round
		_, err = cs.addProposalBlockPart(msg.Height, msg.Part)
	case *VoteMessage:
		// attempt to add the vote and dupeout the validator if its a duplicate signature
		// if the vote gives us a 2/3-any or 2/3-one, we transition
		err := cs.tryAddVote(msg.ValidatorIndex, msg.Vote, peerKey)
		if err == ErrAddingVote {
			// TODO: punish peer
		}

		// NOTE: the vote is broadcast to peers by the reactor listening
		// for vote events

		// TODO: If rs.Height == vote.Height && rs.Round < vote.Round,
		// the peer is sending us CatchupCommit precommits.
		// We could make note of this and help filter in broadcastHasVoteMessage().
	default:
		log.Warn("Unknown msg type", reflect.TypeOf(msg))
	}
	if err != nil {
		log.Error("error with msg", "error", err)
	}
}

func (cs *ConsensusState) handleTimeout(ti timeoutInfo, rs RoundState) {
	log.Debug("Received tock", "timeout", ti.duration, "height", ti.height, "round", ti.round, "step", ti.step)

	// timeouts must be for current height, round, step
	if ti.height != rs.Height || ti.round < rs.Round || (ti.round == rs.Round && ti.step < rs.Step) {
		log.Debug("Ignoring tock because we're ahead", "height", rs.Height, "round", rs.Round, "step", rs.Step)
		return
	}

	// the timeout will now cause a state transition
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	switch ti.step {
	case RoundStepNewHeight:
		// NewRound event fired from enterNewRound.
		// Do we want a timeout event too?
		cs.enterNewRound(ti.height, 0)
	case RoundStepPropose:
		cs.evsw.FireEvent(types.EventStringTimeoutPropose(), cs.RoundStateEvent())
		cs.enterPrevote(ti.height, ti.round)
	case RoundStepPrevoteWait:
		cs.evsw.FireEvent(types.EventStringTimeoutWait(), cs.RoundStateEvent())
		cs.enterPrecommit(ti.height, ti.round)
	case RoundStepPrecommitWait:
		cs.evsw.FireEvent(types.EventStringTimeoutWait(), cs.RoundStateEvent())
		cs.enterNewRound(ti.height, ti.round+1)
	default:
		panic(Fmt("Invalid timeout step: %v", ti.step))
	}

}

//-----------------------------------------------------------------------------
// State functions
// Used internally by handleTimeout and handleMsg to make state transitions

// Enter: +2/3 precommits for nil at (height,round-1)
// Enter: `timeoutPrecommits` after any +2/3 precommits from (height,round-1)
// Enter: `startTime = commitTime+timeoutCommit` from NewHeight(height)
// NOTE: cs.StartTime was already set for height.
func (cs *ConsensusState) enterNewRound(height int, round int) {
	if cs.Height != height || round < cs.Round || (cs.Round == round && cs.Step != RoundStepNewHeight) {
		log.Debug(Fmt("enterNewRound(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}

	if now := time.Now(); cs.StartTime.After(now) {
		log.Warn("Need to set a buffer and log.Warn() here for sanity.", "startTime", cs.StartTime, "now", now)
	}
	// cs.stopTimer()

	log.Notice(Fmt("enterNewRound(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	// Increment validators if necessary
	validators := cs.Validators
	if cs.Round < round {
		validators = validators.Copy()
		validators.IncrementAccum(round - cs.Round)
	}

	// Setup new round
	// we don't fire newStep for this step,
	// but we fire an event, so update the round step first
	cs.updateRoundStep(round, RoundStepNewRound)
	cs.Validators = validators
	if round == 0 {
		// We've already reset these upon new height,
		// and meanwhile we might have received a proposal
		// for round 0.
	} else {
		cs.Proposal = nil
		cs.ProposalBlock = nil
		cs.ProposalBlockParts = nil
	}
	cs.Votes.SetRound(round + 1) // also track next round (round+1) to allow round-skipping

	cs.evsw.FireEvent(types.EventStringNewRound(), cs.RoundStateEvent())

	// Immediately go to enterPropose.
	cs.enterPropose(height, round)
}

// Enter: from NewRound(height,round).
func (cs *ConsensusState) enterPropose(height int, round int) {
	//	cs.mtx.Lock()
	//	cs.mtx.Unlock()

	if cs.Height != height || round < cs.Round || (cs.Round == round && RoundStepPropose <= cs.Step) {
		log.Debug(Fmt("enterPropose(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}
	log.Info(Fmt("enterPropose(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterPropose:
		cs.updateRoundStep(round, RoundStepPropose)
		cs.newStep()

		// If we have the whole proposal + POL, then goto Prevote now.
		// else, we'll enterPrevote when the rest of the proposal is received (in AddProposalBlockPart),
		// or else after timeoutPropose
		if cs.isProposalComplete() {
			cs.enterPrevote(height, cs.Round)
		}
	}()

	// This step times out after `timeoutPropose`
	cs.scheduleTimeout(timeoutPropose, height, round, RoundStepPropose)

	// Nothing more to do if we're not a validator
	if cs.privValidator == nil {
		return
	}

	if !bytes.Equal(cs.Validators.Proposer().Address, cs.privValidator.Address) {
		log.Info("enterPropose: Not our turn to propose", "proposer", cs.Validators.Proposer().Address, "privValidator", cs.privValidator)
	} else {
		log.Info("enterPropose: Our turn to propose", "proposer", cs.Validators.Proposer().Address, "privValidator", cs.privValidator)
		cs.decideProposal(height, round)
	}

}

func (cs *ConsensusState) decideProposal(height, round int) {
	var block *types.Block
	var blockParts *types.PartSet

	// Decide on block
	if cs.LockedBlock != nil {
		// If we're locked onto a block, just choose that.
		block, blockParts = cs.LockedBlock, cs.LockedBlockParts
	} else {
		// Create a new proposal block from state/txs from the mempool.
		block, blockParts = cs.createProposalBlock()
		if block == nil { // on error
			return
		}
	}

	// Make proposal
	proposal := types.NewProposal(height, round, blockParts.Header(), cs.Votes.POLRound())
	err := cs.privValidator.SignProposal(cs.state.ChainID, proposal)
	if err == nil {
		// Set fields
		/*  fields set by setProposal and addBlockPart
		cs.Proposal = proposal
		cs.ProposalBlock = block
		cs.ProposalBlockParts = blockParts
		*/

		// send proposal and block parts on internal msg queue
		cs.sendInternalMessage(msgInfo{&ProposalMessage{proposal}, ""})
		for i := 0; i < blockParts.Total(); i++ {
			part := blockParts.GetPart(i)
			cs.sendInternalMessage(msgInfo{&BlockPartMessage{cs.Height, cs.Round, part}, ""})
		}
		log.Info("Signed and sent proposal", "height", height, "round", round, "proposal", proposal)
		log.Debug(Fmt("Signed and sent proposal block: %v", block))
	} else {
		log.Warn("enterPropose: Error signing proposal", "height", height, "round", round, "error", err)
	}
}

// Returns true if the proposal block is complete &&
// (if POLRound was proposed, we have +2/3 prevotes from there).
func (cs *ConsensusState) isProposalComplete() bool {
	if cs.Proposal == nil || cs.ProposalBlock == nil {
		return false
	}
	// we have the proposal. if there's a POLRound,
	// make sure we have the prevotes from it too
	if cs.Proposal.POLRound < 0 {
		return true
	} else {
		// if this is false the proposer is lying or we haven't received the POL yet
		return cs.Votes.Prevotes(cs.Proposal.POLRound).HasTwoThirdsMajority()
	}
}

// Create the next block to propose and return it.
// Returns nil block upon error.
// NOTE: keep it side-effect free for clarity.
func (cs *ConsensusState) createProposalBlock() (block *types.Block, blockParts *types.PartSet) {
	var validation *types.Validation
	if cs.Height == 1 {
		// We're creating a proposal for the first block.
		// The validation is empty, but not nil.
		validation = &types.Validation{}
	} else if cs.LastCommit.HasTwoThirdsMajority() {
		// Make the validation from LastCommit
		validation = cs.LastCommit.MakeValidation()
	} else {
		// This shouldn't happen.
		log.Error("enterPropose: Cannot propose anything: No validation for the previous block.")
		return
	}

	// Mempool validated transactions
	txs, err := cs.mempool.Reap()
	if err != nil {
		log.Warn("createProposalBlock: Error getting proposal txs", "error", err)
		return nil, nil
	}

	block = &types.Block{
		Header: &types.Header{
			ChainID:        cs.state.ChainID,
			Height:         cs.Height,
			Time:           time.Now(),
			Fees:           0, // TODO fees
			NumTxs:         len(txs),
			LastBlockHash:  cs.state.LastBlockHash,
			LastBlockParts: cs.state.LastBlockParts,
			ValidatorsHash: cs.state.Validators.Hash(),
			AppHash:        cs.state.AppHash, // state merkle root of txs from the previous block.
		},
		LastValidation: validation,
		Data: &types.Data{
			Txs: txs,
		},
	}
	block.FillHeader()
	blockParts = block.MakePartSet()

	return block, blockParts
}

// Enter: `timeoutPropose` after entering Propose.
// Enter: proposal block and POL is ready.
// Enter: any +2/3 prevotes for future round.
// Prevote for LockedBlock if we're locked, or ProposalBlock if valid.
// Otherwise vote nil.
func (cs *ConsensusState) enterPrevote(height int, round int) {
	//cs.mtx.Lock()
	//defer cs.mtx.Unlock()
	if cs.Height != height || round < cs.Round || (cs.Round == round && RoundStepPrevote <= cs.Step) {
		log.Debug(Fmt("enterPrevote(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}

	defer func() {
		// Done enterPrevote:
		cs.updateRoundStep(round, RoundStepPrevote)
		cs.newStep()
	}()

	// fire event for how we got here
	if cs.isProposalComplete() {
		cs.evsw.FireEvent(types.EventStringCompleteProposal(), cs.RoundStateEvent())
	} else {
		// we received +2/3 prevotes for a future round
		// TODO: catchup event?
	}

	// cs.stopTimer()

	log.Info(Fmt("enterPrevote(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	// Sign and broadcast vote as necessary
	cs.doPrevote(height, round)

	// Once `addVote` hits any +2/3 prevotes, we will go to PrevoteWait
	// (so we have more time to try and collect +2/3 prevotes for a single block)
}

func (cs *ConsensusState) doPrevote(height int, round int) {
	// If a block is locked, prevote that.
	if cs.LockedBlock != nil {
		log.Info("enterPrevote: Block was locked")
		cs.signAddVote(types.VoteTypePrevote, cs.LockedBlock.Hash(), cs.LockedBlockParts.Header())
		return
	}

	// If ProposalBlock is nil, prevote nil.
	if cs.ProposalBlock == nil {
		log.Warn("enterPrevote: ProposalBlock is nil")
		cs.signAddVote(types.VoteTypePrevote, nil, types.PartSetHeader{})
		return
	}

	// Valdiate proposal block
	err := cs.state.ValidateBlock(cs.ProposalBlock)
	if err != nil {
		// ProposalBlock is invalid, prevote nil.
		log.Warn("enterPrevote: ProposalBlock is invalid", "error", err)
		cs.signAddVote(types.VoteTypePrevote, nil, types.PartSetHeader{})
		return
	}

	// Prevote cs.ProposalBlock
	// NOTE: the proposal signature is validated when it is received,
	// and the proposal block parts are validated as they are received (against the merkle hash in the proposal)
	cs.signAddVote(types.VoteTypePrevote, cs.ProposalBlock.Hash(), cs.ProposalBlockParts.Header())
	return
}

// Enter: any +2/3 prevotes at next round.
func (cs *ConsensusState) enterPrevoteWait(height int, round int) {
	//cs.mtx.Lock()
	//defer cs.mtx.Unlock()
	if cs.Height != height || round < cs.Round || (cs.Round == round && RoundStepPrevoteWait <= cs.Step) {
		log.Debug(Fmt("enterPrevoteWait(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}
	if !cs.Votes.Prevotes(round).HasTwoThirdsAny() {
		PanicSanity(Fmt("enterPrevoteWait(%v/%v), but Prevotes does not have any +2/3 votes", height, round))
	}
	log.Info(Fmt("enterPrevoteWait(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterPrevoteWait:
		cs.updateRoundStep(round, RoundStepPrevoteWait)
		cs.newStep()
	}()

	// After `timeoutPrevote0+timeoutPrevoteDelta*round`, enterPrecommit()
	cs.scheduleTimeout(timeoutPrevote0+timeoutPrevoteDelta*time.Duration(round), height, round, RoundStepPrevoteWait)
}

// Enter: +2/3 precomits for block or nil.
// Enter: `timeoutPrevote` after any +2/3 prevotes.
// Enter: any +2/3 precommits for next round.
// Lock & precommit the ProposalBlock if we have enough prevotes for it (a POL in this round)
// else, unlock an existing lock and precommit nil if +2/3 of prevotes were nil,
// else, precommit nil otherwise.
func (cs *ConsensusState) enterPrecommit(height int, round int) {
	//cs.mtx.Lock()
	//	defer cs.mtx.Unlock()
	if cs.Height != height || round < cs.Round || (cs.Round == round && RoundStepPrecommit <= cs.Step) {
		log.Debug(Fmt("enterPrecommit(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}

	// cs.stopTimer()

	log.Info(Fmt("enterPrecommit(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterPrecommit:
		cs.updateRoundStep(round, RoundStepPrecommit)
		cs.newStep()
	}()

	hash, partsHeader, ok := cs.Votes.Prevotes(round).TwoThirdsMajority()

	// If we don't have a polka, we must precommit nil
	if !ok {
		if cs.LockedBlock != nil {
			log.Info("enterPrecommit: No +2/3 prevotes during enterPrecommit while we're locked. Precommitting nil")
		} else {
			log.Info("enterPrecommit: No +2/3 prevotes during enterPrecommit. Precommitting nil.")
		}
		cs.signAddVote(types.VoteTypePrecommit, nil, types.PartSetHeader{})
		return
	}

	// At this point +2/3 prevoted for a particular block or nil
	cs.evsw.FireEvent(types.EventStringPolka(), cs.RoundStateEvent())

	// the latest POLRound should be this round
	if cs.Votes.POLRound() < round {
		PanicSanity(Fmt("This POLRound should be %v but got %", round, cs.Votes.POLRound()))
	}

	// +2/3 prevoted nil. Unlock and precommit nil.
	if len(hash) == 0 {
		if cs.LockedBlock == nil {
			log.Info("enterPrecommit: +2/3 prevoted for nil.")
		} else {
			log.Info("enterPrecommit: +2/3 prevoted for nil. Unlocking")
			cs.LockedRound = 0
			cs.LockedBlock = nil
			cs.LockedBlockParts = nil
			cs.evsw.FireEvent(types.EventStringUnlock(), cs.RoundStateEvent())
		}
		cs.signAddVote(types.VoteTypePrecommit, nil, types.PartSetHeader{})
		return
	}

	// At this point, +2/3 prevoted for a particular block.

	// If we're already locked on that block, precommit it, and update the LockedRound
	if cs.LockedBlock.HashesTo(hash) {
		log.Info("enterPrecommit: +2/3 prevoted locked block. Relocking")
		cs.LockedRound = round
		cs.evsw.FireEvent(types.EventStringRelock(), cs.RoundStateEvent())
		cs.signAddVote(types.VoteTypePrecommit, hash, partsHeader)
		return
	}

	// If +2/3 prevoted for proposal block, stage and precommit it
	if cs.ProposalBlock.HashesTo(hash) {
		log.Info("enterPrecommit: +2/3 prevoted proposal block. Locking", "hash", hash)
		// Validate the block.
		if err := cs.state.ValidateBlock(cs.ProposalBlock); err != nil {
			PanicConsensus(Fmt("enterPrecommit: +2/3 prevoted for an invalid block: %v", err))
		}
		cs.LockedRound = round
		cs.LockedBlock = cs.ProposalBlock
		cs.LockedBlockParts = cs.ProposalBlockParts
		cs.evsw.FireEvent(types.EventStringLock(), cs.RoundStateEvent())
		cs.signAddVote(types.VoteTypePrecommit, hash, partsHeader)
		return
	}

	// There was a polka in this round for a block we don't have.
	// Fetch that block, unlock, and precommit nil.
	// The +2/3 prevotes for this round is the POL for our unlock.
	// TODO: In the future save the POL prevotes for justification.
	cs.LockedRound = 0
	cs.LockedBlock = nil
	cs.LockedBlockParts = nil
	if !cs.ProposalBlockParts.HasHeader(partsHeader) {
		cs.ProposalBlock = nil
		cs.ProposalBlockParts = types.NewPartSetFromHeader(partsHeader)
	}
	cs.evsw.FireEvent(types.EventStringUnlock(), cs.RoundStateEvent())
	cs.signAddVote(types.VoteTypePrecommit, nil, types.PartSetHeader{})
	return
}

// Enter: any +2/3 precommits for next round.
func (cs *ConsensusState) enterPrecommitWait(height int, round int) {
	//cs.mtx.Lock()
	//defer cs.mtx.Unlock()
	if cs.Height != height || round < cs.Round || (cs.Round == round && RoundStepPrecommitWait <= cs.Step) {
		log.Debug(Fmt("enterPrecommitWait(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}
	if !cs.Votes.Precommits(round).HasTwoThirdsAny() {
		PanicSanity(Fmt("enterPrecommitWait(%v/%v), but Precommits does not have any +2/3 votes", height, round))
	}
	log.Info(Fmt("enterPrecommitWait(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterPrecommitWait:
		cs.updateRoundStep(round, RoundStepPrecommitWait)
		cs.newStep()
	}()

	// After `timeoutPrecommit0+timeoutPrecommitDelta*round`, enterNewRound()
	cs.scheduleTimeout(timeoutPrecommit0+timeoutPrecommitDelta*time.Duration(round), height, round, RoundStepPrecommitWait)

}

// Enter: +2/3 precommits for block
func (cs *ConsensusState) enterCommit(height int, commitRound int) {
	//cs.mtx.Lock()
	//defer cs.mtx.Unlock()
	if cs.Height != height || RoundStepCommit <= cs.Step {
		log.Debug(Fmt("enterCommit(%v/%v): Invalid args. Current step: %v/%v/%v", height, commitRound, cs.Height, cs.Round, cs.Step))
		return
	}
	log.Info(Fmt("enterCommit(%v/%v). Current: %v/%v/%v", height, commitRound, cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterCommit:
		// keep ca.Round the same, it points to the right Precommits set.
		cs.updateRoundStep(cs.Round, RoundStepCommit)
		cs.CommitRound = commitRound
		cs.newStep()

		// Maybe finalize immediately.
		cs.tryFinalizeCommit(height)
	}()

	hash, partsHeader, ok := cs.Votes.Precommits(commitRound).TwoThirdsMajority()
	if !ok {
		PanicSanity("RunActionCommit() expects +2/3 precommits")
	}

	// The Locked* fields no longer matter.
	// Move them over to ProposalBlock if they match the commit hash,
	// otherwise they'll be cleared in updateToState.
	if cs.LockedBlock.HashesTo(hash) {
		cs.ProposalBlock = cs.LockedBlock
		cs.ProposalBlockParts = cs.LockedBlockParts
	}

	// If we don't have the block being committed, set up to get it.
	if !cs.ProposalBlock.HashesTo(hash) {
		if !cs.ProposalBlockParts.HasHeader(partsHeader) {
			// We're getting the wrong block.
			// Set up ProposalBlockParts and keep waiting.
			cs.ProposalBlock = nil
			cs.ProposalBlockParts = types.NewPartSetFromHeader(partsHeader)
		} else {
			// We just need to keep waiting.
		}
	}
}

// If we have the block AND +2/3 commits for it, finalize.
func (cs *ConsensusState) tryFinalizeCommit(height int) {
	if cs.Height != height {
		PanicSanity(Fmt("tryFinalizeCommit() cs.Height: %v vs height: %v", cs.Height, height))
	}

	hash, _, ok := cs.Votes.Precommits(cs.CommitRound).TwoThirdsMajority()
	if !ok || len(hash) == 0 {
		log.Warn("Attempt to finalize failed. There was no +2/3 majority, or +2/3 was for <nil>.")
		return
	}
	if !cs.ProposalBlock.HashesTo(hash) {
		log.Warn("Attempt to finalize failed. We don't have the commit block.")
		return
	}
	//	go
	cs.finalizeCommit(height)
}

// Increment height and goto RoundStepNewHeight
func (cs *ConsensusState) finalizeCommit(height int) {
	//cs.mtx.Lock()
	//defer cs.mtx.Unlock()

	if cs.Height != height || cs.Step != RoundStepCommit {
		log.Debug(Fmt("finalizeCommit(%v): Invalid args. Current step: %v/%v/%v", height, cs.Height, cs.Round, cs.Step))
		return
	}

	hash, header, ok := cs.Votes.Precommits(cs.CommitRound).TwoThirdsMajority()
	block, blockParts := cs.ProposalBlock, cs.ProposalBlockParts

	if !ok {
		PanicSanity(Fmt("Cannot finalizeCommit, commit does not have two thirds majority"))
	}
	if !blockParts.HasHeader(header) {
		PanicSanity(Fmt("Expected ProposalBlockParts header to be commit header"))
	}
	if !block.HashesTo(hash) {
		PanicSanity(Fmt("Cannot finalizeCommit, ProposalBlock does not hash to commit hash"))
	}
	if err := cs.state.ValidateBlock(block); err != nil {
		PanicConsensus(Fmt("+2/3 committed an invalid block: %v", err))
	}

	log.Notice("Finalizing commit of block", "height", block.Height, "hash", block.Hash())
	log.Info(Fmt("%v", block))

	// Fire off event for new block.
	// TODO: Handle app failure.  See #177
	cs.evsw.FireEvent(types.EventStringNewBlock(), types.EventDataNewBlock{block})

	// Create a copy of the state for staging
	stateCopy := cs.state.Copy()

	// Run the block on the State:
	// + update validator sets
	// + run txs on the proxyAppConn
	err := stateCopy.ExecBlock(cs.evsw, cs.proxyAppConn, block, blockParts.Header())
	if err != nil {
		// when proxyAppConn is reset, we will try again
		log.Error("Exec failed for application", "error", err)
		log.Notice("Waiting for proxyApp reset to retry commit")
		resetProxy := <-cs.proxyAppUpdateCh
		cs.proxyAppConn = resetProxy.ProxyApp
		resetProxy.Wg.Done()
		<-resetProxy.Done
		cs.finalizeCommit(height)
		return
	}

	// Save to blockStore.
	if cs.blockStore.Height() < block.Height {
		commits := cs.Votes.Precommits(cs.CommitRound)
		seenValidation := commits.MakeValidation()
		cs.blockStore.SaveBlock(block, blockParts, seenValidation)
	}

	/*
		// Commit to proxyAppConn
		err = cs.proxyAppConn.CommitSync()
		if err != nil {
			// TODO: handle this gracefully.
			PanicQ(Fmt("Commit failed for application"))
		}
	*/

	// Save the state.
	stateCopy.Save()

	// Update mempool.
	cs.mempool.Update(block.Height, block.Txs)

	// NewHeightStep!
	cs.updateToState(stateCopy)

	// cs.StartTime is already set.
	// Schedule Round0 to start soon.
	cs.scheduleRound0(height + 1)

	// By here,
	// * cs.Height has been increment to height+1
	// * cs.Step is now RoundStepNewHeight
	// * cs.StartTime is set to when we will start round0.
	return
}

//-----------------------------------------------------------------------------

func (cs *ConsensusState) setProposal(proposal *types.Proposal) error {
	//cs.mtx.Lock()
	//defer cs.mtx.Unlock()

	// Already have one
	if cs.Proposal != nil {
		return nil
	}

	// Does not apply
	if proposal.Height != cs.Height || proposal.Round != cs.Round {
		return nil
	}

	// We don't care about the proposal if we're already in RoundStepCommit.
	if RoundStepCommit <= cs.Step {
		return nil
	}

	// Verify POLRound, which must be -1 or between 0 and proposal.Round exclusive.
	if proposal.POLRound != -1 &&
		(proposal.POLRound < 0 || proposal.Round <= proposal.POLRound) {
		return ErrInvalidProposalPOLRound
	}

	// Verify signature
	if !cs.Validators.Proposer().PubKey.VerifyBytes(types.SignBytes(cs.state.ChainID, proposal), proposal.Signature) {
		return ErrInvalidProposalSignature
	}

	cs.Proposal = proposal
	cs.ProposalBlockParts = types.NewPartSetFromHeader(proposal.BlockPartsHeader)
	return nil
}

// NOTE: block is not necessarily valid.
// This can trigger us to go into enterPrevote asynchronously (before we timeout of propose) or to attempt to commit
func (cs *ConsensusState) addProposalBlockPart(height int, part *types.Part) (added bool, err error) {

	// Blocks might be reused, so round mismatch is OK
	if cs.Height != height {
		return false, nil
	}

	// We're not expecting a block part.
	if cs.ProposalBlockParts == nil {
		return false, nil // TODO: bad peer? Return error?
	}

	added, err = cs.ProposalBlockParts.AddPart(part)
	if err != nil {
		return added, err
	}
	if added && cs.ProposalBlockParts.IsComplete() {
		// Added and completed!
		var n int
		var err error
		cs.ProposalBlock = wire.ReadBinary(&types.Block{}, cs.ProposalBlockParts.GetReader(), types.MaxBlockSize, &n, &err).(*types.Block)
		log.Info("Received complete proposal", "height", cs.ProposalBlock.Height, "hash", cs.ProposalBlock.Hash())
		if cs.Step == RoundStepPropose && cs.isProposalComplete() {
			// Move onto the next step
			cs.enterPrevote(height, cs.Round)
		} else if cs.Step == RoundStepCommit {
			// If we're waiting on the proposal block...
			cs.tryFinalizeCommit(height)
		}
		return true, err
	}
	return added, nil
}

// Attempt to add the vote. if its a duplicate signature, dupeout the validator
func (cs *ConsensusState) tryAddVote(valIndex int, vote *types.Vote, peerKey string) error {
	_, _, err := cs.addVote(valIndex, vote, peerKey)
	if err != nil {
		// If the vote height is off, we'll just ignore it,
		// But if it's a conflicting sig, broadcast evidence tx for slashing.
		// If it's otherwise invalid, punish peer.
		if err == ErrVoteHeightMismatch {
			return err
		} else if _, ok := err.(*types.ErrVoteConflictingSignature); ok {
			log.Warn("Found conflicting vote. Publish evidence")
			/* TODO
			evidenceTx := &types.DupeoutTx{
				Address: address,
				VoteA:   *errDupe.VoteA,
				VoteB:   *errDupe.VoteB,
			}
			cs.mempool.BroadcastTx(struct{???}{evidenceTx}) // shouldn't need to check returned err
			*/
			return err
		} else {
			// Probably an invalid signature. Bad peer.
			log.Warn("Error attempting to add vote", "error", err)
			return ErrAddingVote
		}
	}
	return nil
}

//-----------------------------------------------------------------------------

func (cs *ConsensusState) addVote(valIndex int, vote *types.Vote, peerKey string) (added bool, address []byte, err error) {
	log.Debug("addVote", "voteHeight", vote.Height, "voteType", vote.Type, "csHeight", cs.Height)

	// A precommit for the previous height?
	if vote.Height+1 == cs.Height {
		if !(cs.Step == RoundStepNewHeight && vote.Type == types.VoteTypePrecommit) {
			// TODO: give the reason ..
			// fmt.Errorf("tryAddVote: Wrong height, not a LastCommit straggler commit.")
			return added, nil, ErrVoteHeightMismatch
		}
		added, address, err = cs.LastCommit.AddByIndex(valIndex, vote)
		if added {
			log.Info(Fmt("Added to lastPrecommits: %v", cs.LastCommit.StringShort()))
			cs.evsw.FireEvent(types.EventStringVote(), &types.EventDataVote{valIndex, address, vote})

		}
		return
	}

	// A prevote/precommit for this height?
	if vote.Height == cs.Height {
		height := cs.Height
		added, address, err = cs.Votes.AddByIndex(valIndex, vote, peerKey)
		if added {
			cs.evsw.FireEvent(types.EventStringVote(), &types.EventDataVote{valIndex, address, vote})

			switch vote.Type {
			case types.VoteTypePrevote:
				prevotes := cs.Votes.Prevotes(vote.Round)
				log.Info("Added to prevote", "vote", vote, "prevotes", prevotes.StringShort())
				// First, unlock if prevotes is a valid POL.
				// >> lockRound < POLRound <= unlockOrChangeLockRound (see spec)
				// NOTE: If (lockRound < POLRound) but !(POLRound <= unlockOrChangeLockRound),
				// we'll still enterNewRound(H,vote.R) and enterPrecommit(H,vote.R) to process it
				// there.
				if (cs.LockedBlock != nil) && (cs.LockedRound < vote.Round) && (vote.Round <= cs.Round) {
					hash, _, ok := prevotes.TwoThirdsMajority()
					if ok && !cs.LockedBlock.HashesTo(hash) {
						log.Notice("Unlocking because of POL.", "lockedRound", cs.LockedRound, "POLRound", vote.Round)
						cs.LockedRound = 0
						cs.LockedBlock = nil
						cs.LockedBlockParts = nil
						cs.evsw.FireEvent(types.EventStringUnlock(), cs.RoundStateEvent())
					}
				}
				if cs.Round <= vote.Round && prevotes.HasTwoThirdsAny() {
					// Round-skip over to PrevoteWait or goto Precommit.
					cs.enterNewRound(height, vote.Round) // if the vote is ahead of us
					if prevotes.HasTwoThirdsMajority() {
						cs.enterPrecommit(height, vote.Round)
					} else {
						cs.enterPrevote(height, vote.Round) // if the vote is ahead of us
						cs.enterPrevoteWait(height, vote.Round)
					}
				} else if cs.Proposal != nil && 0 <= cs.Proposal.POLRound && cs.Proposal.POLRound == vote.Round {
					// If the proposal is now complete, enter prevote of cs.Round.
					if cs.isProposalComplete() {
						cs.enterPrevote(height, cs.Round)
					}
				}
			case types.VoteTypePrecommit:
				precommits := cs.Votes.Precommits(vote.Round)
				log.Info("Added to precommit", "vote", vote, "precommits", precommits.StringShort())
				hash, _, ok := precommits.TwoThirdsMajority()
				if ok {
					if len(hash) == 0 {
						cs.enterNewRound(height, vote.Round+1)
					} else {
						cs.enterNewRound(height, vote.Round)
						cs.enterPrecommit(height, vote.Round)
						cs.enterCommit(height, vote.Round)
					}
				} else if cs.Round <= vote.Round && precommits.HasTwoThirdsAny() {
					cs.enterNewRound(height, vote.Round)
					cs.enterPrecommit(height, vote.Round)
					cs.enterPrecommitWait(height, vote.Round)
					//}()
				}
			default:
				PanicSanity(Fmt("Unexpected vote type %X", vote.Type)) // Should not happen.
			}
		}
		// Either duplicate, or error upon cs.Votes.AddByIndex()
		return
	} else {
		err = ErrVoteHeightMismatch
	}

	// Height mismatch, bad peer?
	log.Info("Vote ignored and not added", "voteHeight", vote.Height, "csHeight", cs.Height)
	return
}

func (cs *ConsensusState) signVote(type_ byte, hash []byte, header types.PartSetHeader) (*types.Vote, error) {
	vote := &types.Vote{
		Height:           cs.Height,
		Round:            cs.Round,
		Type:             type_,
		BlockHash:        hash,
		BlockPartsHeader: header,
	}
	err := cs.privValidator.SignVote(cs.state.ChainID, vote)
	return vote, err
}

// signs the vote, publishes on internalMsgQueue
func (cs *ConsensusState) signAddVote(type_ byte, hash []byte, header types.PartSetHeader) *types.Vote {
	if cs.privValidator == nil || !cs.Validators.HasAddress(cs.privValidator.Address) {
		return nil
	}
	vote, err := cs.signVote(type_, hash, header)
	if err == nil {
		// TODO: store our index in the cs so we don't have to do this every time
		valIndex, _ := cs.Validators.GetByAddress(cs.privValidator.Address)
		cs.sendInternalMessage(msgInfo{&VoteMessage{valIndex, vote}, ""})
		log.Info("Signed and pushed vote", "height", cs.Height, "round", cs.Round, "vote", vote, "error", err)
		return vote
	} else {
		log.Warn("Error signing vote", "height", cs.Height, "round", cs.Round, "vote", vote, "error", err)
		return nil
	}
}

//---------------------------------------------------------

func CompareHRS(h1, r1 int, s1 RoundStepType, h2, r2 int, s2 RoundStepType) int {
	if h1 < h2 {
		return -1
	} else if h1 > h2 {
		return 1
	}
	if r1 < r2 {
		return -1
	} else if r1 > r2 {
		return 1
	}
	if s1 < s2 {
		return -1
	} else if s1 > s2 {
		return 1
	}
	return 0
}
