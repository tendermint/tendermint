package consensus

import (
	"errors"
	"sync"
	"time"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
	. "github.com/tendermint/tendermint/mempool"
	. "github.com/tendermint/tendermint/state"
)

const (
	RoundStepStart     = uint8(0x00) // Round started.
	RoundStepPropose   = uint8(0x01) // Did propose, broadcasting proposal.
	RoundStepVote      = uint8(0x02) // Did vote, broadcasting votes.
	RoundStepPrecommit = uint8(0x03) // Did precommit, broadcasting precommits.
	RoundStepCommit    = uint8(0x04) // We committed at this round -- do not progress to the next round.
)

var (
	ErrInvalidProposalSignature = errors.New("Error invalid proposal signature")

	consensusStateKey = []byte("consensusState")
)

// Immutable when returned from ConsensusState.GetRoundState()
type RoundState struct {
	Height               uint32 // Height we are working on
	Round                uint16
	Step                 uint8
	StartTime            time.Time
	Validators           *ValidatorSet
	Proposer             *Validator
	Proposal             *Proposal
	ProposalBlock        *Block
	ProposalBlockPartSet *PartSet
	ProposalPOL          *POL
	ProposalPOLPartSet   *PartSet
	LockedBlock          *Block
	LockedPOL            *POL
	Votes                *VoteSet
	Precommits           *VoteSet
	Commits              *VoteSet
	PrivValidator        *PrivValidator
}

//-------------------------------------

// Tracks consensus state across block heights and rounds.
type ConsensusState struct {
	mtx sync.Mutex
	RoundState

	blockStore *BlockStore
	mempool    *Mempool

	state       *State // State until height-1.
	stagedBlock *Block // Cache last staged block.
	stagedState *State // Cache result of staged block.
}

func NewConsensusState(state *State, blockStore *BlockStore, mempool *Mempool) *ConsensusState {
	cs := &ConsensusState{
		blockStore: blockStore,
		mempool:    mempool,
	}
	cs.updateToState(state)
	return cs
}

func (cs *ConsensusState) GetRoundState() *RoundState {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	rs := cs.RoundState // copy
	return &rs
}

func (cs *ConsensusState) updateToState(state *State) {
	// Sanity check state.
	stateHeight := state.Height()
	if stateHeight > 0 && stateHeight != cs.Height+1 {
		Panicf("updateToState() expected state height of %v but found %v", cs.Height+1, stateHeight)
	}

	// Reset fields based on state.
	height := state.Height()
	validators := state.Validators()
	cs.Height = height
	cs.Round = 0
	cs.Step = RoundStepStart
	cs.StartTime = state.CommitTime().Add(newBlockWaitDuration)
	cs.Validators = validators
	cs.Proposer = validators.GetProposer()
	cs.Proposal = nil
	cs.ProposalBlock = nil
	cs.ProposalBlockPartSet = nil
	cs.ProposalPOL = nil
	cs.ProposalPOLPartSet = nil
	cs.LockedBlock = nil
	cs.LockedPOL = nil
	cs.Votes = NewVoteSet(height, 0, VoteTypeBare, validators)
	cs.Precommits = NewVoteSet(height, 0, VoteTypePrecommit, validators)
	cs.Commits = NewVoteSet(height, 0, VoteTypeCommit, validators)

	cs.stagedBlock = nil
	cs.stagedState = nil

	// Update the round if we need to.
	round := calcRound(cs.StartTime)
	if round > 0 {
		cs.setupRound(round)
	}
}

func (cs *ConsensusState) SetupRound(round uint16) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if cs.Round >= round {
		Panicf("ConsensusState round %v not lower than desired round %v", cs.Round, round)
	}
	cs.setupRound(round)
}

func (cs *ConsensusState) setupRound(round uint16) {

	// Increment all the way to round.
	validators := cs.Validators.Copy()
	for r := cs.Round; r < round; r++ {
		validators.IncrementAccum()
	}

	cs.Round = round
	cs.Step = RoundStepStart
	cs.Validators = validators
	cs.Proposer = validators.GetProposer()
	cs.Proposal = nil
	cs.ProposalBlock = nil
	cs.ProposalBlockPartSet = nil
	cs.ProposalPOL = nil
	cs.ProposalPOLPartSet = nil
	cs.Votes = NewVoteSet(cs.Height, round, VoteTypeBare, validators)
	cs.Votes.AddVotesFromCommits(cs.Commits)
	cs.Precommits = NewVoteSet(cs.Height, round, VoteTypePrecommit, validators)
	cs.Precommits.AddVotesFromCommits(cs.Commits)
}

func (cs *ConsensusState) SetStep(step byte) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if cs.Step < step {
		cs.Step = step
	} else {
		panic("step regression")
	}
}

func (cs *ConsensusState) SetPrivValidator(priv *PrivValidator) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	cs.PrivValidator = priv
}

func (cs *ConsensusState) SetProposal(proposal *Proposal) error {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	// Already have one
	if cs.Proposal != nil {
		return nil
	}

	// Invalid.
	if proposal.Height != cs.Height || proposal.Round != cs.Round {
		return nil
	}

	// Verify signature
	if !cs.Proposer.Verify(proposal.GenDocument(), proposal.Signature) {
		return ErrInvalidProposalSignature
	}

	cs.Proposal = proposal
	cs.ProposalBlockPartSet = NewPartSetFromMetadata(proposal.BlockPartsTotal, proposal.BlockPartsHash)
	cs.ProposalPOLPartSet = NewPartSetFromMetadata(proposal.POLPartsTotal, proposal.POLPartsHash)
	return nil
}

func (cs *ConsensusState) MakeProposal() {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	if cs.PrivValidator == nil || cs.Proposer.Id != cs.PrivValidator.Id {
		return
	}

	var block *Block
	var blockPartSet *PartSet
	var pol *POL
	var polPartSet *PartSet

	// Decide on block and POL
	if cs.LockedBlock != nil {
		// If we're locked onto a block, just choose that.
		block = cs.LockedBlock
		pol = cs.LockedPOL
	} else {
		// TODO: make use of state returned from MakeProposalBlock()
		block, _ = cs.mempool.MakeProposalBlock()
		pol = cs.LockedPOL // If exists, is a PoUnlock.
	}

	blockPartSet = NewPartSetFromData(BinaryBytes(block))
	if pol != nil {
		polPartSet = NewPartSetFromData(BinaryBytes(pol))
	}

	// Make proposal
	proposal := NewProposal(cs.Height, cs.Round, blockPartSet.Total(), blockPartSet.RootHash(),
		polPartSet.Total(), polPartSet.RootHash())
	cs.PrivValidator.SignProposal(proposal)

	// Set fields
	cs.Proposal = proposal
	cs.ProposalBlock = block
	cs.ProposalBlockPartSet = blockPartSet
	cs.ProposalPOL = pol
	cs.ProposalPOLPartSet = polPartSet
}

// NOTE: block is not necessarily valid.
func (cs *ConsensusState) AddProposalBlockPart(height uint32, round uint16, part *Part) (added bool, err error) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	// Blocks might be reused, so round mismatch is OK
	if cs.Height != height {
		return false, nil
	}

	// We're not expecting a block part.
	if cs.ProposalBlockPartSet != nil {
		return false, nil // TODO: bad peer? Return error?
	}

	added, err = cs.ProposalBlockPartSet.AddPart(part)
	if err != nil {
		return added, err
	}
	if added && cs.ProposalBlockPartSet.IsComplete() {
		var n int64
		var err error
		cs.ProposalBlock = ReadBlock(cs.ProposalBlockPartSet.GetReader(), &n, &err)
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
	if cs.ProposalPOLPartSet != nil {
		return false, nil // TODO: bad peer? Return error?
	}

	added, err = cs.ProposalPOLPartSet.AddPart(part)
	if err != nil {
		return added, err
	}
	if added && cs.ProposalPOLPartSet.IsComplete() {
		var n int64
		var err error
		cs.ProposalPOL = ReadPOL(cs.ProposalPOLPartSet.GetReader(), &n, &err)
		return true, err
	}
	return true, nil
}

func (cs *ConsensusState) AddVote(vote *Vote) (added bool, err error) {
	switch vote.Type {
	case VoteTypeBare:
		// Votes checks for height+round match.
		return cs.Votes.AddVote(vote)
	case VoteTypePrecommit:
		// Precommits checks for height+round match.
		return cs.Precommits.AddVote(vote)
	case VoteTypeCommit:
		// Commits checks for height match.
		cs.Votes.AddVote(vote)
		cs.Precommits.AddVote(vote)
		return cs.Commits.AddVote(vote)
	default:
		panic("Unknown vote type")
	}
}

// Lock the ProposalBlock if we have enough votes for it,
// or unlock an existing lock if +2/3 of votes were nil.
// Returns a blockhash if a block was locked.
func (cs *ConsensusState) LockOrUnlock(height uint32, round uint16) []byte {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	if cs.Height != height || cs.Round != round {
		return nil
	}

	if hash, _, ok := cs.Votes.TwoThirdsMajority(); ok {

		// Remember this POL. (hash may be nil)
		cs.LockedPOL = cs.Votes.MakePOL()

		if len(hash) == 0 {
			// +2/3 voted nil. Just unlock.
			cs.LockedBlock = nil
			return nil
		} else if cs.ProposalBlock.HashesTo(hash) {
			// +2/3 voted for proposal block
			// Validate the block.
			// See note on ZombieValidators to see why.
			if cs.stageBlock(cs.ProposalBlock) != nil {
				log.Warning("+2/3 voted for an invalid block.")
				return nil
			}
			cs.LockedBlock = cs.ProposalBlock
			return hash
		} else if cs.LockedBlock.HashesTo(hash) {
			// +2/3 voted for already locked block
			// cs.LockedBlock = cs.LockedBlock
			return hash
		} else {
			// We don't have the block that hashes to hash.
			// Unlock if we're locked.
			cs.LockedBlock = nil
			return nil
		}
	} else {
		return nil
	}
}

func (cs *ConsensusState) Commit(height uint32, round uint16) *Block {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	if cs.Height != height || cs.Round != round {
		return nil
	}

	if hash, commitTime, ok := cs.Precommits.TwoThirdsMajority(); ok {

		// There are some strange cases that shouldn't happen
		// (unless voters are duplicitous).
		// For example, the hash may not be the one that was
		// proposed this round.  These cases should be identified
		// and warn the administrator.  We should err on the side of
		// caution and not, for example, sign a block.
		// TODO: Identify these strange cases.

		var block *Block
		if cs.LockedBlock.HashesTo(hash) {
			block = cs.LockedBlock
		} else if cs.ProposalBlock.HashesTo(hash) {
			block = cs.ProposalBlock
		} else {
			return nil
		}

		// The proposal must be valid.
		if err := cs.stageBlock(block); err != nil {
			log.Warning("Network is commiting an invalid proposal? %v", err)
			return nil
		}

		// Save to blockStore
		err := cs.blockStore.SaveBlock(block)
		if err != nil {
			return nil
		}

		// What was staged becomes committed.
		state := cs.stagedState
		state.Save(commitTime)
		cs.updateToState(state)

		// Update mempool.
		cs.mempool.ResetForBlockAndState(block, state)

		return block
	}

	return nil
}

func (cs *ConsensusState) stageBlock(block *Block) error {

	// Already staged?
	if cs.stagedBlock == block {
		return nil
	}

	// Basic validation is done in state.CommitBlock().
	//err := block.ValidateBasic()
	//if err != nil {
	//	return err
	//}

	// Create a copy of the state for staging
	stateCopy := cs.state.Copy() // Deep copy the state before staging.

	// Commit block onto the copied state.
	err := stateCopy.CommitBlock(block)
	if err != nil {
		return err
	} else {
		cs.stagedBlock = block
		cs.stagedState = stateCopy
		return nil
	}
}
