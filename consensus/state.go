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
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
	. "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/state"
)

type RoundStep uint8
type RoundActionType uint8

const (
	RoundStepNewHeight = RoundStep(0x00) // Round0 for new height started, wait til CommitTime + Delta
	RoundStepNewRound  = RoundStep(0x01) // Pseudostep, immediately goes to RoundStepPropose
	RoundStepPropose   = RoundStep(0x10) // Did propose, gossip proposal
	RoundStepPrevote   = RoundStep(0x11) // Did prevote, gossip prevotes
	RoundStepPrecommit = RoundStep(0x12) // Did precommit, gossip precommits
	RoundStepCommit    = RoundStep(0x20) // Entered commit state machine

	RoundActionPropose     = RoundActionType(0xA0) // Propose and goto RoundStepPropose
	RoundActionPrevote     = RoundActionType(0xA1) // Prevote and goto RoundStepPrevote
	RoundActionPrecommit   = RoundActionType(0xA2) // Precommit and goto RoundStepPrecommit
	RoundActionTryCommit   = RoundActionType(0xC0) // Goto RoundStepCommit, or RoundStepPropose for next round.
	RoundActionTryFinalize = RoundActionType(0xC1) // Maybe goto RoundStepPropose for next round.

	roundDuration0         = 60 * time.Second   // The first round is 60 seconds long.
	roundDurationDelta     = 15 * time.Second   // Each successive round lasts 15 seconds longer.
	roundDeadlinePrevote   = float64(1.0 / 3.0) // When the prevote is due.
	roundDeadlinePrecommit = float64(2.0 / 3.0) // When the precommit vote is due.
	newHeightDelta         = roundDuration0 / 3 // The time to wait between commitTime and startTime of next consensus rounds.
)

var (
	ErrInvalidProposalSignature = errors.New("Error invalid proposal signature")
)

type RoundAction struct {
	Height uint32          // The block height for which consensus is reaching for.
	Round  uint16          // The round number at given height.
	Action RoundActionType // Action to perform.
}

//-----------------------------------------------------------------------------

// Immutable when returned from ConsensusState.GetRoundState()
type RoundState struct {
	Height             uint32 // Height we are working on
	Round              uint16
	Step               RoundStep
	StartTime          time.Time
	CommitTime         time.Time // Time when +2/3 commits were found
	Validators         *state.ValidatorSet
	Proposal           *Proposal
	ProposalBlock      *Block
	ProposalBlockParts *PartSet
	ProposalPOL        *POL
	ProposalPOLParts   *PartSet
	LockedBlock        *Block
	LockedBlockParts   *PartSet
	LockedPOL          *POL // Rarely needed, so no LockedPOLParts.
	Prevotes           *VoteSet
	Precommits         *VoteSet
	Commits            *VoteSet
	LastCommits        *VoteSet
	PrivValidator      *PrivValidator
}

func (rs *RoundState) String() string {
	return rs.StringWithIndent("")
}

func (rs *RoundState) StringWithIndent(indent string) string {
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
		indent, rs.Validators.StringWithIndent(indent+"    "),
		indent, rs.Proposal,
		indent, rs.ProposalBlockParts.Description(), rs.ProposalBlock.Description(),
		indent, rs.ProposalPOLParts.Description(), rs.ProposalPOL.Description(),
		indent, rs.LockedBlockParts.Description(), rs.LockedBlock.Description(),
		indent, rs.LockedPOL.Description(),
		indent, rs.Prevotes.StringWithIndent(indent+"    "),
		indent, rs.Precommits.StringWithIndent(indent+"    "),
		indent, rs.Commits.StringWithIndent(indent+"    "),
		indent, rs.LastCommits.Description(),
		indent)
}

func (rs *RoundState) Description() string {
	return fmt.Sprintf(`RS{%v/%v/%X %v}`,
		rs.Height, rs.Round, rs.Step, rs.StartTime)
}

//-----------------------------------------------------------------------------

// Tracks consensus state across block heights and rounds.
type ConsensusState struct {
	started uint32
	stopped uint32
	quit    chan struct{}

	blockStore  *BlockStore
	mempool     *mempool.Mempool
	runActionCh chan RoundAction
	newStepCh   chan *RoundState

	mtx sync.Mutex
	RoundState
	state       *state.State // State until height-1.
	stagedBlock *Block       // Cache last staged block.
	stagedState *state.State // Cache result of staged block.
}

func NewConsensusState(state *state.State, blockStore *BlockStore, mempool *mempool.Mempool) *ConsensusState {
	cs := &ConsensusState{
		quit:        make(chan struct{}),
		blockStore:  blockStore,
		mempool:     mempool,
		runActionCh: make(chan RoundAction, 1),
		newStepCh:   make(chan *RoundState, 1),
	}
	cs.updateToState(state)
	return cs
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

// Source of all round state transitions (and votes).
func (cs *ConsensusState) stepTransitionRoutine() {

	// For clarity, all state transitions that happen after some timeout are here.
	// Schedule the next action by pushing a RoundAction{} to cs.runActionCh.
	scheduleNextAction := func() {
		go func() {
			rs := cs.getRoundState()
			round, roundStartTime, roundDuration, _, elapsedRatio := calcRoundInfo(rs.StartTime)
			log.Debug("Called scheduleNextAction. round:%v roundStartTime:%v elapsedRatio:%v", round, roundStartTime, elapsedRatio)
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
	// However, other fields like Proposal could change, due to gossip.
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
		log.Info("Running round action A:%X %v", action, rs.Description())

		// Continue if action is not relevant
		if height != rs.Height {
			continue
		}
		// If action <= RoundActionPrecommit, the round must match too.
		if action <= RoundActionPrecommit && round != rs.Round {
			continue
		}

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
				// Maybe finalize already
				cs.runActionCh <- RoundAction{rs.Height, rs.Round, RoundActionTryFinalize}
				continue ACTION_LOOP
			} else {
				// Could not commit, move onto next round.
				cs.SetupNewRound(rs.Height, rs.Round+1)
				// cs.Step is now at RoundStepNewRound
				scheduleNextAction()
				continue ACTION_LOOP
			}

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
func (cs *ConsensusState) updateToState(state *state.State) {
	// Sanity check state.
	if cs.Height > 0 && cs.Height != state.LastBlockHeight {
		Panicf("updateToState() expected state height of %v but found %v",
			cs.Height, state.LastBlockHeight)
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
	cs.Prevotes = NewVoteSet(height, 0, VoteTypePrevote, validators)
	cs.Precommits = NewVoteSet(height, 0, VoteTypePrecommit, validators)
	cs.LastCommits = cs.Commits
	cs.Commits = NewVoteSet(height, 0, VoteTypeCommit, validators)

	cs.state = state
	cs.stagedBlock = nil
	cs.stagedState = nil

	// Update the round if we need to.
	round := calcRound(cs.StartTime)
	if round > 0 {
		cs.setupNewRound(round)
	}
}

// After the call cs.Step becomes RoundStepNewRound.
func (cs *ConsensusState) setupNewRound(round uint16) {
	// Sanity check
	if round == 0 {
		panic("setupNewRound() should never be called for round 0")
	}

	// Increment all the way to round.
	validators := cs.Validators.Copy()
	for r := cs.Round; r < round; r++ {
		validators.IncrementAccum()
	}

	cs.Round = round
	cs.Step = RoundStepNewRound
	cs.Validators = validators
	cs.Proposal = nil
	cs.ProposalBlock = nil
	cs.ProposalBlockParts = nil
	cs.ProposalPOL = nil
	cs.ProposalPOLParts = nil
	cs.Prevotes = NewVoteSet(cs.Height, round, VoteTypePrevote, validators)
	cs.Prevotes.AddFromCommits(cs.Commits)
	cs.Precommits = NewVoteSet(cs.Height, round, VoteTypePrecommit, validators)
	cs.Precommits.AddFromCommits(cs.Commits)
}

func (cs *ConsensusState) SetPrivValidator(priv *PrivValidator) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	cs.PrivValidator = priv
}

//-----------------------------------------------------------------------------

// Set up the round to desired round and set step to RoundStepNewRound
func (cs *ConsensusState) SetupNewRound(height uint32, desiredRound uint16) bool {
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

func (cs *ConsensusState) RunActionPropose(height uint32, round uint16) {
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
	if cs.PrivValidator == nil || cs.Validators.Proposer().Id != cs.PrivValidator.Id {
		return
	}

	var block *Block
	var blockParts *PartSet
	var pol *POL
	var polParts *PartSet

	// Decide on block and POL
	if cs.LockedBlock != nil {
		// If we're locked onto a block, just choose that.
		block = cs.LockedBlock
		blockParts = cs.LockedBlockParts
		pol = cs.LockedPOL
	} else {
		// Otherwise we should create a new proposal.
		var validation *Validation
		if cs.Height == 1 {
			// We're creating a proposal for the first block.
			// The validation is empty.
			validation = &Validation{}
		} else {
			// We need to create a proposal.
			// If we don't have enough commits from the last height,
			// we can't do anything.
			if !cs.LastCommits.HasTwoThirdsMajority() {
				return
			} else {
				validation = cs.LastCommits.MakeValidation()
			}
		}
		txs, state := cs.mempool.GetProposalTxs() // TODO: cache state
		block = &Block{
			Header: &Header{
				Network:        Config.Network,
				Height:         cs.Height,
				Time:           time.Now(),
				Fees:           0, // TODO fees
				LastBlockHash:  cs.state.LastBlockHash,
				LastBlockParts: cs.state.LastBlockParts,
				StateHash:      state.Hash(),
			},
			Validation: validation,
			Data: &Data{
				Txs: txs,
			},
		}
		blockParts = NewPartSetFromData(BinaryBytes(block))
		pol = cs.LockedPOL // If exists, is a PoUnlock.
	}

	if pol != nil {
		polParts = NewPartSetFromData(BinaryBytes(pol))
	}

	// Make proposal
	proposal := NewProposal(cs.Height, cs.Round, blockParts.Header(), polParts.Header())
	cs.PrivValidator.Sign(proposal)

	// Set fields
	cs.Proposal = proposal
	cs.ProposalBlock = block
	cs.ProposalBlockParts = blockParts
	cs.ProposalPOL = pol
	cs.ProposalPOLParts = polParts
}

// Prevote for LockedBlock if we're locked, or ProposealBlock if valid.
// Otherwise vote nil.
func (cs *ConsensusState) RunActionPrevote(height uint32, round uint16) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if cs.Height != height || cs.Round != round {
		Panicf("RunActionPrevote(%v/%v), expected %v/%v", height, round, cs.Height, cs.Round)
	}
	defer func() {
		cs.Step = RoundStepPrevote
		cs.newStepCh <- cs.getRoundState()
	}()

	// If a block is locked, prevote that.
	if cs.LockedBlock != nil {
		cs.signAddVote(VoteTypePrevote, cs.LockedBlock.Hash(), cs.LockedBlockParts.Header())
		return
	}

	// If ProposalBlock is nil, prevote nil.
	if cs.ProposalBlock == nil {
		cs.signAddVote(VoteTypePrevote, nil, PartSetHeader{})
		return
	}

	// Try staging cs.ProposalBlock
	err := cs.stageBlock(cs.ProposalBlock, cs.ProposalBlockParts)
	if err != nil {
		// ProposalBlock is invalid, prevote nil.
		cs.signAddVote(VoteTypePrevote, nil, PartSetHeader{})
		return
	}

	// Prevote cs.ProposalBlock
	cs.signAddVote(VoteTypePrevote, cs.ProposalBlock.Hash(), cs.ProposalBlockParts.Header())
	return
}

// Lock & Precommit the ProposalBlock if we have enough prevotes for it,
// or unlock an existing lock if +2/3 of prevotes were nil.
func (cs *ConsensusState) RunActionPrecommit(height uint32, round uint16) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if cs.Height != height || cs.Round != round {
		Panicf("RunActionPrecommit(%v/%v), expected %v/%v", height, round, cs.Height, cs.Round)
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
		cs.signAddVote(VoteTypePrecommit, hash, partsHeader)
		return
	}

	// If +2/3 prevoted for cs.ProposalBlock, lock it and precommit it.
	if cs.ProposalBlock.HashesTo(hash) {
		// Validate the block.
		if err := cs.stageBlock(cs.ProposalBlock, cs.ProposalBlockParts); err != nil {
			// Prevent zombies.
			log.Warning("+2/3 prevoted for an invalid block: %v", err)
			return
		}
		cs.LockedBlock = cs.ProposalBlock
		cs.LockedBlockParts = cs.ProposalBlockParts
		cs.signAddVote(VoteTypePrecommit, hash, partsHeader)
		return
	}

	// We don't have the block that validators prevoted.
	// Unlock if we're locked.
	cs.LockedBlock = nil
	cs.LockedBlockParts = nil
	return
}

// Enter commit step. See the diagram for details.
func (cs *ConsensusState) RunActionCommit(height uint32) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if cs.Height != height {
		Panicf("RunActionCommit(%v), expected %v", height, cs.Height)
	}
	defer func() {
		cs.Step = RoundStepCommit
		cs.newStepCh <- cs.getRoundState()
	}()

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
			cs.ProposalBlockParts = NewPartSetFromHeader(partsHeader)

		} else {
			// We just need to keep waiting.
		}
	} else {
		// We have the block, so save/stage/sign-commit-vote.

		cs.processBlockForCommit(cs.ProposalBlock, cs.ProposalBlockParts)
	}

	// If we have +2/3 commits, set the CommitTime
	if cs.Commits.HasTwoThirdsMajority() {
		cs.CommitTime = time.Now()
		log.Debug("Set CommitTime to %v", cs.CommitTime)
	}

}

// Returns true if Finalize happened, which increments height && sets
// the step to RoundStepNewHeight (or RoundStepNewRound, but probably not).
func (cs *ConsensusState) TryFinalizeCommit(height uint32) bool {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	if cs.Height != height {
		Panicf("TryFinalizeCommit(%v), expected %v", height, cs.Height)
	}

	if cs.Step == RoundStepCommit &&
		cs.Commits.HasTwoThirdsMajority() &&
		cs.ProposalBlockParts.IsComplete() {

		// Sanity check
		if cs.ProposalBlock == nil {
			Panicf("Expected ProposalBlock to exist")
		}
		hash, header, _ := cs.Commits.TwoThirdsMajority()
		if !cs.ProposalBlock.HashesTo(hash) {
			Panicf("Expected ProposalBlock to hash to commit hash")
		}
		if !cs.ProposalBlockParts.HasHeader(header) {
			Panicf("Expected ProposalBlockParts header to be commit header")
		}

		err := cs.stageBlock(cs.ProposalBlock, cs.ProposalBlockParts)
		if err == nil {
			log.Debug("Finalizing commit of block: %v", cs.ProposalBlock)
			// Increment height.
			cs.updateToState(cs.stagedState)
			// cs.Step is now RoundStepNewHeight or RoundStepNewRound
			cs.newStepCh <- cs.getRoundState()
			return true
		} else {
			// Prevent zombies.
			Panicf("+2/3 committed an invalid block: %v", err)
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
	if !cs.Validators.Proposer().Verify(proposal) {
		return ErrInvalidProposalSignature
	}

	cs.Proposal = proposal
	cs.ProposalBlockParts = NewPartSetFromHeader(proposal.BlockParts)
	cs.ProposalPOLParts = NewPartSetFromHeader(proposal.POLParts)
	return nil
}

// NOTE: block is not necessarily valid.
// NOTE: This function may increment the height.
func (cs *ConsensusState) AddProposalBlockPart(height uint32, round uint16, part *Part) (added bool, err error) {
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
		cs.ProposalBlock = ReadBlock(cs.ProposalBlockParts.GetReader(), &n, &err)
		cs.runActionCh <- RoundAction{cs.Height, cs.Round, RoundActionTryFinalize}
		return true, err
	}
	return true, nil
}

// NOTE: POL is not necessarily valid.
func (cs *ConsensusState) AddProposalPOLPart(height uint32, round uint16, part *Part) (added bool, err error) {
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
		cs.ProposalPOL = ReadPOL(cs.ProposalPOLParts.GetReader(), &n, &err)
		return true, err
	}
	return true, nil
}

func (cs *ConsensusState) AddVote(vote *Vote) (added bool, index uint, err error) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	return cs.addVote(vote)
}

// TODO: Maybe move this out of here?
func (cs *ConsensusState) LoadHeaderValidation(height uint32) (*Header, *Validation) {
	meta := cs.blockStore.LoadBlockMeta(height)
	if meta == nil {
		return nil, nil
	}
	validation := cs.blockStore.LoadBlockValidation(height)
	return meta.Header, validation
}

//-----------------------------------------------------------------------------

func (cs *ConsensusState) addVote(vote *Vote) (added bool, index uint, err error) {
	switch vote.Type {
	case VoteTypePrevote:
		// Prevotes checks for height+round match.
		return cs.Prevotes.Add(vote)
	case VoteTypePrecommit:
		// Precommits checks for height+round match.
		return cs.Precommits.Add(vote)
	case VoteTypeCommit:
		if vote.Height == cs.Height {
			// No need to check if vote.Round < cs.Round ...
			// Prevotes && Precommits already checks that.
			cs.Prevotes.Add(vote)
			cs.Precommits.Add(vote)
			added, index, err = cs.Commits.Add(vote)
			if added && cs.Commits.HasTwoThirdsMajority() {
				cs.runActionCh <- RoundAction{cs.Height, cs.Round, RoundActionTryFinalize}
			}
			return added, index, err
		}
		if vote.Height+1 == cs.Height {
			return cs.LastCommits.Add(vote)
		}
		return false, 0, nil
	default:
		panic("Unknown vote type")
	}
}

func (cs *ConsensusState) stageBlock(block *Block, blockParts *PartSet) error {
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
	err := stateCopy.AppendBlock(block, blockParts.Header(), true)
	if err != nil {
		return err
	} else {
		cs.stagedBlock = block
		cs.stagedState = stateCopy
		return nil
	}
}

func (cs *ConsensusState) signAddVote(type_ byte, hash []byte, header PartSetHeader) *Vote {
	if cs.PrivValidator == nil || !cs.Validators.HasId(cs.PrivValidator.Id) {
		return nil
	}
	vote := &Vote{
		Height:     cs.Height,
		Round:      cs.Round,
		Type:       type_,
		BlockHash:  hash,
		BlockParts: header,
	}
	cs.PrivValidator.Sign(vote)
	cs.addVote(vote)
	return vote
}

func (cs *ConsensusState) processBlockForCommit(block *Block, blockParts *PartSet) {

	// The proposal must be valid.
	if err := cs.stageBlock(block, blockParts); err != nil {
		// Prevent zombies.
		log.Warning("+2/3 precommitted an invalid block: %v", err)
		return
	}

	// Save to blockStore
	cs.blockStore.SaveBlock(block, blockParts)

	// Save the state
	cs.stagedState.Save()

	// Update mempool.
	cs.mempool.ResetForBlockAndState(block, cs.stagedState)

	cs.signAddVote(VoteTypeCommit, block.Hash(), blockParts.Header())
}

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
