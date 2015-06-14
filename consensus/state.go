/*

Consensus State Machine Overview:

* NewHeight, NewRound, Propose, Prevote, Precommit represent state machine steps. (aka RoundStep).
* To "prevote/precommit" something means to broadcast a prevote/precommit vote for something.
* During NewHeight/NewRound/Propose/Prevote/Precommit:
  * Nodes gossip the locked proposal, if locked on a proposal.
  * Nodes gossip the proposal proposed by the designated proposer for that round.
  * Nodes gossip prevotes/precommits for rounds [0...currentRound+1] (currentRound+1 for catch-up)
* Upon each state transition, the height/round/step is broadcast to neighboring peers.
* The set of +2/3 of precommits from the same round for a block "commits the block".

* NewRound:
	* Set up new round. --> Then, goto Propose

* Propose:
  * Upon entering Propose:
    * The designated proposer proposes a block.
  * The Propose step ends:
    * After `timeoutPropose` after entering Propose. --> Then, goto Prevote
    * After receiving proposal block and POL prevotes are ready. --> Then, goto Prevote
    * After any +2/3 prevotes received for the next round. --> Then, goto Prevote next round
    * After any +2/3 precommits received for the next round. --> Then, goto Precommit next round
    * After +2/3 precommits received for a particular block. --> Then, goto Commit

* Prevote:
  * Upon entering Prevote, each validator broadcasts its prevote vote.
    * If the validator is locked on a block, it prevotes that.
    * Else, if the proposed block from the previous step is good, it prevotes that.
    * Else, if the proposal is invalid or wasn't received on time, it prevotes <nil>.
  * The Prevote step ends:
    * After +2/3 prevotes for a particular block or <nil>. --> Then, goto Precommit
    * After `timeoutPrevote` after receiving any +2/3 prevotes. --> Then, goto Precommit
    * After any +2/3 prevotes received for the next round. --> Then, goto Prevote next round
    * After any +2/3 precommits received for the next round. --> Then, goto Precommit next round
    * After +2/3 precommits received for a particular block. --> Then, goto Commit

* Precommit:
  * Upon entering Precommit, each validator broadcasts its precommit vote.
    * If the validator had seen +2/3 of prevotes for a particular block,
      it locks (changes lock to) that block and precommits that block.
    * Else, if the validator had seen +2/3 of prevotes for nil, it unlocks and precommits <nil>.
    * Else, if +2/3 of prevotes for a particular block or nil is not received on time,
      it precommits what it's locked on, or <nil>.
  * The Precommit step ends:
    * After +2/3 precommits for a particular block. --> Then, goto Commit
    * After +2/3 precommits for <nil>. --> Then, goto NewRound next round
    * After `timeoutPrecommit` after receiving any +2/3 precommits. --> Then, goto NewRound next round
    * After any +2/3 prevotes received for the next round. --> Then, goto Prevote next round
    * After any +2/3 precommits received for the next round. --> Then, goto Precommit next round

* Commit:
  * Set CommitTime = now
  * Wait until block is received, then goto NewHeight

* NewHeight:
  * Upon entering NewHeight,
    * Move Precommits to LastPrecommits and increment height.
    * Wait until `CommitTime+timeoutCommit` to receive straggler commits. --> Then, goto NewRound round 0

* Proof of Safety:
  If a good validator commits at round R, it's because it saw +2/3 of precommits for round R.
  This implies that (assuming tolerance bounds) +1/3 of honest nodes are still locked at round R+1.
  These locked validators will remain locked until they see +2/3 prevote for something
  else, but this won't happen because +1/3 are locked and honest.

* Proof of Liveness:
  Lemma 1: If +1/3 good nodes are locked on two different blocks, the proposers' POLRound will
    eventually cause nodes locked from the earlier round to unlock.
    -> `timeoutProposalR` increments with round R, while the block.size && POL prevote size
       are fixed, so eventually we'll be able to "fully gossip" the block & POL.
    TODO: cap the block.size at something reasonable.
  Lemma 2: If a good node is at round R, neighboring good nodes will soon catch up to round R.

                            +-------------------------------------+
                            v                                     |(Wait til `CommmitTime+timeoutCommit`)
                      +-----------+                         +-----+-----+
         +----------> |  Propose  +--------------+          | NewHeight |
         |            +-----------+              |          +-----------+
         |                                       |                ^
         |(Else, after timeoutPrecommit)         v                |
   +-----+-----+                           +-----------+          |
   | Precommit |  <------------------------+  Prevote  |          |
   +-----+-----+                           +-----------+          |
         |(When +2/3 Precommits for block found)                  |
         v                                                        |
   +--------------------------------------------------------------------+
   |  Commit                                                            |
   |                                                                    |
   |  * Set CommitTime = now;                                           |
   |  * Wait for block, then stage/save/commit block;                   |
   +--------------------------------------------------------------------+

*/

package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	bc "github.com/tendermint/tendermint/blockchain"
	. "github.com/tendermint/tendermint/common"
	. "github.com/tendermint/tendermint/consensus/types"
	"github.com/tendermint/tendermint/events"
	mempl "github.com/tendermint/tendermint/mempool"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

var (
	timeoutPropose        = 3000 * time.Millisecond // Maximum duration of RoundStepPropose
	timeoutPrevote0       = 1000 * time.Millisecond // After any +2/3 prevotes received, wait this long for stragglers.
	timeoutPrevoteDelta   = 0500 * time.Millisecond // timeoutPrevoteN is timeoutPrevote0 + timeoutPrevoteDelta*N
	timeoutPrecommit0     = 1000 * time.Millisecond // After any +2/3 precommits received, wait this long for stragglers.
	timeoutPrecommitDelta = 0500 * time.Millisecond // timeoutPrecommitN is timeoutPrecommit0 + timeoutPrecommitDelta*N
	timeoutCommit         = 2000 * time.Millisecond // After +2/3 commits received for committed block, wait this long for stragglers in the next height's RoundStepNewHeight.
)

var (
	ErrInvalidProposalSignature = errors.New("Error invalid proposal signature")
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
		panic(Fmt("Unknown RoundStep %X", rs))
	}
}

//-----------------------------------------------------------------------------

// Immutable when returned from ConsensusState.GetRoundState()
type RoundState struct {
	Height             uint // Height we are working on
	Round              uint
	Step               RoundStepType
	StartTime          time.Time
	CommitTime         time.Time // Subjective time when +2/3 precommits for Block at Round were found
	Validators         *sm.ValidatorSet
	Proposal           *Proposal
	ProposalBlock      *types.Block
	ProposalBlockParts *types.PartSet
	LockedBlock        *types.Block
	LockedBlockParts   *types.PartSet
	Votes              *HeightVoteSet
	LastPrecommits     *VoteSet // Last precommits for Height-1
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
%s  LockedBlock:   %v %v
%s  Votes:         %v
%s  LastPrecommits: %v
%s}`,
		indent, rs.Height, rs.Round, rs.Step,
		indent, rs.StartTime,
		indent, rs.CommitTime,
		indent, rs.Validators.StringIndented(indent+"    "),
		indent, rs.Proposal,
		indent, rs.ProposalBlockParts.StringShort(), rs.ProposalBlock.StringShort(),
		indent, rs.LockedBlockParts.StringShort(), rs.LockedBlock.StringShort(),
		indent, rs.Votes.StringIndented(indent+"    "),
		indent, rs.LastPrecommits.StringShort(),
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

	blockStore     *bc.BlockStore
	mempoolReactor *mempl.MempoolReactor
	privValidator  *sm.PrivValidator
	newStepCh      chan *RoundState

	mtx sync.Mutex
	RoundState
	state       *sm.State    // State until height-1.
	stagedBlock *types.Block // Cache last staged block.
	stagedState *sm.State    // Cache result of staged block.

	evsw events.Fireable
	evc  *events.EventCache // set in stageBlock and passed into state
}

func NewConsensusState(state *sm.State, blockStore *bc.BlockStore, mempoolReactor *mempl.MempoolReactor) *ConsensusState {
	cs := &ConsensusState{
		quit:           make(chan struct{}),
		blockStore:     blockStore,
		mempoolReactor: mempoolReactor,
		newStepCh:      make(chan *RoundState, 10),
	}
	cs.updateToState(state, true)
	cs.maybeRebond()
	cs.reconstructLastPrecommits(state)
	return cs
}

// Reconstruct LastPrecommits from SeenValidation, which we saved along with the block,
// (which happens even before saving the state)
func (cs *ConsensusState) reconstructLastPrecommits(state *sm.State) {
	if state.LastBlockHeight == 0 {
		return
	}
	lastPrecommits := NewVoteSet(state.LastBlockHeight, 0, types.VoteTypePrecommit, state.LastBondedValidators)
	seenValidation := cs.blockStore.LoadSeenValidation(state.LastBlockHeight)
	for idx, precommit := range seenValidation.Precommits {
		precommitVote := &types.Vote{
			Height:     state.LastBlockHeight,
			Round:      seenValidation.Round,
			Type:       types.VoteTypePrecommit,
			BlockHash:  state.LastBlockHash,
			BlockParts: state.LastBlockParts,
			Signature:  precommit.Signature,
		}
		added, _, err := lastPrecommits.AddByIndex(uint(idx), precommitVote)
		if !added || err != nil {
			panic(Fmt("Failed to reconstruct LastPrecommits: %v", err))
		}
	}
	if !lastPrecommits.HasTwoThirdsMajority() {
		panic("Failed to reconstruct LastPrecommits: Does not have +2/3 maj")
	}
	cs.LastPrecommits = lastPrecommits
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
		cs.scheduleRound0()
	}
}

func (cs *ConsensusState) scheduleRound0(height uint) {
	sleepDuration := cs.StartTime.Sub(time.Now())
	go func() {
		if sleepDuration > 0 {
			time.Sleep(sleepDuration)
		}
		cs.EnterNewRound(height, 0)
	}()
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

// Updates ConsensusState and increments height to match that of state.
// The round becomes 0 and cs.Step becomes RoundStepNewHeight.
func (cs *ConsensusState) updateToState(state *sm.State, contiguous bool) {
	// SANITY CHECK
	if contiguous && cs.Height > 0 && cs.Height != state.LastBlockHeight {
		panic(Fmt("updateToState() expected state height of %v but found %v",
			cs.Height, state.LastBlockHeight))
	}
	// END SANITY CHECK

	// Reset fields based on state.
	validators := state.BondedValidators
	height := state.LastBlockHeight + 1 // next desired block height
	lastPrecommits := (*VoteSet)(nil)
	if contiguous && cs.Votes != nil {
		lastPrecommits = cs.Votes.Precommits(cs.Round)
	}

	// RoundState fields
	cs.Height = height
	cs.Round = 0
	cs.Step = RoundStepNewHeight
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
	cs.LockedBlock = nil
	cs.LockedBlockParts = nil
	cs.Votes = NewHeightVoteSet(height, validators)
	cs.LastPrecommits = lastPrecommits

	cs.state = state
	cs.stagedBlock = nil
	cs.stagedState = nil
}

// If we're unbonded, broadcast RebondTx.
func (cs *ConsensusState) maybeRebond() {
	if cs.privValidator == nil || !cs.state.UnbondingValidators.HasAddress(cs.privValidator.Address) {
		return
	}
	rebondTx := &types.RebondTx{
		Address: cs.privValidator.Address,
		Height:  cs.Height,
	}
	err := cs.privValidator.SignRebondTx(cs.state.ChainID, rebondTx)
	if err == nil {
		err := cs.mempoolReactor.BroadcastTx(rebondTx)
		if err != nil {
			log.Error("Failed to broadcast RebondTx",
				"height", cs.Height, "round", cs.Round, "tx", rebondTx, "error", err)
		} else {
			log.Info("Signed and broadcast RebondTx",
				"height", cs.Height, "round", cs.Round, "tx", rebondTx)
		}
	} else {
		log.Warn("Error signing RebondTx", "height", cs.Height, "round", cs.Round, "tx", rebondTx, "error", err)
	}
}

func (cs *ConsensusState) SetPrivValidator(priv *sm.PrivValidator) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	cs.privValidator = priv
}

//-----------------------------------------------------------------------------

// Enter: +2/3 precommits for nil from previous round
// Enter: `timeoutPrecommits` after any +2/3 precommits
// Enter: `commitTime+timeoutCommit` from NewHeight
// NOTE: cs.StartTime was already set for height.
func (cs *ConsensusState) EnterNewRound(height uint, round uint) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if cs.Height != height || cs.Round >= round {
		log.Debug(Fmt("EnterNewRound(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}
	if now := time.Now(); cs.StartTime.After(now) {
		log.Warn("Need to set a buffer and log.Warn() here for sanity.", "startTime", cs.StartTime, "now", now)
	}

	// Increment validators if necessary
	if cs.Round < round {
		validators := cs.Validators.Copy()
		validators.IncrementAccum(round - cs.Round)
	}

	// Setup new round
	cs.Round = round
	cs.Step = RoundStepNewRound
	cs.Validators = validators
	cs.Proposal = nil
	cs.ProposalBlock = nil
	cs.ProposalBlockParts = nil
	cs.Votes.SetRound(round + 1) // track next round.

	// Immediately go to EnterPropose.
	go cs.EnterPropose(height, round)
}

// Enter: from NewRound.
func (cs *ConsensusState) EnterPropose(height uint, round uint) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if cs.Height != height || cs.Round > round || (cs.Round == round && cs.Step >= RoundStepPropose) {
		log.Debug(Fmt("EnterPropose(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}

	defer func() {
		// Done EnterPropose:
		cs.Round = round
		cs.Step = RoundStepPropose
		cs.newStepCh <- cs.getRoundState()

		// If we already have the proposal + POL, then goto Prevote
		if cs.isProposalComplete() {
			go cs.EnterPrevote(height, round)
		}
	}()

	// This step times out after `timeoutPropose`
	go func() {
		time.Sleep(timeoutPropose)
		cs.EnterPrevote(height, round)
	}()

	// Nothing to do if it's not our turn.
	if cs.privValidator == nil {
		return
	}

	// See if it is our turn to propose
	if !bytes.Equal(cs.Validators.Proposer().Address, cs.privValidator.Address) {
		log.Debug("EnterPropose: Not our turn to propose", "proposer", cs.Validators.Proposer().Address, "privValidator", cs.privValidator)
		return
	} else {
		log.Debug("EnterPropose: Our turn to propose", "proposer", cs.Validators.Proposer().Address, "privValidator", cs.privValidator)
	}

	// We are going to propose a block.

	var block *types.Block
	var blockParts *types.PartSet

	// Decide on block
	if cs.LockedBlock != nil {
		// If we're locked onto a block, just choose that.
		block, blockParts = cs.LockedBlock, cs.LockedBlockParts
	} else {
		// Create a new proposal block from state/txs from the mempool.
		block, blockParts = cs.createProposalBlock()
	}

	// Make proposal
	proposal := NewProposal(cs.Height, cs.Round, blockParts.Header(), cs.Votes.POLRound())
	err := cs.privValidator.SignProposal(cs.state.ChainID, proposal)
	if err == nil {
		log.Info("Signed and set proposal", "height", cs.Height, "round", cs.Round, "proposal", proposal)
		log.Debug(Fmt("Signed and set proposal block: %v", block))
		// Set fields
		cs.Proposal = proposal
		cs.ProposalBlock = block
		cs.ProposalBlockParts = blockParts
	} else {
		log.Warn("EnterPropose: Error signing proposal", "height", cs.Height, "round", cs.Round, "error", err)
	}
}

func (cs *ConsensusState) isProposalComplete() bool {
	if cs.Proposal == nil || cs.ProposalBlock == nil {
		return false
	}
	return cs.Votes.Prevote(cs.Proposal.POLRound).HasTwoThirdsMajority()
}

// Create the next block to propose and return it.
// NOTE: make it side-effect free for clarity.
func (cs *ConsensusState) createProposalBlock() (*types.Block, *types.PartSet) {
	var validation *types.Validation
	if cs.Height == 1 {
		// We're creating a proposal for the first block.
		// The validation is empty.
		validation = &types.Validation{}
	} else if cs.LastPrecommits.HasTwoThirdsMajority() {
		// Make the validation from LastPrecommits
		validation = cs.LastPrecommits.MakeValidation()
	} else {
		// This shouldn't happen.
		log.Error("EnterPropose: Cannot propose anything: No validation for the previous block.")
		return
	}
	txs := cs.mempoolReactor.Mempool.GetProposalTxs()
	block = &types.Block{
		Header: &types.Header{
			ChainID:        cs.state.ChainID,
			Height:         cs.Height,
			Time:           time.Now(),
			Fees:           0, // TODO fees
			NumTxs:         uint(len(txs)),
			LastBlockHash:  cs.state.LastBlockHash,
			LastBlockParts: cs.state.LastBlockParts,
			StateHash:      nil, // Will set afterwards.
		},
		Validation: validation,
		Data: &types.Data{
			Txs: txs,
		},
	}

	// Set the block.Header.StateHash.
	err := cs.state.ComputeBlockStateHash(block)
	if err != nil {
		log.Error("EnterPropose: Error setting state hash", "error", err)
		return
	}

	blockParts = block.MakePartSet()
	return block, blockParts
}

// Enter: `timeoutPropose` after start of Propose.
// Enter: proposal block and POL is ready.
// Enter: any +2/3 prevotes for next round.
// Prevote for LockedBlock if we're locked, or ProposealBlock if valid.
// Otherwise vote nil.
func (cs *ConsensusState) EnterPrevote(height uint, round uint) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if cs.Height != height || cs.Round > round || (cs.Round == round && cs.Step >= RoundStepPrevote) {
		log.Debug(Fmt("EnterPrevote(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}

	defer func() {
		// Done EnterPrevote:
		cs.Round = round
		cs.Step = RoundStepPrevote
		cs.newStepCh <- cs.getRoundState()
		// Maybe immediately go to EnterPrevoteWait.
		if cs.Votes.Prevotes(round).HasTwoThirdsAny() {
			go cs.EnterPrevoteWait(height, round)
		}
	}()

	// Sign and broadcast vote as necessary
	cs.doPrevote(height, round)
}

func (cs *ConsensusState) doPrevote(height uint, round uint) {
	// If a block is locked, prevote that.
	if cs.LockedBlock != nil {
		log.Debug("EnterPrevote: Block was locked")
		cs.signAddVote(types.VoteTypePrevote, cs.LockedBlock.Hash(), cs.LockedBlockParts.Header())
		return
	}

	// If ProposalBlock is nil, prevote nil.
	if cs.ProposalBlock == nil {
		log.Warn("EnterPrevote: ProposalBlock is nil")
		cs.signAddVote(types.VoteTypePrevote, nil, types.PartSetHeader{})
		return
	}

	// Try staging cs.ProposalBlock
	err := cs.stageBlock(cs.ProposalBlock, cs.ProposalBlockParts)
	if err != nil {
		// ProposalBlock is invalid, prevote nil.
		log.Warn("EnterPrevote: ProposalBlock is invalid", "error", err)
		cs.signAddVote(types.VoteTypePrevote, nil, types.PartSetHeader{})
		return
	}

	// Prevote cs.ProposalBlock
	cs.signAddVote(types.VoteTypePrevote, cs.ProposalBlock.Hash(), cs.ProposalBlockParts.Header())
	return
}

// Enter: any +2/3 prevotes for next round.
func (cs *ConsensusState) EnterPrevoteWait(height uint, round uint) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if cs.Height != height || cs.Round > round || (cs.Round == round && cs.Step >= RoundStepPrevoteWait) {
		log.Debug(Fmt("EnterPrevoteWait(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}
	if !cs.Votes.Prevotes(round).HasTwoThirdsAny() {
		panic(Fmt("EnterPrevoteWait(%v/%v), but Prevotes does not have any +2/3 votes", height, round))
	}

	// Done EnterPrevoteWait:
	cs.Round = round
	cs.Step = RoundStepPrevoteWait
	cs.newStepCh <- cs.getRoundState()

	// After `timeoutPrevote`, EnterPrecommit()
	go func() {
		time.Sleep(timeoutPrevote)
		cs.EnterPrecommit(height, round)
	}()
}

// Enter: +2/3 precomits for block or nil.
// Enter: `timeoutPrevote` after any +2/3 prevotes.
// Enter: any +2/3 precommits for next round.
// Lock & precommit the ProposalBlock if we have enough prevotes for it,
// else, unlock an existing lock and precommit nil if +2/3 of prevotes were nil,
// else, precommit locked block or nil otherwise.
func (cs *ConsensusState) EnterPrecommit(height uint, round uint) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if cs.Height != height || cs.Round > round || (cs.Round == round && cs.Step >= RoundStepPrecommit) {
		log.Debug(Fmt("EnterPrecommit(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}

	defer func() {
		// Done EnterPrecommit:
		cs.Round = round
		cs.Step = RoundStepPrecommit
		cs.newStepCh <- cs.getRoundState()
		// Maybe immediately go to EnterPrecommitWait.
		if cs.Votes.Precommits(round).HasTwoThirdsAny() {
			go cs.EnterPrecommitWait(height, round)
		}
	}()

	hash, partsHeader, ok := cs.Votes.Prevotes(round).TwoThirdsMajority()

	// If we don't have two thirds of prevotes, just precommit locked block or nil
	if !ok {
		if cs.LockedBlock != nil {
			log.Info("EnterPrecommit: No +2/3 prevotes during EnterPrecommit. Precommitting lock.")
			cs.signAddVote(types.VoteTypePrecommit, cs.LockedBlock.Hash(), cs.LockedBlockParts.Header())
		} else {
			log.Info("EnterPrecommit: No +2/3 prevotes during EnterPrecommit. Precommitting nil.")
			cs.signAddVote(types.VoteTypePrecommit, nil, types.PartSetHeader{})
		}
		return
	}

	// +2/3 prevoted nil. Unlock and precommit nil.
	if len(hash) == 0 {
		if cs.LockedBlock == nil {
			log.Info("EnterPrecommit: +2/3 prevoted for nil.")
		} else {
			log.Info("EnterPrecommit: +2/3 prevoted for nil. Unlocking")
			cs.LockedBlock = nil
			cs.LockedBlockParts = nil
		}
		cs.signAddVote(types.VoteTypePrecommit, nil, types.PartSetHeader{})
		return
	}

	// At this point, +2/3 prevoted for a particular block.

	// If +2/3 prevoted for already locked block, precommit it.
	if cs.LockedBlock.HashesTo(hash) {
		log.Info("EnterPrecommit: +2/3 prevoted locked block.")
		cs.signAddVote(types.VoteTypePrecommit, hash, partsHeader)
		return
	}

	// If +2/3 prevoted for proposal block, stage and precommit it
	if cs.ProposalBlock.HashesTo(hash) {
		log.Info("EnterPrecommit: +2/3 prevoted proposal block.")
		// Validate the block.
		if err := cs.stageBlock(cs.ProposalBlock, cs.ProposalBlockParts); err != nil {
			panic(Fmt("EnterPrecommit: +2/3 prevoted for an invalid block: %v", err))
		}
		cs.LockedBlock = cs.ProposalBlock
		cs.LockedBlockParts = cs.ProposalBlockParts
		cs.signAddVote(types.VoteTypePrecommit, hash, partsHeader)
		return
	}

	// Otherwise, we need to fetch the +2/3 prevoted block.
	// We don't have the block yet so we can't lock/precommit it.
	cs.LockedBlock = nil
	cs.LockedBlockParts = nil
	if !cs.ProposalBlockParts.HasHeader(partsHeader) {
		cs.ProposalBlock = nil
		cs.ProposalBlockParts = types.NewPartSetFromHeader(partsHeader)
	}
	cs.signAddVote(types.VoteTypePrecommit, nil, PartSetHeader{})
	return
}

// Enter: any +2/3 precommits for next round.
func (cs *ConsensusState) EnterPrecommitWait(height uint, round uint) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if cs.Height != height || cs.Round > round || (cs.Round == round && cs.Step >= RoundStepPrecommitWait) {
		log.Debug(Fmt("EnterPrecommitWait(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}
	if !cs.Votes.Precommits(round).HasTwoThirdsAny() {
		panic(Fmt("EnterPrecommitWait(%v/%v), but Precommits does not have any +2/3 votes", height, round))
	}

	// Done EnterPrecommitWait:
	cs.Round = round
	cs.Step = RoundStepPrecommitWait
	cs.newStepCh <- cs.getRoundState()

	// After `timeoutPrecommit`, EnterNewRound()
	go func() {
		time.Sleep(timeoutPrecommit)
		// If we have +2/3 of precommits for a particular block (or nil),
		// we already entered commit (or the next round).
		// So just try to transition to the next round,
		// which is what we'd do otherwise.
		cs.EnterNewRound(height, round+1)
	}()
}

// Enter: +2/3 precommits for block
func (cs *ConsensusState) EnterCommit(height uint) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if cs.Height != height || cs.Step >= RoundStepCommit {
		log.Debug(Fmt("EnterCommit(%v): Invalid args. Current step: %v/%v/%v", height, cs.Height, cs.Round, cs.Step))
		return
	}

	defer func() {
		// Done Entercommit:
		// keep ca.Round the same, it points to the right Precommits set.
		cs.Step = RoundStepCommit
		cs.newStepCh <- cs.getRoundState()

		// Maybe finalize immediately.
		cs.TryFinalizeCommit(height)
	}()

	// SANITY CHECK
	hash, partsHeader, ok := cs.Precommits.TwoThirdsMajority()
	if !ok {
		panic("RunActionCommit() expects +2/3 precommits")
	}
	// END SANITY CHECK

	// The Locked* fields no longer matter.
	// Move them over to ProposalBlock if they match the commit hash,
	// otherwise they can now be cleared.
	if cs.LockedBlock.HashesTo(hash) {
		cs.ProposalBlock = cs.LockedBlock
		cs.ProposalBlockParts = cs.LockedBlockParts
		cs.LockedBlock = nil
		cs.LockedBlockParts = nil
	} else {
		cs.LockedBlock = nil
		cs.LockedBlockParts = nil
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
	} else {
		// We have the block, so sign a Commit-vote.
		cs.commitVoteBlock(cs.ProposalBlock, cs.ProposalBlockParts)
	}
}

// If we have the block AND +2/3 commits for it, finalize.
func (cs *ConsensusState) TryFinalizeCommit(height uint) {
	if cs.ProposalBlock.HashesTo(hash) && cs.Commits.HasTwoThirdsMajority() {
		go cs.FinalizeCommit(height)
	}
}

// Increment height and goto RoundStepNewHeight
func (cs *ConsensusState) FinalizeCommit(height uint) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	if cs.Height != height || cs.Step != RoundStepCommit {
		log.Debug(Fmt("FinalizeCommit(%v): Invalid args. Current step: %v/%v/%v", height, cs.Height, cs.Round, cs.Step))
		return
	}

	hash, header, ok := cs.Commits.TwoThirdsMajority()

	// SANITY CHECK
	if !ok {
		panic(Fmt("Cannot FinalizeCommit, commit does not have two thirds majority"))
	}
	if !cs.ProposalBlockParts.HasHeader(header) {
		panic(Fmt("Expected ProposalBlockParts header to be commit header"))
	}
	if !cs.ProposalBlock.HashesTo(hash) {
		panic(Fmt("Cannot FinalizeCommit, ProposalBlock does not hash to commit hash"))
	}
	if err := cs.stageBlock(cs.ProposalBlock, cs.ProposalBlockParts); err != nil {
		panic(Fmt("+2/3 committed an invalid block: %v", err))
	}
	// END SANITY CHECK

	log.Debug(Fmt("Finalizing commit of block: %v", cs.ProposalBlock))
	// We have the block, so stage/save/commit-vote.
	cs.saveBlock(cs.ProposalBlock, cs.ProposalBlockParts, cs.Commits)
	cs.commitVoteBlock(cs.ProposalBlock, cs.ProposalBlockParts, cs.Commits)
	// Fire off event
	go func() {
		cs.evsw.FireEvent(types.EventStringNewBlock(), cs.ProposalBlock)
		cs.evc.Flush()
	}(cs.ProposalBlock)

	// Increment height.
	cs.updateToState(cs.stagedState, true)

	// If we're unbonded, broadcast RebondTx.
	cs.maybeRebond()

	// By here,
	// * cs.Height has been increment to height+1
	// * cs.Step is now RoundStepNewHeight
	// * cs.StartTime is set to when we should start round0.
	cs.newStepCh <- cs.getRoundState()
	// Start round 0 when cs.StartTime.
	go cs.scheduleRound0(height)
	return
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
	if cs.Step >= RoundStepCommit {
		return nil
	}

	// Verify signature
	if !cs.Validators.Proposer().PubKey.VerifyBytes(account.SignBytes(cs.state.ChainID, proposal), proposal.Signature) {
		return ErrInvalidProposalSignature
	}

	cs.Proposal = proposal
	cs.ProposalBlockParts = types.NewPartSetFromHeader(proposal.BlockParts)
	return nil
}

// NOTE: block is not necessarily valid.
func (cs *ConsensusState) AddProposalBlockPart(height uint, round uint, part *types.Part) (added bool, err error) {
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
		cs.ProposalBlock = binary.ReadBinary(&types.Block{}, cs.ProposalBlockParts.GetReader(), &n, &err).(*types.Block)
		log.Debug("Received complete proposal", "hash", cs.ProposalBlock.Hash())
		if cs.Step == RoundStepPropose && cs.isProposalComplete() {
			go cs.EnterPrevote(height, round)
		} else if cs.Step == RoundStepCommit {
			cs.TryFinalizeCommit(height)
		}
		return true, err
	}
	return true, nil
}

func (cs *ConsensusState) AddVote(address []byte, vote *types.Vote) (added bool, index uint, err error) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	return cs.addVote(address, vote)
}

//-----------------------------------------------------------------------------

func (cs *ConsensusState) addVote(address []byte, vote *types.Vote) (added bool, index uint, err error) {
	// A precommit for the previous height?
	if vote.Height+1 == cs.Height && vote.Type == types.VoteTypePrecommit {
		added, index, err = cs.LastPrecommits.AddByAddress(address, vote)
		if added {
			log.Debug(Fmt("Added to lastPrecommits: %v", cs.LastPrecommits.StringShort()))
		}
		return
	}

	// A prevote/precommit for this height?
	if vote.Height == cs.Height {
		added, index, err = cs.Votes.AddByAddress(address, vote)
		if added {
			switch vote.Type {
			case types.VoteTypePrevote:
				log.Debug(Fmt("Added to prevotes: %v", cs.Votes.Prevotes(vote.Round).StringShort()))
				if cs.Round < vote.Round && cs.Votes.Prevotes(vote.Round).HasTwoThirdsAny() {
					// Goto to Prevote vote.Round.
					go func() {
						cs.EnterNewRound(height, vote.Round)
						cs.EnterPrevote(height, vote.Round)
						cs.EnterPrevoteWait(height, vote.Round)
					}()
				} else if cs.Round == vote.Round {
					if cs.Votes.Prevotes(cs.Round).HasTwoThirdsMajority() {
						// Goto Precommit, whether for block or nil.
						go cs.EnterPrecommit(height, cs.Round)
					} else if cs.Votes.Prevotes(cs.Round).HasTwoThirdsAny() {
						// Goto PrevoteWait
						go func() {
							cs.EnterPrevote(height, cs.Round)
							cs.EnterPrevoteWait(height, cs.Round)
						}()
					}
				} else if cs.Proposal != nil && cs.Proposal.POLRound == vote.Round {
					if cs.isProposalComplete() {
						go cs.EnterPrevote(height, cs.Round)
					}
				}
			case types.VoteTypePrecommit:
				log.Debug(Fmt("Added to precommit: %v", cs.Votes.Precommits(vote.Round).StringShort()))
				if cs.Round < vote.Round {
					if hash, _, ok := cs.Votes.Precommits(cs.Round).TwoThirdsMajority(); ok {
						if len(hash) == 0 {
							// This is weird, shouldn't happen
							log.Warn("This is weird, why did we receive +2/3 of nil precommits?")
							// Skip to Precommit of vote.Round
							go func() {
								cs.EnterNewRound(height, vote.Round)
								cs.EnterPrecommit(height, vote.Round)
								cs.EnterPrecommitWait(height, vote.Round)
							}()
						} else {
							// If hash is block, goto Commit
							go func() {
								cs.EnterNewRound(height, vote.Round)
								cs.EnterCommit(height, vote.Round)
							}()
						}
					} else if cs.Votes.Precommits(vote.Round).HasTwoThirdsAny() {
						// Skip to Precommit of vote.Round
						go func() {
							cs.EnterNewRound(height, vote.Round)
							cs.EnterPrecommit(height, vote.Round)
							cs.EnterPrecommitWait(height, vote.Round)
						}()
					}
				} else if cs.Round == vote.Round {
					if hash, _, ok := cs.Votes.Precommits(cs.Round).TwoThirdsMajority(); ok {
						if len(hash) == 0 {
							// If hash is nil, goto NewRound
							go cs.EnterNewRound(height, cs.Round+1)
						} else {
							// If hash is block, goto Commit
							go cs.EnterCommit(height, cs.Round)
						}
					} else if cs.Votes.Precommits(cs.Round).HasTwoThirdsAny() {
						// Goto PrecommitWait
						go func() {
							cs.EnterPrecommit(height, cs.Round)
							cs.EnterPrecommitWait(height, cs.Round)
						}()
					}
				}
			default:
				panic(Fmt("Unexpected vote type %X", vote.Type)) // Should not happen.
			}
		}
		return
	}

	// Height mismatch, bad peer? TODO
	return
}

func (cs *ConsensusState) stageBlock(block *types.Block, blockParts *types.PartSet) error {
	if block == nil {
		panic("Cannot stage nil block")
	}

	// Already staged?
	blockHash := block.Hash()
	if cs.stagedBlock != nil && len(blockHash) != 0 && bytes.Equal(cs.stagedBlock.Hash(), blockHash) {
		return nil
	}

	// Create a copy of the state for staging
	stateCopy := cs.state.Copy()
	// reset the event cache and pass it into the state
	cs.evc = events.NewEventCache(cs.evsw)
	stateCopy.SetFireable(cs.evc)

	// Commit block onto the copied state.
	// NOTE: Basic validation is done in state.AppendBlock().
	err := sm.ExecBlock(stateCopy, block, blockParts.Header())
	if err != nil {
		return err
	} else {
		cs.stagedBlock = block
		cs.stagedState = stateCopy
		return nil
	}
}

func (cs *ConsensusState) signAddVote(type_ byte, hash []byte, header types.PartSetHeader) *types.Vote {
	if cs.privValidator == nil || !cs.Validators.HasAddress(cs.privValidator.Address) {
		return nil
	}
	vote := &types.Vote{
		Height:     cs.Height,
		Round:      cs.Round,
		Type:       type_,
		BlockHash:  hash,
		BlockParts: header,
	}
	err := cs.privValidator.SignVote(cs.state.ChainID, vote)
	if err == nil {
		log.Info("Signed and added vote", "height", cs.Height, "round", cs.Round, "vote", vote)
		cs.addVote(cs.privValidator.Address, vote)
		return vote
	} else {
		log.Warn("Error signing vote", "height", cs.Height, "round", cs.Round, "vote", vote, "error", err)
		return nil
	}
}

// Save Block, save the +2/3 Commits we've seen
func (cs *ConsensusState) saveBlock(block *types.Block, blockParts *types.PartSet, commits *VoteSet) {

	// The proposal must be valid.
	if err := cs.stageBlock(block, blockParts); err != nil {
		panic(Fmt("saveBlock() an invalid block: %v", err))
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

}

// implements events.Eventable
func (cs *ConsensusState) SetFireable(evsw events.Fireable) {
	cs.evsw = evsw
}
