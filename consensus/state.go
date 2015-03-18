/*

Consensus State Machine Overview:

* Propose, Prevote, Precommit represent state machine stages. (aka RoundStep, or step).
  Each take a predetermined amount of time depending on the round number.
* The Commit step can be entered by two means:
  1. After the Precommit step, +2/3 Precommits were found
  2. At any time, +2/3 Commits were found
* Once in the Commit stage, two conditions must both be satisfied
  before proceeding to the next height NewHeight.
* The Propose step of the next height does not begin until
  at least Delta duration *after* +2/3 Commits were found.
  The step stays at NewHeight until this timeout occurs before
  proceeding to Propose.

                            +-------------------------------------+
                            |                                     |
                            v                                     |(Wait til CommitTime + Delta)
                      +-----------+                         +-----+-----+
         +----------> |  Propose  +--------------+          | NewHeight |
         |            +-----------+              |          +-----------+
         |                                       |                ^
         |                                       |                |
         |                                       |                |
         |(Else)                                 v                |
   +-----+-----+                           +-----------+          |
   | Precommit |  <------------------------+  Prevote  |          |
   +-----+-----+                           +-----------+          |
         |(If +2/3 Precommits found)                              |
         |                                                        |
         |                + (When +2/3 Commits found)             |
         |                |                                       |
         v                v                                       |
   +------------------------------------------------------------------------------+
   |  Commit                                                      |               |
   |                                                              |               |
   |  +----------------+     * Save Block                         |               |
   |  |Get Block Parts |---> * Stage Block     +--+               +               |
   |  +----------------+     * Broadcast Commit   |    * Setup New Height         |
   |                                              |    * Move Commits set to      |
   |                                              +-->   LastCommits to continue  |
   |                                              |      collecting commits       |
   |  +-----------------+                         |    * Broadcast New State      |
   |  |Get +2/3 Commits |--> * Set CommitTime  +--+                               |
   |  +-----------------+                                                         |
   |                                                                              |
   +------------------------------------------------------------------------------+

*/

package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	blk "github.com/tendermint/tendermint/block"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/config"
	. "github.com/tendermint/tendermint/consensus/types"
	mempl "github.com/tendermint/tendermint/mempool"
	sm "github.com/tendermint/tendermint/state"
)

const (
	roundDuration0         = 10 * time.Second   // The first round is 60 seconds long.
	roundDurationDelta     = 3 * time.Second    // Each successive round lasts 15 seconds longer.
	roundDeadlinePrevote   = float64(1.0 / 3.0) // When the prevote is due.
	roundDeadlinePrecommit = float64(2.0 / 3.0) // When the precommit vote is due.
	newHeightDelta         = roundDuration0 / 3 // The time to wait between commitTime and startTime of next consensus rounds.
)

var (
	ErrInvalidProposalSignature = errors.New("Error invalid proposal signature")
)

//-----------------------------------------------------------------------------
// RoundStep enum type

type RoundStep uint8

const (
	RoundStepNewHeight = RoundStep(0x00) // Round0 for new height started, wait til CommitTime + Delta
	RoundStepNewRound  = RoundStep(0x01) // Pseudostep, immediately goes to RoundStepPropose
	RoundStepPropose   = RoundStep(0x10) // Did propose, gossip proposal
	RoundStepPrevote   = RoundStep(0x11) // Did prevote, gossip prevotes
	RoundStepPrecommit = RoundStep(0x12) // Did precommit, gossip precommits
	RoundStepCommit    = RoundStep(0x20) // Entered commit state machine
)

func (rs RoundStep) String() string {
	switch rs {
	case RoundStepNewHeight:
		return "RoundStepNewHeight"
	case RoundStepNewRound:
		return "RoundStepNewRound"
	case RoundStepPropose:
		return "RoundStepPropose"
	case RoundStepPrevote:
		return "RoundStepPrevote"
	case RoundStepPrecommit:
		return "RoundStepPrecommit"
	case RoundStepCommit:
		return "RoundStepCommit"
	default:
		panic(Fmt("Unknown RoundStep %X", rs))
	}
}

//-----------------------------------------------------------------------------
// RoundAction enum type

type RoundActionType uint8

const (
	RoundActionPropose     = RoundActionType(0xA0) // Propose and goto RoundStepPropose
	RoundActionPrevote     = RoundActionType(0xA1) // Prevote and goto RoundStepPrevote
	RoundActionPrecommit   = RoundActionType(0xA2) // Precommit and goto RoundStepPrecommit
	RoundActionTryCommit   = RoundActionType(0xC0) // Goto RoundStepCommit, or RoundStepPropose for next round.
	RoundActionCommit      = RoundActionType(0xC1) // Goto RoundStepCommit upon +2/3 commits
	RoundActionTryFinalize = RoundActionType(0xC2) // Maybe goto RoundStepPropose for next round.
)

func (rat RoundActionType) String() string {
	switch rat {
	case RoundActionPropose:
		return "RoundActionPropose"
	case RoundActionPrevote:
		return "RoundActionPrevote"
	case RoundActionPrecommit:
		return "RoundActionPrecommit"
	case RoundActionTryCommit:
		return "RoundActionTryCommit"
	case RoundActionCommit:
		return "RoundActionCommit"
	case RoundActionTryFinalize:
		return "RoundActionTryFinalize"
	default:
		panic(Fmt("Unknown RoundAction %X", rat))
	}
}

//-----------------------------------------------------------------------------

type RoundAction struct {
	Height uint            // The block height for which consensus is reaching for.
	Round  uint            // The round number at given height.
	Action RoundActionType // Action to perform.
}

func (ra RoundAction) String() string {
	return Fmt("RoundAction{H:%v R:%v A:%v}", ra.Height, ra.Round, ra.Action)
}

//-----------------------------------------------------------------------------

// Immutable when returned from ConsensusState.GetRoundState()
type RoundState struct {
	Height             uint // Height we are working on
	Round              uint
	Step               RoundStep
	StartTime          time.Time
	CommitTime         time.Time // Time when +2/3 commits were found
	Validators         *sm.ValidatorSet
	Proposal           *Proposal
	ProposalBlock      *blk.Block
	ProposalBlockParts *blk.PartSet
	ProposalPOL        *POL
	ProposalPOLParts   *blk.PartSet
	LockedBlock        *blk.Block
	LockedBlockParts   *blk.PartSet
	LockedPOL          *POL // Rarely needed, so no LockedPOLParts.
	Prevotes           *VoteSet
	Precommits         *VoteSet
	Commits            *VoteSet
	LastCommits        *VoteSet
	PrivValidator      *sm.PrivValidator
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
%s  ProposalPOL:   %v %v
%s  LockedBlock:   %v %v
%s  LockedPOL:     %v
%s  Prevotes:      %v
%s  Precommits:    %v
%s  Commits:       %v
%s  LastCommits:   %v
%s}`,
		indent, rs.Height, rs.Round, rs.Step,
		indent, rs.StartTime,
		indent, rs.CommitTime,
		indent, rs.Validators.StringIndented(indent+"    "),
		indent, rs.Proposal,
		indent, rs.ProposalBlockParts.StringShort(), rs.ProposalBlock.StringShort(),
		indent, rs.ProposalPOLParts.StringShort(), rs.ProposalPOL.StringShort(),
		indent, rs.LockedBlockParts.StringShort(), rs.LockedBlock.StringShort(),
		indent, rs.LockedPOL.StringShort(),
		indent, rs.Prevotes.StringIndented(indent+"    "),
		indent, rs.Precommits.StringIndented(indent+"    "),
		indent, rs.Commits.StringIndented(indent+"    "),
		indent, rs.LastCommits.StringShort(),
		indent)
}

func (rs *RoundState) StringShort() string {
	return fmt.Sprintf(`RoundState{H:%v R:%v S:%v ST:%v}`,
		rs.Height, rs.Round, rs.Step, rs.StartTime)
}

//-----------------------------------------------------------------------------

// Tracks consensus state across block heights and rounds.
type ConsensusState struct {
	started uint32
	stopped uint32
	quit    chan struct{}

	blockStore     *blk.BlockStore
	mempoolReactor *mempl.MempoolReactor
	runActionCh    chan RoundAction
	newStepCh      chan *RoundState

	mtx sync.Mutex
	RoundState
	state                *sm.State  // State until height-1.
	stagedBlock          *blk.Block // Cache last staged block.
	stagedState          *sm.State  // Cache result of staged block.
	lastCommitVoteHeight uint       // Last called commitVoteBlock() or saveCommitVoteBlock() on.
}

func NewConsensusState(state *sm.State, blockStore *blk.BlockStore, mempoolReactor *mempl.MempoolReactor) *ConsensusState {
	cs := &ConsensusState{
		quit:           make(chan struct{}),
		blockStore:     blockStore,
		mempoolReactor: mempoolReactor,
		runActionCh:    make(chan RoundAction, 1),
		newStepCh:      make(chan *RoundState, 1),
	}
	cs.updateToState(state)
	return cs
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

func (cs *ConsensusState) NewStepCh() chan *RoundState {
	return cs.newStepCh
}

func (cs *ConsensusState) Start() {
	if atomic.CompareAndSwapUint32(&cs.started, 0, 1) {
		log.Info("Starting ConsensusState")
		go cs.stepTransitionRoutine()
	}
}

func (cs *ConsensusState) Stop() {
	if atomic.CompareAndSwapUint32(&cs.stopped, 0, 1) {
		log.Info("Stopping ConsensusState")
		close(cs.quit)
	}
}

func (cs *ConsensusState) IsStopped() bool {
	return atomic.LoadUint32(&cs.stopped) == 1
}

func (cs *ConsensusState) queueAction(ra RoundAction) {
	go func() {
		cs.runActionCh <- ra
	}()
}

// Source of all round state transitions (and votes).
func (cs *ConsensusState) stepTransitionRoutine() {

	// For clarity, all state transitions that happen after some timeout are here.
	// Schedule the next action by pushing a RoundAction{} to cs.runActionCh.
	scheduleNextAction := func() {
		go func() {
			// NOTE: We can push directly to runActionCh because
			// we're running in a separate goroutine, which avoids deadlocks.
			rs := cs.getRoundState()
			round, roundStartTime, roundDuration, _, elapsedRatio := calcRoundInfo(rs.StartTime)
			log.Debug("Scheduling next action", "height", rs.Height, "round", round, "step", rs.Step, "roundStartTime", roundStartTime, "elapsedRatio", elapsedRatio)
			switch rs.Step {
			case RoundStepNewHeight:
				// We should run RoundActionPropose when rs.StartTime passes.
				if elapsedRatio < 0 {
					// startTime is in the future.
					time.Sleep(time.Duration((-1.0 * elapsedRatio) * float64(roundDuration)))
				}
				cs.runActionCh <- RoundAction{rs.Height, rs.Round, RoundActionPropose}
			case RoundStepNewRound:
				// Pseudostep: Immediately goto propose.
				cs.runActionCh <- RoundAction{rs.Height, rs.Round, RoundActionPropose}
			case RoundStepPropose:
				// Wake up when it's time to vote.
				time.Sleep(time.Duration((roundDeadlinePrevote - elapsedRatio) * float64(roundDuration)))
				cs.runActionCh <- RoundAction{rs.Height, rs.Round, RoundActionPrevote}
			case RoundStepPrevote:
				// Wake up when it's time to precommit.
				time.Sleep(time.Duration((roundDeadlinePrecommit - elapsedRatio) * float64(roundDuration)))
				cs.runActionCh <- RoundAction{rs.Height, rs.Round, RoundActionPrecommit}
			case RoundStepPrecommit:
				// Wake up when the round is over.
				time.Sleep(time.Duration((1.0 - elapsedRatio) * float64(roundDuration)))
				cs.runActionCh <- RoundAction{rs.Height, rs.Round, RoundActionTryCommit}
			case RoundStepCommit:
				// There's nothing to scheudle, we're waiting for
				// ProposalBlockParts.IsComplete() &&
				// Commits.HasTwoThirdsMajority()
				panic("The next action from RoundStepCommit is not scheduled by time")
			default:
				panic("Should not happen")
			}
		}()
	}

	scheduleNextAction()

	// NOTE: All ConsensusState.RunAction*() calls come from here.
	// Since only one routine calls them, it is safe to assume that
	// the RoundState Height/Round/Step won't change concurrently.
	// However, other fields like Proposal could change concurrent
	// due to gossip routines.
ACTION_LOOP:
	for {
		var roundAction RoundAction
		select {
		case roundAction = <-cs.runActionCh:
		case <-cs.quit:
			return
		}

		height, round, action := roundAction.Height, roundAction.Round, roundAction.Action
		rs := cs.GetRoundState()

		// Continue if action is not relevant
		if height != rs.Height {
			log.Debug("Discarding round action: Height mismatch", "height", rs.Height, "roundAction", roundAction)
			continue
		}
		// If action <= RoundActionPrecommit, the round must match too.
		if action <= RoundActionPrecommit && round != rs.Round {
			log.Debug("Discarding round action: Round mismatch", "round", rs.Round, "roundAction", roundAction)
			continue
		}

		log.Info("Running round action", "height", rs.Height, "round", rs.Round, "step", rs.Step, "roundAction", roundAction, "startTime", rs.StartTime)

		// Run action
		switch action {
		case RoundActionPropose:
			if rs.Step != RoundStepNewHeight && rs.Step != RoundStepNewRound {
				continue ACTION_LOOP
			}
			cs.RunActionPropose(rs.Height, rs.Round)
			scheduleNextAction()
			continue ACTION_LOOP

		case RoundActionPrevote:
			if rs.Step >= RoundStepPrevote {
				continue ACTION_LOOP
			}
			cs.RunActionPrevote(rs.Height, rs.Round)
			scheduleNextAction()
			continue ACTION_LOOP

		case RoundActionPrecommit:
			if rs.Step >= RoundStepPrecommit {
				continue ACTION_LOOP
			}
			cs.RunActionPrecommit(rs.Height, rs.Round)
			scheduleNextAction()
			continue ACTION_LOOP

		case RoundActionTryCommit:
			if rs.Step >= RoundStepCommit {
				continue ACTION_LOOP
			}
			if rs.Precommits.HasTwoThirdsMajority() {
				// Enter RoundStepCommit and commit.
				cs.RunActionCommit(rs.Height)
				continue ACTION_LOOP
			} else {
				// Could not commit, move onto next round.
				cs.SetupNewRound(rs.Height, rs.Round+1)
				// cs.Step is now at RoundStepNewRound
				scheduleNextAction()
				continue ACTION_LOOP
			}

		case RoundActionCommit:
			if rs.Step >= RoundStepCommit {
				continue ACTION_LOOP
			}
			// Enter RoundStepCommit and commit.
			cs.RunActionCommit(rs.Height)
			continue ACTION_LOOP

		case RoundActionTryFinalize:
			if cs.TryFinalizeCommit(rs.Height) {
				// Now at new height
				// cs.Step is at RoundStepNewHeight or RoundStepNewRound.
				scheduleNextAction()
				continue ACTION_LOOP
			} else {
				// do not schedule next action.
				continue ACTION_LOOP
			}

		default:
			panic("Unknown action")
		}

		// For clarity, ensure that all switch cases call "continue"
		panic("Should not happen.")
	}
}

// Updates ConsensusState and increments height to match that of state.
// If calculated round is greater than 0 (based on BlockTime or calculated StartTime)
// then also sets up the appropriate round, and cs.Step becomes RoundStepNewRound.
// Otherwise the round is 0 and cs.Step becomes RoundStepNewHeight.
func (cs *ConsensusState) updateToState(state *sm.State) {
	// Sanity check state.
	if cs.Height > 0 && cs.Height != state.LastBlockHeight {
		panic(Fmt("updateToState() expected state height of %v but found %v",
			cs.Height, state.LastBlockHeight))
	}

	// Reset fields based on state.
	validators := state.BondedValidators
	height := state.LastBlockHeight + 1 // next desired block height
	cs.Height = height
	cs.Round = 0
	cs.Step = RoundStepNewHeight
	if cs.CommitTime.IsZero() {
		cs.StartTime = state.LastBlockTime.Add(newHeightDelta)
	} else {
		cs.StartTime = cs.CommitTime.Add(newHeightDelta)
	}
	cs.CommitTime = time.Time{}
	cs.Validators = validators
	cs.Proposal = nil
	cs.ProposalBlock = nil
	cs.ProposalBlockParts = nil
	cs.ProposalPOL = nil
	cs.ProposalPOLParts = nil
	cs.LockedBlock = nil
	cs.LockedBlockParts = nil
	cs.LockedPOL = nil
	cs.Prevotes = NewVoteSet(height, 0, blk.VoteTypePrevote, validators)
	cs.Precommits = NewVoteSet(height, 0, blk.VoteTypePrecommit, validators)
	cs.LastCommits = cs.Commits
	cs.Commits = NewVoteSet(height, 0, blk.VoteTypeCommit, validators)

	cs.state = state
	cs.stagedBlock = nil
	cs.stagedState = nil

	// Update the round if we need to.
	round := calcRound(cs.StartTime)
	if round > 0 {
		cs.setupNewRound(round)
	}

	// If we've timed out, then send rebond tx.
	if cs.PrivValidator != nil && cs.state.UnbondingValidators.HasAddress(cs.PrivValidator.Address) {
		rebondTx := &blk.RebondTx{
			Address: cs.PrivValidator.Address,
			Height:  cs.Height + 1,
		}
		err := cs.PrivValidator.SignRebondTx(rebondTx)
		if err == nil {
			log.Info("Signed and broadcast RebondTx", "height", cs.Height, "round", cs.Round, "tx", rebondTx)
			cs.mempoolReactor.BroadcastTx(rebondTx)
		} else {
			log.Warn("Error signing RebondTx", "height", cs.Height, "round", cs.Round, "tx", rebondTx, "error", err)
		}
	}
}

// After the call cs.Step becomes RoundStepNewRound.
func (cs *ConsensusState) setupNewRound(round uint) {
	// Sanity check
	if round == 0 {
		panic("setupNewRound() should never be called for round 0")
	}

	// Increment all the way to round.
	validators := cs.Validators.Copy()
	validators.IncrementAccum(round - cs.Round)

	cs.Round = round
	cs.Step = RoundStepNewRound
	cs.Validators = validators
	cs.Proposal = nil
	cs.ProposalBlock = nil
	cs.ProposalBlockParts = nil
	cs.ProposalPOL = nil
	cs.ProposalPOLParts = nil
	cs.Prevotes = NewVoteSet(cs.Height, round, blk.VoteTypePrevote, validators)
	cs.Prevotes.AddFromCommits(cs.Commits)
	cs.Precommits = NewVoteSet(cs.Height, round, blk.VoteTypePrecommit, validators)
	cs.Precommits.AddFromCommits(cs.Commits)
}

func (cs *ConsensusState) SetPrivValidator(priv *sm.PrivValidator) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	cs.PrivValidator = priv
}

//-----------------------------------------------------------------------------

// Set up the round to desired round and set step to RoundStepNewRound
func (cs *ConsensusState) SetupNewRound(height uint, desiredRound uint) bool {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if cs.Height != height {
		return false
	}
	if desiredRound <= cs.Round {
		return false
	}
	cs.setupNewRound(desiredRound)
	// c.Step is now RoundStepNewRound
	cs.newStepCh <- cs.getRoundState()
	return true
}

func (cs *ConsensusState) RunActionPropose(height uint, round uint) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if cs.Height != height || cs.Round != round {
		return
	}
	defer func() {
		cs.Step = RoundStepPropose
		cs.newStepCh <- cs.getRoundState()
	}()

	// Nothing to do if it's not our turn.
	if cs.PrivValidator == nil {
		return
	}
	if !bytes.Equal(cs.Validators.Proposer().Address, cs.PrivValidator.Address) {
		log.Debug("Not our turn to propose", "proposer", cs.Validators.Proposer().Address, "privValidator", cs.PrivValidator)
		return
	} else {
		log.Debug("Our turn to propose", "proposer", cs.Validators.Proposer().Address, "privValidator", cs.PrivValidator)
	}

	var block *blk.Block
	var blockParts *blk.PartSet
	var pol *POL
	var polParts *blk.PartSet

	// Decide on block and POL
	if cs.LockedBlock != nil {
		// If we're locked onto a block, just choose that.
		block = cs.LockedBlock
		blockParts = cs.LockedBlockParts
		pol = cs.LockedPOL
	} else {
		// Otherwise we should create a new proposal.
		var validation *blk.Validation
		if cs.Height == 1 {
			// We're creating a proposal for the first block.
			// The validation is empty.
			validation = &blk.Validation{}
		} else if cs.LastCommits.HasTwoThirdsMajority() {
			// Make the validation from LastCommits
			validation = cs.LastCommits.MakeValidation()
		} else {
			// Upon reboot, we may have to use SeenValidation
			validation = cs.blockStore.LoadSeenValidation(height - 1)
			if validation == nil {
				// We just don't have any validation for the previous block
				log.Debug("Cannot propose anything: No validation for the previous block.")
				return
			}
		}
		txs := cs.mempoolReactor.Mempool.GetProposalTxs()
		block = &blk.Block{
			Header: &blk.Header{
				Network:        config.App().GetString("Network"),
				Height:         cs.Height,
				Time:           time.Now(),
				Fees:           0, // TODO fees
				NumTxs:         uint(len(txs)),
				LastBlockHash:  cs.state.LastBlockHash,
				LastBlockParts: cs.state.LastBlockParts,
				StateHash:      nil, // Will set afterwards.
			},
			Validation: validation,
			Data: &blk.Data{
				Txs: txs,
			},
		}

		// Set the blk.Header.StateHash.
		err := cs.state.SetBlockStateHash(block)
		if err != nil {
			log.Error("Error setting state hash", "error", err)
			return
		}

		blockParts = blk.NewPartSetFromData(binary.BinaryBytes(block))
		pol = cs.LockedPOL // If exists, is a PoUnlock.
	}

	if pol != nil {
		polParts = blk.NewPartSetFromData(binary.BinaryBytes(pol))
	}

	// Make proposal
	proposal := NewProposal(cs.Height, cs.Round, blockParts.Header(), polParts.Header())
	err := cs.PrivValidator.SignProposal(proposal)
	if err == nil {
		log.Info("Signed and set proposal", "height", cs.Height, "round", cs.Round, "proposal", proposal)
		log.Debug(Fmt("Signed and set proposal block: %v", block))
		// Set fields
		cs.Proposal = proposal
		cs.ProposalBlock = block
		cs.ProposalBlockParts = blockParts
		cs.ProposalPOL = pol
		cs.ProposalPOLParts = polParts
	} else {
		log.Warn("Error signing proposal", "height", cs.Height, "round", cs.Round, "error", err)
	}
}

// Prevote for LockedBlock if we're locked, or ProposealBlock if valid.
// Otherwise vote nil.
func (cs *ConsensusState) RunActionPrevote(height uint, round uint) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if cs.Height != height || cs.Round != round {
		panic(Fmt("RunActionPrevote(%v/%v), expected %v/%v", height, round, cs.Height, cs.Round))
	}
	defer func() {
		cs.Step = RoundStepPrevote
		cs.newStepCh <- cs.getRoundState()
	}()

	// If a block is locked, prevote that.
	if cs.LockedBlock != nil {
		cs.signAddVote(blk.VoteTypePrevote, cs.LockedBlock.Hash(), cs.LockedBlockParts.Header())
		return
	}

	// If ProposalBlock is nil, prevote nil.
	if cs.ProposalBlock == nil {
		log.Warn("ProposalBlock is nil")
		cs.signAddVote(blk.VoteTypePrevote, nil, blk.PartSetHeader{})
		return
	}

	// Try staging cs.ProposalBlock
	err := cs.stageBlock(cs.ProposalBlock, cs.ProposalBlockParts)
	if err != nil {
		// ProposalBlock is invalid, prevote nil.
		log.Warn("ProposalBlock is invalid", "error", err)
		cs.signAddVote(blk.VoteTypePrevote, nil, blk.PartSetHeader{})
		return
	}

	// Prevote cs.ProposalBlock
	cs.signAddVote(blk.VoteTypePrevote, cs.ProposalBlock.Hash(), cs.ProposalBlockParts.Header())
	return
}

// Lock & Precommit the ProposalBlock if we have enough prevotes for it,
// or unlock an existing lock if +2/3 of prevotes were nil.
func (cs *ConsensusState) RunActionPrecommit(height uint, round uint) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if cs.Height != height || cs.Round != round {
		panic(Fmt("RunActionPrecommit(%v/%v), expected %v/%v", height, round, cs.Height, cs.Round))
	}
	defer func() {
		cs.Step = RoundStepPrecommit
		cs.newStepCh <- cs.getRoundState()
	}()

	hash, partsHeader, ok := cs.Prevotes.TwoThirdsMajority()
	if !ok {
		// If we don't have two thirds of prevotes,
		// don't do anything at all.
		return
	}

	// Remember this POL. (hash may be nil)
	cs.LockedPOL = cs.Prevotes.MakePOL()

	// If +2/3 prevoted nil. Just unlock.
	if len(hash) == 0 {
		cs.LockedBlock = nil
		cs.LockedBlockParts = nil
		return
	}

	// If +2/3 prevoted for already locked block, precommit it.
	if cs.LockedBlock.HashesTo(hash) {
		cs.signAddVote(blk.VoteTypePrecommit, hash, partsHeader)
		return
	}

	// If +2/3 prevoted for cs.ProposalBlock, lock it and precommit it.
	if cs.ProposalBlock.HashesTo(hash) {
		// Validate the block.
		if err := cs.stageBlock(cs.ProposalBlock, cs.ProposalBlockParts); err != nil {
			// Prevent zombies.
			log.Warn("+2/3 prevoted for an invalid block", "error", err)
			return
		}
		cs.LockedBlock = cs.ProposalBlock
		cs.LockedBlockParts = cs.ProposalBlockParts
		cs.signAddVote(blk.VoteTypePrecommit, hash, partsHeader)
		return
	}

	// We don't have the block that validators prevoted.
	// Unlock if we're locked.
	cs.LockedBlock = nil
	cs.LockedBlockParts = nil
	return
}

// Enter commit step. See the diagram for details.
// There are two ways to enter this step:
// * After the Precommit step with +2/3 precommits, or,
// * Upon +2/3 commits regardless of current step
// Either way this action is run at most once per round.
func (cs *ConsensusState) RunActionCommit(height uint) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if cs.Height != height {
		panic(Fmt("RunActionCommit(%v), expected %v", height, cs.Height))
	}
	defer func() {
		cs.Step = RoundStepCommit
		cs.newStepCh <- cs.getRoundState()
	}()

	// Sanity check.
	// There are two ways to enter:
	// 1. +2/3 precommits at the end of RoundStepPrecommit
	// 2. +2/3 commits at any time
	hash, partsHeader, ok := cs.Precommits.TwoThirdsMajority()
	if !ok {
		hash, partsHeader, ok = cs.Commits.TwoThirdsMajority()
		if !ok {
			panic("RunActionCommit() expects +2/3 precommits or commits")
		}
	}

	// Clear the Locked* fields and use cs.Proposed*
	if cs.LockedBlock.HashesTo(hash) {
		cs.ProposalBlock = cs.LockedBlock
		cs.ProposalBlockParts = cs.LockedBlockParts
		cs.LockedBlock = nil
		cs.LockedBlockParts = nil
		cs.LockedPOL = nil
	}

	// If we don't have the block being committed, set up to get it.
	if !cs.ProposalBlock.HashesTo(hash) {
		if !cs.ProposalBlockParts.HasHeader(partsHeader) {
			// We're getting the wrong block.
			// Set up ProposalBlockParts and keep waiting.
			cs.ProposalBlock = nil
			cs.ProposalBlockParts = blk.NewPartSetFromHeader(partsHeader)

		} else {
			// We just need to keep waiting.
		}
	} else {
		// We have the block, so sign a Commit-vote.
		cs.commitVoteBlock(cs.ProposalBlock, cs.ProposalBlockParts)
	}

	// If we have the block AND +2/3 commits, queue RoundActionTryFinalize.
	// Round will immediately become finalized.
	if cs.ProposalBlock.HashesTo(hash) && cs.Commits.HasTwoThirdsMajority() {
		cs.queueAction(RoundAction{cs.Height, cs.Round, RoundActionTryFinalize})
	}

}

// Returns true if Finalize happened, which increments height && sets
// the step to RoundStepNewHeight (or RoundStepNewRound, but probably not).
func (cs *ConsensusState) TryFinalizeCommit(height uint) bool {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	if cs.Height != height {
		panic(Fmt("TryFinalizeCommit(%v), expected %v", height, cs.Height))
	}

	if cs.Step == RoundStepCommit &&
		cs.Commits.HasTwoThirdsMajority() &&
		cs.ProposalBlockParts.IsComplete() {

		// Sanity check
		if cs.ProposalBlock == nil {
			panic(Fmt("Expected ProposalBlock to exist"))
		}
		hash, header, _ := cs.Commits.TwoThirdsMajority()
		if !cs.ProposalBlock.HashesTo(hash) {
			panic(Fmt("Expected ProposalBlock to hash to commit hash. Expected %X, got %X", hash, cs.ProposalBlock.Hash()))
		}
		if !cs.ProposalBlockParts.HasHeader(header) {
			panic(Fmt("Expected ProposalBlockParts header to be commit header"))
		}

		err := cs.stageBlock(cs.ProposalBlock, cs.ProposalBlockParts)
		if err == nil {
			log.Debug(Fmt("Finalizing commit of block: %v", cs.ProposalBlock))
			// We have the block, so save/stage/sign-commit-vote.
			cs.saveCommitVoteBlock(cs.ProposalBlock, cs.ProposalBlockParts, cs.Commits)
			// Increment height.
			cs.updateToState(cs.stagedState)
			// cs.Step is now RoundStepNewHeight or RoundStepNewRound
			cs.newStepCh <- cs.getRoundState()
			return true
		} else {
			// Prevent zombies.
			// TODO: Does this ever happen?
			panic(Fmt("+2/3 committed an invalid block: %v", err))
		}
	}
	return false
}

//-----------------------------------------------------------------------------

func (cs *ConsensusState) SetProposal(proposal *Proposal) error {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	// Already have one
	if cs.Proposal != nil {
		return nil
	}

	// Does not apply
	if proposal.Height != cs.Height || proposal.Round != cs.Round {
		return nil
	}

	// We don't care about the proposal if we're already in RoundStepCommit.
	if cs.Step == RoundStepCommit {
		return nil
	}

	// Verify signature
	if !cs.Validators.Proposer().PubKey.VerifyBytes(account.SignBytes(proposal), proposal.Signature) {
		return ErrInvalidProposalSignature
	}

	cs.Proposal = proposal
	cs.ProposalBlockParts = blk.NewPartSetFromHeader(proposal.BlockParts)
	cs.ProposalPOLParts = blk.NewPartSetFromHeader(proposal.POLParts)
	return nil
}

// NOTE: block is not necessarily valid.
// NOTE: This function may increment the height.
func (cs *ConsensusState) AddProposalBlockPart(height uint, round uint, part *blk.Part) (added bool, err error) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

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
		var n int64
		var err error
		cs.ProposalBlock = binary.ReadBinary(&blk.Block{}, cs.ProposalBlockParts.GetReader(), &n, &err).(*blk.Block)
		// If we're already in the commit step, try to finalize round.
		if cs.Step == RoundStepCommit {
			cs.queueAction(RoundAction{cs.Height, cs.Round, RoundActionTryFinalize})
		}
		// XXX If POL is valid, consider unlocking.
		return true, err
	}
	return true, nil
}

// NOTE: POL is not necessarily valid.
func (cs *ConsensusState) AddProposalPOLPart(height uint, round uint, part *blk.Part) (added bool, err error) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	if cs.Height != height || cs.Round != round {
		return false, nil
	}

	// We're not expecting a POL part.
	if cs.ProposalPOLParts == nil {
		return false, nil // TODO: bad peer? Return error?
	}

	added, err = cs.ProposalPOLParts.AddPart(part)
	if err != nil {
		return added, err
	}
	if added && cs.ProposalPOLParts.IsComplete() {
		var n int64
		var err error
		cs.ProposalPOL = binary.ReadBinary(&POL{}, cs.ProposalPOLParts.GetReader(), &n, &err).(*POL)
		return true, err
	}
	return true, nil
}

func (cs *ConsensusState) AddVote(address []byte, vote *blk.Vote) (added bool, index uint, err error) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	return cs.addVote(address, vote)
}

//-----------------------------------------------------------------------------

func (cs *ConsensusState) addVote(address []byte, vote *blk.Vote) (added bool, index uint, err error) {
	switch vote.Type {
	case blk.VoteTypePrevote:
		// Prevotes checks for height+round match.
		return cs.Prevotes.Add(address, vote)
	case blk.VoteTypePrecommit:
		// Precommits checks for height+round match.
		return cs.Precommits.Add(address, vote)
	case blk.VoteTypeCommit:
		if vote.Height == cs.Height {
			// No need to check if vote.Round < cs.Round ...
			// Prevotes && Precommits already checks that.
			cs.Prevotes.Add(address, vote)
			cs.Precommits.Add(address, vote)
			added, index, err = cs.Commits.Add(address, vote)
			if added && cs.Commits.HasTwoThirdsMajority() && cs.CommitTime.IsZero() {
				cs.CommitTime = time.Now()
				log.Debug(Fmt("Set CommitTime to %v", cs.CommitTime))
				if cs.Step < RoundStepCommit {
					cs.queueAction(RoundAction{cs.Height, cs.Round, RoundActionCommit})
				} else {
					cs.queueAction(RoundAction{cs.Height, cs.Round, RoundActionTryFinalize})
				}
			}
			return added, index, err
		}
		if vote.Height+1 == cs.Height {
			return cs.LastCommits.Add(address, vote)
		}
		return false, 0, nil
	default:
		panic("Unknown vote type")
	}
}

func (cs *ConsensusState) stageBlock(block *blk.Block, blockParts *blk.PartSet) error {
	if block == nil {
		panic("Cannot stage nil block")
	}

	// Already staged?
	if cs.stagedBlock == block {
		return nil
	}

	// Create a copy of the state for staging
	stateCopy := cs.state.Copy()

	// Commit block onto the copied state.
	// NOTE: Basic validation is done in state.AppendBlock().
	err := stateCopy.AppendBlock(block, blockParts.Header())
	if err != nil {
		return err
	} else {
		cs.stagedBlock = block
		cs.stagedState = stateCopy
		return nil
	}
}

func (cs *ConsensusState) signAddVote(type_ byte, hash []byte, header blk.PartSetHeader) *blk.Vote {
	if cs.PrivValidator == nil || !cs.Validators.HasAddress(cs.PrivValidator.Address) {
		return nil
	}
	vote := &blk.Vote{
		Height:     cs.Height,
		Round:      cs.Round,
		Type:       type_,
		BlockHash:  hash,
		BlockParts: header,
	}
	err := cs.PrivValidator.SignVote(vote)
	if err == nil {
		log.Info("Signed and added vote", "height", cs.Height, "round", cs.Round, "vote", vote)
		cs.addVote(cs.PrivValidator.Address, vote)
		return vote
	} else {
		log.Warn("Error signing vote", "height", cs.Height, "round", cs.Round, "vote", vote, "error", err)
		return nil
	}
}

// sign a Commit-Vote
func (cs *ConsensusState) commitVoteBlock(block *blk.Block, blockParts *blk.PartSet) {

	// The proposal must be valid.
	if err := cs.stageBlock(block, blockParts); err != nil {
		// Prevent zombies.
		log.Warn("commitVoteBlock() an invalid block", "error", err)
		return
	}

	// Commit-vote.
	if cs.lastCommitVoteHeight < block.Height {
		cs.signAddVote(blk.VoteTypeCommit, block.Hash(), blockParts.Header())
		cs.lastCommitVoteHeight = block.Height
	} else {
		log.Error("Duplicate commitVoteBlock() attempt", "lastCommitVoteHeight", cs.lastCommitVoteHeight, "blk.Height", block.Height)
	}
}

// Save Block, save the +2/3 Commits we've seen,
// and sign a Commit-Vote if we haven't already
func (cs *ConsensusState) saveCommitVoteBlock(block *blk.Block, blockParts *blk.PartSet, commits *VoteSet) {

	// The proposal must be valid.
	if err := cs.stageBlock(block, blockParts); err != nil {
		// Prevent zombies.
		log.Warn("saveCommitVoteBlock() an invalid block", "error", err)
		return
	}

	// Save to blockStore.
	if cs.blockStore.Height() < block.Height {
		seenValidation := commits.MakeValidation()
		cs.blockStore.SaveBlock(block, blockParts, seenValidation)
	}

	// Save the state.
	cs.stagedState.Save()

	// Update mempool.
	cs.mempoolReactor.Mempool.ResetForBlockAndState(block, cs.stagedState)

	// Commit-vote if we haven't already.
	if cs.lastCommitVoteHeight < block.Height {
		cs.signAddVote(blk.VoteTypeCommit, block.Hash(), blockParts.Header())
		cs.lastCommitVoteHeight = block.Height
	}
}

//-----------------------------------------------------------------------------

// total duration of given round
func calcRoundDuration(round uint) time.Duration {
	return roundDuration0 + roundDurationDelta*time.Duration(round)
}

// startTime is when round zero started.
func calcRoundStartTime(round uint, startTime time.Time) time.Time {
	return startTime.Add(roundDuration0*time.Duration(round) +
		roundDurationDelta*(time.Duration((int64(round)*int64(round)-int64(round))/2)))
}

// calculates the current round given startTime of round zero.
// NOTE: round is zero if startTime is in the future.
func calcRound(startTime time.Time) uint {
	now := time.Now()
	if now.Before(startTime) {
		return 0
	}
	// Start  +  D_0 * R  +  D_delta * (R^2 - R)/2  <=  Now; find largest integer R.
	// D_delta * R^2  +  (2D_0 - D_delta) * R  +  2(Start - Now)  <=  0.
	// AR^2 + BR + C <= 0; A = D_delta, B = (2_D0 - D_delta), C = 2(Start - Now).
	// R = Floor((-B + Sqrt(B^2 - 4AC))/2A)
	A := float64(roundDurationDelta)
	B := 2.0*float64(roundDuration0) - float64(roundDurationDelta)
	C := 2.0 * float64(startTime.Sub(now))
	R := math.Floor((-B + math.Sqrt(B*B-4.0*A*C)) / (2 * A))
	if math.IsNaN(R) {
		panic("Could not calc round, should not happen")
	}
	if R > math.MaxInt32 {
		panic(Fmt("Could not calc round, round overflow: %v", R))
	}
	if R < 0 {
		return 0
	}
	return uint(R)
}

// convenience
// NOTE: elapsedRatio can be negative if startTime is in the future.
func calcRoundInfo(startTime time.Time) (round uint, roundStartTime time.Time, roundDuration time.Duration,
	roundElapsed time.Duration, elapsedRatio float64) {
	round = calcRound(startTime)
	roundStartTime = calcRoundStartTime(round, startTime)
	roundDuration = calcRoundDuration(round)
	roundElapsed = time.Now().Sub(roundStartTime)
	elapsedRatio = float64(roundElapsed) / float64(roundDuration)
	return
}
