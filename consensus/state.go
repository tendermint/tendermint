package consensus

import (
	"sync"
	"time"

	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
	. "github.com/tendermint/tendermint/state"
)

var (
	consensusStateKey = []byte("consensusState")
)

// Tracks consensus state across block heights and rounds.
type ConsensusState struct {
	mtx            sync.Mutex
	height         uint32        // Height we are working on.
	validatorsR0   *ValidatorSet // A copy of the validators at round 0
	lockedProposal *BlockPartSet // A BlockPartSet of the locked proposal.
	startTime      time.Time     // Start of round 0 for this height.
	commits        *VoteSet      // Commits for this height.
	roundState     *RoundState   // The RoundState object for the current round.
	commitTime     time.Time     // Time at which a block was found to be committed by +2/3.
}

func NewConsensusState(state *State) *ConsensusState {
	cs := &ConsensusState{}
	cs.Update(state)
	return cs
}

func (cs *ConsensusState) LockProposal(blockPartSet *BlockPartSet) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	cs.lockedProposal = blockPartSet
}

func (cs *ConsensusState) UnlockProposal() {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	cs.lockedProposal = nil
}

func (cs *ConsensusState) LockedProposal() *BlockPartSet {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	return cs.lockedProposal
}

func (cs *ConsensusState) RoundState() *RoundState {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	return cs.roundState
}

// Primarily gets called upon block commit by ConsensusManager.
func (cs *ConsensusState) Update(state *State) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	// Sanity check state.
	stateHeight := state.Height()
	if stateHeight > 0 && stateHeight != cs.height+1 {
		Panicf("Update() expected state height of %v but found %v", cs.height+1, stateHeight)
	}

	// Reset fields based on state.
	cs.height = stateHeight
	cs.validatorsR0 = state.Validators().Copy() // NOTE: immutable.
	cs.lockedProposal = nil
	cs.startTime = state.CommitTime().Add(newBlockWaitDuration) // NOTE: likely future time.
	cs.commits = NewVoteSet(stateHeight, 0, VoteTypeCommit, cs.validatorsR0)

	// Setup the roundState
	cs.roundState = nil
	cs.setupRound(0)

}

// If cs.roundState isn't at round, set up new roundState at round.
func (cs *ConsensusState) SetupRound(round uint16) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if cs.roundState != nil && cs.roundState.Round >= round {
		return
	}
	cs.setupRound(round)
}

// Initialize roundState for given round.
// Involves incrementing validators for each past rand.
func (cs *ConsensusState) setupRound(round uint16) {
	// Increment validator accums as necessary.
	// We need to start with cs.validatorsR0 or cs.roundState.Validators
	var validators *ValidatorSet
	var validatorsRound uint16
	if cs.roundState == nil {
		// We have no roundState so we start from validatorsR0 at round 0.
		validators = cs.validatorsR0.Copy()
		validatorsRound = 0
	} else {
		// We have a previous roundState so we start from that.
		validators = cs.roundState.Validators.Copy()
		validatorsRound = cs.roundState.Round
	}
	// Increment all the way to round.
	for r := validatorsRound; r < round; r++ {
		validators.IncrementAccum()
	}

	roundState := NewRoundState(cs.height, round, cs.startTime, validators, cs.commits)
	cs.roundState = roundState
}

//-----------------------------------------------------------------------------

const (
	RoundStepStart          = uint8(0x00) // Round started.
	RoundStepProposal       = uint8(0x01) // Did propose, broadcasting proposal.
	RoundStepBareVotes      = uint8(0x02) // Did vote bare, broadcasting bare votes.
	RoundStepPrecommits     = uint8(0x03) // Did precommit, broadcasting precommits.
	RoundStepCommitOrUnlock = uint8(0x04) // We committed at this round -- do not progress to the next round.
)

//-----------------------------------------------------------------------------

// RoundState encapsulates all the state needed to engage in the consensus protocol.
type RoundState struct {
	Height          uint32        // Immutable
	Round           uint16        // Immutable
	StartTime       time.Time     // Time in which consensus started for this height.
	Expires         time.Time     // Time after which this round is expired.
	Proposer        *Validator    // The proposer to propose a block for this round.
	Validators      *ValidatorSet // All validators with modified accumPower for this round.
	Proposal        *BlockPartSet // All block parts received for this round.
	RoundBareVotes  *VoteSet      // All votes received for this round.
	RoundPrecommits *VoteSet      // All precommits received for this round.
	Commits         *VoteSet      // A shared object for all commit votes of this height.

	mtx  sync.Mutex
	step uint8 // mutable
}

func NewRoundState(height uint32, round uint16, startTime time.Time,
	validators *ValidatorSet, commits *VoteSet) *RoundState {

	proposer := validators.GetProposer()
	blockPartSet := NewBlockPartSet(height, nil)
	roundBareVotes := NewVoteSet(height, round, VoteTypeBare, validators)
	roundPrecommits := NewVoteSet(height, round, VoteTypePrecommit, validators)

	rs := &RoundState{
		Height:          height,
		Round:           round,
		StartTime:       startTime,
		Expires:         calcRoundStartTime(round+1, startTime),
		Proposer:        proposer,
		Validators:      validators,
		Proposal:        blockPartSet,
		RoundBareVotes:  roundBareVotes,
		RoundPrecommits: roundPrecommits,
		Commits:         commits,

		step: RoundStepStart,
	}
	return rs
}

func (rs *RoundState) AddVote(vote *Vote) (bool, error) {
	switch vote.Type {
	case VoteTypeBare:
		return rs.RoundBareVotes.AddVote(vote)
	case VoteTypePrecommit:
		return rs.RoundPrecommits.AddVote(vote)
	case VoteTypeCommit:
		return rs.Commits.AddVote(vote)
	default:
		panic("Unknown vote type")
	}
}

func (rs *RoundState) Expired() bool {
	return time.Now().After(rs.Expires)
}

func (rs *RoundState) Step() uint8 {
	rs.mtx.Lock()
	defer rs.mtx.Unlock()
	return rs.step
}

func (rs *RoundState) SetStep(step uint8) bool {
	rs.mtx.Lock()
	defer rs.mtx.Unlock()
	if rs.step < step {
		rs.step = step
		return true
	} else {
		return false
	}
}
