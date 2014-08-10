package consensus

import (
	"bytes"
	"sync"
	"time"

	. "github.com/tendermint/tendermint/accounts"
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
	db_ "github.com/tendermint/tendermint/db"
)

var (
	consensusStateKey = []byte("consensusState")
)

/*
Determining the order of proposers at height h:

    A   B   C		  All validators A, B, and C
  [+10, +5, +2] (+17) Voting power

  [  0,  0,  0]       Genesis?
  [ 10,  5,  2] (+17)
A [ -7,  5,  2] (-17) Round 0 proposer: A
  [  3, 10,  4] (+17)
B [  3, -7,  4] (-17) Round 1 proposer: B
  [ 13, -2,  6] (+17)
A [ -4, -2,  6] (-17) Round 2 proposer: A
  [  6,  3,  8] (+17)
C [  6,  3, -9] (-17) Round 3 proposer: C
  [ 16,  8, -7] (+17)
A [ -1,  8, -7] (-17) Round 4 proposer: A
  [  9, 13, -5] (+17)
B [  9, -4, -5] (-17) Round 5 proposer: B
  [ 19,  1, -3] (+17)
A [  2,  1, -3] (-17) Round 6 proposer: A
   ...........   ...

For a node, once consensus has been reached at some round R,
the moment the node sees +2/3 in votes for a proposal is when
the consensus rounds for the *next* height h+1 begins.

Round R+1 in the consensus rounds at height h+1 is the same as
round R   in the consensus rounds at height h (the parent block).

We omit details of dealing with membership changes.
*/

func getProposer(validators map[uint64]*Validator) (proposer *Validator) {
	highestAccum := Int64(0)
	for _, validator := range validators {
		if validator.Accum > highestAccum {
			highestAccum = validator.Accum
			proposer = validator
		} else if validator.Accum == highestAccum {
			if validator.Id < proposer.Id { // Seniority
				proposer = validator
			}
		}
	}
	return
}

func incrementAccum(validators map[uint64]*Validator) {
	totalDelta := UInt64(0)
	for _, validator := range validators {
		validator.Accum += Int64(validator.VotingPower)
		totalDelta += validator.VotingPower
	}
	proposer := getProposer(validators)
	proposer.Accum -= Int64(totalDelta)
	// NOTE: sum(validators) here should be zero.
	if true {
		totalAccum := int64(0)
		for _, validator := range validators {
			totalAccum += int64(validator.Accum)
		}
		if totalAccum != 0 {
			Panicf("Total Accum of validators did not equal 0. Got: ", totalAccum)
		}
	}
}

// Creates a deep copy of validators.
// Caller can then modify the resulting validators' .Accum field without
// modifying the original *Validator's.
func copyValidators(validators map[uint64]*Validator) map[uint64]*Validator {
	mapCopy := map[uint64]*Validator{}
	for _, val := range validators {
		mapCopy[uint64(val.Id)] = val.Copy()
	}
	return mapCopy
}

//-----------------------------------------------------------------------------

// Handles consensus state tracking across block heights.
// NOTE: When adding more fields, also reset it in Load() and CommitBlock()
type ConsensusStateControl struct {
	mtx            sync.Mutex
	db             db_.Db                // Where we store the validators list & other data.
	validatorsR0   map[uint64]*Validator // A copy of the validators at round 0
	privValidator  *PrivValidator        // PrivValidator used to participate, if present.
	accountStore   *AccountStore         // Account storage
	height         uint32                // Height we are working on.
	lockedProposal *BlockPartSet         // A BlockPartSet of the locked proposal.
	startTime      time.Time             // Start of round 0 for this height.
	roundState     *RoundState           // The RoundState object for the current round.
	commits        *VoteSet              // Commits for this height.
}

func NewConsensusStateControl(db db_.Db, accountStore *AccountStore) *ConsensusStateControl {
	csc := &ConsensusStateControl{
		db:           db,
		accountStore: accountStore,
	}
	csc.Load()
	return csc
}

// Load the current state from db.
func (csc *ConsensusStateControl) Load() {
	csc.mtx.Lock()
	defer csc.mtx.Unlock()
	buf := csc.db.Get(consensusStateKey)
	if len(buf) == 0 {
		height := uint32(0)
		validators := make(map[uint64]*Validator) // XXX BOOTSTRAP
		startTime := time.Now()                   // XXX BOOTSTRAP
		csc.setupHeight(height, validators, startTime)
	} else {
		reader := bytes.NewReader(buf)
		height := ReadUInt32(reader)
		validators := make(map[uint64]*Validator)
		startTime := ReadTime(reader)
		for reader.Len() > 0 {
			validator := ReadValidator(reader)
			validators[uint64(validator.Id)] = validator
		}
		csc.setupHeight(uint32(height), validators, startTime.Time)
	}
}

// Save the current state onto db.
// Doesn't save the round state, just initial state at round 0.
func (csc *ConsensusStateControl) Save() {
	csc.mtx.Lock()
	defer csc.mtx.Unlock()
	var buf bytes.Buffer
	UInt32(csc.height).WriteTo(&buf)
	Time{csc.startTime}.WriteTo(&buf)
	for _, validator := range csc.validatorsR0 {
		validator.WriteTo(&buf)
	}
	csc.db.Set(consensusStateKey, buf.Bytes())
}

// Finds more blocks from blockStore and commits them.
func (csc *ConsensusStateControl) Update(blockStore *BlockStore) {
	csc.mtx.Lock()
	defer csc.mtx.Unlock()
	for h := csc.height + 1; h <= blockStore.Height(); h++ {
		block := blockStore.LoadBlock(h)
		// TODO: would be better to be able to override
		// the block commit time, but in the meantime,
		// just use the block time as proposed by the proposer.
		csc.CommitBlock(block, block.Header.Time.Time)
	}
}

func (csc *ConsensusStateControl) PrivValidator() *PrivValidator {
	csc.mtx.Lock()
	defer csc.mtx.Unlock()
	return csc.privValidator
}

func (csc *ConsensusStateControl) SetPrivValidator(privValidator *PrivValidator) error {
	csc.mtx.Lock()
	defer csc.mtx.Unlock()
	if csc.privValidator != nil {
		panic("ConsensusStateControl privValidator already set.")
	}
	csc.privValidator = privValidator
	return nil
}

// Set blockPartSet to nil to unlock.
func (csc *ConsensusStateControl) LockProposal(blockPartSet *BlockPartSet) {
	csc.mtx.Lock()
	defer csc.mtx.Unlock()
	csc.lockedProposal = blockPartSet
}

func (csc *ConsensusStateControl) LockedProposal() *BlockPartSet {
	csc.mtx.Lock()
	defer csc.mtx.Unlock()
	return csc.lockedProposal
}

func (csc *ConsensusStateControl) StageBlock(block *Block) error {
	// XXX implement staging.
	return nil
}

// NOTE: assumes that block is valid.
// NOTE: the block should be saved on the BlockStore before commiting here.
// commitTime is usually set to the system clock time (time.Now()).
func (csc *ConsensusStateControl) CommitBlock(block *Block, commitTime time.Time) error {
	csc.mtx.Lock()
	defer csc.mtx.Unlock()
	// Ensure that block is the next block needed.
	if uint32(block.Height) != csc.height {
		return Errorf("Cannot commit block %v to csc. Expected height %v", block, csc.height+1)
	}
	// Update validator.
	validators := copyValidators(csc.validatorsR0)
	incrementAccum(validators)
	// TODO if there are new validators in the block, add them.

	// XXX: it's not commitTime we want...
	csc.setupHeight(uint32(block.Height)+1, validators, commitTime)

	// Save the state.
	csc.Save()

	return nil
}

func (csc *ConsensusStateControl) RoundState() *RoundState {
	csc.mtx.Lock()
	defer csc.mtx.Unlock()
	return csc.roundState
}

func (csc *ConsensusStateControl) setupHeight(height uint32, validators map[uint64]*Validator, startTime time.Time) {

	if height > 0 && height != csc.height+1 {
		panic("setupHeight() cannot skip heights")
	}

	// Reset the state for the next height.
	csc.validatorsR0 = validators
	csc.height = height
	csc.lockedProposal = nil
	csc.startTime = startTime
	csc.commits = NewVoteSet(height, 0, VoteTypeCommit, validators)

	// Setup the roundState
	csc.roundState = nil
	csc.setupRound(0)

}

// If csc.roundSTate isn't at round, set up new roundState at round.
func (csc *ConsensusStateControl) SetupRound(round uint16) {
	csc.mtx.Lock()
	defer csc.mtx.Unlock()
	if csc.roundState != nil && csc.roundState.Round >= round {
		return
	}
	csc.setupRound(round)
}

// Initialize roundState for given round.
// Involves incrementing validators for each past rand.
func (csc *ConsensusStateControl) setupRound(round uint16) {
	// Increment validator accums as necessary.
	// We need to start with csc.validatorsR0 or csc.roundState.Validators
	var validators map[uint64]*Validator = nil
	var validatorsRound uint16
	if csc.roundState == nil {
		// We have no roundState so we start from validatorsR0 at round 0.
		validators = copyValidators(csc.validatorsR0)
		validatorsRound = 0
	} else {
		// We have a previous roundState so we start from that.
		validators = copyValidators(csc.roundState.Validators)
		validatorsRound = csc.roundState.Round
	}
	// Increment all the way to round.
	for r := validatorsRound; r < round; r++ {
		incrementAccum(validators)
	}

	roundState := NewRoundState(csc.height, round, csc.startTime, validators, csc.commits)
	csc.roundState = roundState
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
	Height          uint32                // Immutable
	Round           uint16                // Immutable
	StartTime       time.Time             // Time in which consensus started for this height.
	Expires         time.Time             // Time after which this round is expired.
	Proposer        *Validator            // The proposer to propose a block for this round.
	Validators      map[uint64]*Validator // All validators with modified accumPower for this round.
	BlockPartSet    *BlockPartSet         // All block parts received for this round.
	RoundBareVotes  *VoteSet              // All votes received for this round.
	RoundPrecommits *VoteSet              // All precommits received for this round.
	Commits         *VoteSet              // A shared object for all commit votes of this height.

	mtx  sync.Mutex
	step uint8 // mutable
}

func NewRoundState(height uint32, round uint16, startTime time.Time,
	validators map[uint64]*Validator, commits *VoteSet) *RoundState {

	proposer := getProposer(validators)
	blockPartSet := NewBlockPartSet(height, round, &(proposer.Account))
	roundBareVotes := NewVoteSet(height, round, VoteTypeBare, validators)
	roundPrecommits := NewVoteSet(height, round, VoteTypePrecommit, validators)

	rs := &RoundState{
		Height:          height,
		Round:           round,
		StartTime:       startTime,
		Expires:         calcRoundStartTime(round+1, startTime),
		Proposer:        proposer,
		Validators:      validators,
		BlockPartSet:    blockPartSet,
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
