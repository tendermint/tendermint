package consensus

import (
	"errors"
	"fmt"
	"sync"
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
	RoundStepStart     = RoundStep(0x00) // Round started.
	RoundStepPropose   = RoundStep(0x01) // Did propose, gossip proposal.
	RoundStepPrevote   = RoundStep(0x02) // Did prevote, gossip prevotes.
	RoundStepPrecommit = RoundStep(0x03) // Did precommit, gossip precommits.
	RoundStepCommit    = RoundStep(0x04) // Did commit, gossip commits.

	// If a block could not be committed at a given round,
	// we progress to the next round, skipping RoundStepCommit.
	//
	// If a block was committed, we goto RoundStepCommit,
	// then wait "finalizeDuration" to gather more commits,
	// then we progress to the next height at round 0.

	RoundActionPropose   = RoundActionType(0x00) // Goto RoundStepPropose
	RoundActionPrevote   = RoundActionType(0x01) // Goto RoundStepPrevote
	RoundActionPrecommit = RoundActionType(0x02) // Goto RoundStepPrecommit
	RoundActionCommit    = RoundActionType(0x03) // Goto RoundStepCommit or RoundStepStart next round
	RoundActionFinalize  = RoundActionType(0x04) // Goto RoundStepStart next height
)

var (
	ErrInvalidProposalSignature = errors.New("Error invalid proposal signature")

	consensusStateKey = []byte("consensusState")
)

// Immutable when returned from ConsensusState.GetRoundState()
type RoundState struct {
	Height               uint32 // Height we are working on
	Round                uint16
	Step                 RoundStep
	StartTime            time.Time
	Validators           *state.ValidatorSet
	Proposal             *Proposal
	ProposalBlock        *Block
	ProposalBlockPartSet *PartSet
	ProposalPOL          *POL
	ProposalPOLPartSet   *PartSet
	LockedBlock          *Block
	LockedPOL            *POL
	Prevotes             *VoteSet
	Precommits           *VoteSet
	Commits              *VoteSet
	LastCommits          *VoteSet
	PrivValidator        *PrivValidator
}

func (rs *RoundState) RoundElapsed() time.Duration {
	return rs.StartTime.Sub(time.Now())
}

func (rs *RoundState) String() string {
	return rs.StringWithIndent("")
}

func (rs *RoundState) StringWithIndent(indent string) string {
	return fmt.Sprintf(`RoundState{
%s  H:%v R:%v S:%v
%s  StartTime:     %v
%s  Validators:    %v
%s  Proposal:      %v
%s  ProposalBlock: %v %v
%s  ProposalPOL:   %v %v
%s  LockedBlock:   %v
%s  LockedPOL:     %v
%s  Prevotes:      %v
%s  Precommits:    %v
%s  Commits:       %v
%s  LastCommits:   %v
%s}`,
		indent, rs.Height, rs.Round, rs.Step,
		indent, rs.StartTime,
		indent, rs.Validators.StringWithIndent(indent+"    "),
		indent, rs.Proposal,
		indent, rs.ProposalBlockPartSet.Description(), rs.ProposalBlock.Description(),
		indent, rs.ProposalPOLPartSet.Description(), rs.ProposalPOL.Description(),
		indent, rs.LockedBlock.Description(),
		indent, rs.LockedPOL.Description(),
		indent, rs.Prevotes.StringWithIndent(indent+"    "),
		indent, rs.Precommits.StringWithIndent(indent+"    "),
		indent, rs.Commits.StringWithIndent(indent+"    "),
		indent, rs.LastCommits.StringWithIndent(indent+"    "),
		indent)
}

//-------------------------------------

// Tracks consensus state across block heights and rounds.
type ConsensusState struct {
	blockStore *BlockStore
	mempool    *mempool.Mempool

	mtx sync.Mutex
	RoundState
	state       *state.State // State until height-1.
	stagedBlock *Block       // Cache last staged block.
	stagedState *state.State // Cache result of staged block.
}

func NewConsensusState(state *state.State, blockStore *BlockStore, mempool *mempool.Mempool) *ConsensusState {
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

func (cs *ConsensusState) updateToState(state *state.State) {
	// Sanity check state.
	if cs.Height > 0 && cs.Height != state.Height {
		Panicf("updateToState() expected state height of %v but found %v",
			cs.Height, state.Height)
	}

	// Reset fields based on state.
	validators := state.BondedValidators
	height := state.Height + 1 // next desired block height
	cs.Height = height
	cs.Round = 0
	cs.Step = RoundStepStart
	cs.StartTime = state.CommitTime.Add(finalizeDuration)
	cs.Validators = validators
	cs.Proposal = nil
	cs.ProposalBlock = nil
	cs.ProposalBlockPartSet = nil
	cs.ProposalPOL = nil
	cs.ProposalPOLPartSet = nil
	cs.LockedBlock = nil
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
	cs.Proposal = nil
	cs.ProposalBlock = nil
	cs.ProposalBlockPartSet = nil
	cs.ProposalPOL = nil
	cs.ProposalPOLPartSet = nil
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
	if !cs.Validators.Proposer().Verify(proposal) {
		return ErrInvalidProposalSignature
	}

	cs.Proposal = proposal
	cs.ProposalBlockPartSet = NewPartSetFromMetadata(proposal.BlockPartsTotal, proposal.BlockPartsHash)
	cs.ProposalPOLPartSet = NewPartSetFromMetadata(proposal.POLPartsTotal, proposal.POLPartsHash)
	return nil
}

func (cs *ConsensusState) RunActionPropose(height uint32, round uint16) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if cs.Height != height || cs.Round != round {
		return
	}
	cs.Step = RoundStepPropose

	if cs.PrivValidator == nil || cs.Validators.Proposer().Id != cs.PrivValidator.Id {
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
		var validation Validation
		if cs.Height == 1 {
			// We're creating a proposal for the first block.
			// The validation is empty.
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
			Header: Header{
				Network:       Config.Network,
				Height:        cs.Height,
				Time:          time.Now(),
				LastBlockHash: cs.state.BlockHash,
				StateHash:     state.Hash(),
			},
			Validation: validation,
			Data: Data{
				Txs: txs,
			},
		}
		pol = cs.LockedPOL // If exists, is a PoUnlock.
	}

	blockPartSet = NewPartSetFromData(BinaryBytes(block))
	if pol != nil {
		polPartSet = NewPartSetFromData(BinaryBytes(pol))
	} else {

	}

	// Make proposal
	proposal := NewProposal(cs.Height, cs.Round,
		blockPartSet.Total(), blockPartSet.RootHash(),
		polPartSet.Total(), polPartSet.RootHash())
	cs.PrivValidator.Sign(proposal)

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

func (cs *ConsensusState) RunActionPrevote(height uint32, round uint16) []byte {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if cs.Height != height || cs.Round != round {
		return nil
	}
	cs.Step = RoundStepPrevote

	// If a block is locked, prevote that.
	if cs.LockedBlock != nil {
		return cs.LockedBlock.Hash()
	}
	// Try staging proposed block.
	err := cs.stageBlock(cs.ProposalBlock)
	if err != nil {
		// Prevote nil.
		return nil
	} else {
		// Prevote block.
		return cs.ProposalBlock.Hash()
	}
}

func (cs *ConsensusState) AddVote(vote *Vote) (added bool, err error) {
	switch vote.Type {
	case VoteTypePrevote:
		// Prevotes checks for height+round match.
		return cs.Prevotes.Add(vote)
	case VoteTypePrecommit:
		// Precommits checks for height+round match.
		return cs.Precommits.Add(vote)
	case VoteTypeCommit:
		// Commits checks for height match.
		cs.Prevotes.Add(vote)
		cs.Precommits.Add(vote)
		return cs.Commits.Add(vote)
	default:
		panic("Unknown vote type")
	}
}

// Lock the ProposalBlock if we have enough prevotes for it,
// or unlock an existing lock if +2/3 of prevotes were nil.
// Returns a blockhash if a block was locked.
func (cs *ConsensusState) RunActionPrecommit(height uint32, round uint16) []byte {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if cs.Height != height || cs.Round != round {
		return nil
	}
	cs.Step = RoundStepPrecommit

	if hash, _, ok := cs.Prevotes.TwoThirdsMajority(); ok {

		// Remember this POL. (hash may be nil)
		cs.LockedPOL = cs.Prevotes.MakePOL()

		if len(hash) == 0 {
			// +2/3 prevoted nil. Just unlock.
			cs.LockedBlock = nil
			return nil
		} else if cs.ProposalBlock.HashesTo(hash) {
			// +2/3 prevoted for proposal block
			// Validate the block.
			// See note on ZombieValidators to see why.
			if err := cs.stageBlock(cs.ProposalBlock); err != nil {
				log.Warning("+2/3 prevoted for an invalid block: %v", err)
				return nil
			}
			cs.LockedBlock = cs.ProposalBlock
			return hash
		} else if cs.LockedBlock.HashesTo(hash) {
			// +2/3 prevoted for already locked block
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

// Commits a block if we have enough precommits (and we have the block).
// If successful, saves the block and state and resets mempool,
// and returns the committed block.
// Commit is not finalized until FinalizeCommit() is called.
// This allows us to stay at this height and gather more commits.
func (cs *ConsensusState) RunActionCommit(height uint32, round uint16) []byte {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if cs.Height != height || cs.Round != round {
		return nil
	}
	cs.Step = RoundStepCommit

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
		cs.blockStore.SaveBlock(block)

		// Save the state
		cs.stagedState.Save(commitTime)

		// Update mempool.
		cs.mempool.ResetForBlockAndState(block, cs.stagedState)

		return block.Hash()
	}

	return nil
}

// After TryCommit(), if successful, must call this in order to
// update the RoundState.
func (cs *ConsensusState) RunActionFinalize(height uint32, round uint16) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if cs.Height != height || cs.Round != round {
		return
	}

	// What was staged becomes committed.
	cs.updateToState(cs.stagedState)
}

func (cs *ConsensusState) stageBlock(block *Block) error {

	// Already staged?
	if cs.stagedBlock == block {
		return nil
	}

	// Create a copy of the state for staging
	stateCopy := cs.state.Copy()

	// Commit block onto the copied state.
	// NOTE: Basic validation is done in state.AppendBlock().
	err := stateCopy.AppendBlock(block, true)
	if err != nil {
		return err
	} else {
		cs.stagedBlock = block
		cs.stagedState = stateCopy
		return nil
	}
}
