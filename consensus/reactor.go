package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/block"
	. "github.com/tendermint/tendermint/common"
	. "github.com/tendermint/tendermint/consensus/types"
	"github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/state"
)

const (
	StateCh = byte(0x20)
	DataCh  = byte(0x21)
	VoteCh  = byte(0x22)

	peerStateKey = "ConsensusReactor.peerState"

	peerGossipSleepDuration = 50 * time.Millisecond // Time to sleep if there's nothing to send.
)

//-----------------------------------------------------------------------------

type ConsensusReactor struct {
	sw      *p2p.Switch
	started uint32
	stopped uint32
	quit    chan struct{}

	conS *ConsensusState
}

func NewConsensusReactor(blockStore *BlockStore, mempoolReactor *mempool.MempoolReactor, state *state.State) *ConsensusReactor {
	conS := NewConsensusState(state, blockStore, mempoolReactor)
	conR := &ConsensusReactor{
		quit: make(chan struct{}),
		conS: conS,
	}
	return conR
}

// Implements Reactor
func (conR *ConsensusReactor) Start(sw *p2p.Switch) {
	if atomic.CompareAndSwapUint32(&conR.started, 0, 1) {
		log.Info("Starting ConsensusReactor")
		conR.sw = sw
		conR.conS.Start()
		go conR.broadcastNewRoundStepRoutine()
	}
}

// Implements Reactor
func (conR *ConsensusReactor) Stop() {
	if atomic.CompareAndSwapUint32(&conR.stopped, 0, 1) {
		log.Info("Stopping ConsensusReactor")
		conR.conS.Stop()
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
	var err error = nil

	log.Debug("[%X][%v] Receive: %v", chId, peer, msg_)

	switch chId {
	case StateCh:
		switch msg_.(type) {
		case *NewRoundStepMessage:
			msg := msg_.(*NewRoundStepMessage)
			ps.ApplyNewRoundStepMessage(msg, rs)

		case *CommitStepMessage:
			msg := msg_.(*CommitStepMessage)
			ps.ApplyCommitStepMessage(msg)

		case *HasVoteMessage:
			msg := msg_.(*HasVoteMessage)
			ps.ApplyHasVoteMessage(msg)

		default:
			// Ignore unknown message
		}

	case DataCh:
		switch msg_.(type) {
		case *Proposal:
			proposal := msg_.(*Proposal)
			ps.SetHasProposal(proposal)
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
		case *VoteMessage:
			voteMessage := msg_.(*VoteMessage)
			vote := voteMessage.Vote
			if rs.Height != vote.Height {
				return // Wrong height. Not necessarily a bad peer.
			}
			validatorIndex := voteMessage.ValidatorIndex
			address, _ := rs.Validators.GetByIndex(validatorIndex)
			added, index, err := conR.conS.AddVote(address, vote)
			if err != nil {
				// Probably an invalid signature. Bad peer.
				log.Warning("Error attempting to add vote: %v", err)
			}
			// Initialize Prevotes/Precommits/Commits if needed
			ps.EnsureVoteBitArrays(rs.Height, rs.Validators.Size())
			ps.SetHasVote(vote, index)
			if added {
				msg := &HasVoteMessage{
					Height: vote.Height,
					Round:  vote.Round,
					Type:   vote.Type,
					Index:  index,
				}
				conR.sw.Broadcast(StateCh, msg)
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

// Sets our private validator account for signing votes.
func (conR *ConsensusReactor) SetPrivValidator(priv *state.PrivValidator) {
	conR.conS.SetPrivValidator(priv)
}

//--------------------------------------

// Listens for changes to the ConsensusState.Step by pulling
// on conR.conS.NewStepCh().
func (conR *ConsensusReactor) broadcastNewRoundStepRoutine() {
	for {
		// Get RoundState with new Step or quit.
		var rs *RoundState
		select {
		case rs = <-conR.conS.NewStepCh():
		case <-conR.quit:
			return
		}

		// Get seconds since beginning of height.
		// Due to the condition documented, this is safe.
		timeElapsed := rs.StartTime.Sub(time.Now())

		// Broadcast NewRoundStepMessage
		{
			msg := &NewRoundStepMessage{
				Height: rs.Height,
				Round:  rs.Round,
				Step:   rs.Step,
				SecondsSinceStartTime: uint(timeElapsed.Seconds()),
			}
			conR.sw.Broadcast(StateCh, msg)
		}

		// If the step is commit, then also broadcast a CommitStepMessage.
		if rs.Step == RoundStepCommit {
			msg := &CommitStepMessage{
				Height:        rs.Height,
				BlockParts:    rs.ProposalBlockParts.Header(),
				BlockBitArray: rs.ProposalBlockParts.BitArray(),
			}
			conR.sw.Broadcast(StateCh, msg)
		}
	}
}

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

		// Send proposal Block parts?
		// NOTE: if we or peer is at RoundStepCommit*, the round
		// won't necessarily match, but that's OK.
		if rs.ProposalBlockParts.HasHeader(prs.ProposalBlockParts) {
			if index, ok := rs.ProposalBlockParts.BitArray().Sub(
				prs.ProposalBlockBitArray).PickRandom(); ok {
				msg := &PartMessage{
					Height: rs.Height,
					Round:  rs.Round,
					Type:   partTypeProposalBlock,
					Part:   rs.ProposalBlockParts.GetPart(index),
				}
				peer.Send(DataCh, msg)
				ps.SetHasProposalBlockPart(rs.Height, rs.Round, index)
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
			ps.SetHasProposal(rs.Proposal)
			continue OUTER_LOOP
		}

		// Send proposal POL parts?
		if rs.ProposalPOLParts.HasHeader(prs.ProposalPOLParts) {
			if index, ok := rs.ProposalPOLParts.BitArray().Sub(
				prs.ProposalPOLBitArray).PickRandom(); ok {
				msg := &PartMessage{
					Height: rs.Height,
					Round:  rs.Round,
					Type:   partTypeProposalPOL,
					Part:   rs.ProposalPOLParts.GetPart(index),
				}
				peer.Send(DataCh, msg)
				ps.SetHasProposalPOLPart(rs.Height, rs.Round, index)
				continue OUTER_LOOP
			}
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

		trySendVote := func(voteSet *VoteSet, peerVoteSet BitArray) (sent bool) {
			// TODO: give priority to our vote.
			// peerVoteSet BitArray is being accessed concurrently with
			// writes from Receive() routines.  We must lock like so here:
			ps.mtx.Lock()
			index, ok := voteSet.BitArray().Sub(peerVoteSet).PickRandom()
			ps.mtx.Unlock()
			if ok {
				vote := voteSet.GetByIndex(index)
				// NOTE: vote may be a commit.
				msg := &VoteMessage{index, vote}
				peer.Send(VoteCh, msg)
				ps.SetHasVote(vote, index)
				return true
			}
			return false
		}

		// If height matches, then send LastCommits, Prevotes, Precommits, or Commits.
		if rs.Height == prs.Height {

			// If there are lastcommits to send...
			if prs.Round == 0 && prs.Step == RoundStepNewHeight {
				if prs.LastCommits.Size() == rs.LastCommits.Size() {
					if trySendVote(rs.LastCommits, prs.LastCommits) {
						continue OUTER_LOOP
					}
				}
			}

			// Initialize Prevotes/Precommits/Commits if needed
			ps.EnsureVoteBitArrays(rs.Height, rs.Validators.Size())

			// If there are prevotes to send...
			if rs.Round == prs.Round && prs.Step <= RoundStepPrevote {
				if trySendVote(rs.Prevotes, prs.Prevotes) {
					continue OUTER_LOOP
				}
			}

			// If there are precommits to send...
			if rs.Round == prs.Round && prs.Step <= RoundStepPrecommit {
				if trySendVote(rs.Precommits, prs.Precommits) {
					continue OUTER_LOOP
				}
			}

			// If there are any commits to send...
			if trySendVote(rs.Commits, prs.Commits) {
				continue OUTER_LOOP
			}
		}

		// If peer is lagging by height 1, match our LastCommits to peer's Commits.
		if rs.Height == prs.Height+1 {

			// Initialize Commits if needed
			ps.EnsureVoteBitArrays(rs.Height-1, rs.LastCommits.Size())

			// If there are lastcommits to send...
			if trySendVote(rs.LastCommits, prs.Commits) {
				continue OUTER_LOOP
			}

		}

		// If peer is lagging by more than 1, load and send Validation and send Commits.
		if rs.Height >= prs.Height+2 {

			// Load the block header and validation for prs.Height+1,
			// which contains commit signatures for prs.Height.
			header, validation := conR.conS.LoadHeaderValidation(prs.Height + 1)
			size := uint(len(validation.Commits))

			// Initialize Commits if needed
			ps.EnsureVoteBitArrays(prs.Height, size)

			index, ok := validation.BitArray().Sub(prs.Commits).PickRandom()
			if ok {
				rsig := validation.Commits[index]
				// Reconstruct vote.
				vote := &Vote{
					Height:     prs.Height,
					Round:      rsig.Round,
					Type:       VoteTypeCommit,
					BlockHash:  header.LastBlockHash,
					BlockParts: header.LastBlockParts,
					Signature:  rsig.Signature,
				}
				msg := &VoteMessage{index, vote}
				peer.Send(VoteCh, msg)
				ps.SetHasVote(vote, index)
				continue OUTER_LOOP
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
	Height                uint          // Height peer is at
	Round                 uint          // Round peer is at
	Step                  RoundStep     // Step peer is at
	StartTime             time.Time     // Estimated start of round 0 at this height
	Proposal              bool          // True if peer has proposal for this round
	ProposalBlockParts    PartSetHeader //
	ProposalBlockBitArray BitArray      // True bit -> has part
	ProposalPOLParts      PartSetHeader //
	ProposalPOLBitArray   BitArray      // True bit -> has part
	Prevotes              BitArray      // All votes peer has for this round
	Precommits            BitArray      // All precommits peer has for this round
	Commits               BitArray      // All commits peer has for this height
	LastCommits           BitArray      // All commits peer has for last height
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

func (ps *PeerState) SetHasProposal(proposal *Proposal) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != proposal.Height || ps.Round != proposal.Round {
		return
	}
	if ps.Proposal {
		return
	}

	ps.Proposal = true
	ps.ProposalBlockParts = proposal.BlockParts
	ps.ProposalBlockBitArray = NewBitArray(uint(proposal.BlockParts.Total))
	ps.ProposalPOLParts = proposal.POLParts
	ps.ProposalPOLBitArray = NewBitArray(uint(proposal.POLParts.Total))
}

func (ps *PeerState) SetHasProposalBlockPart(height uint, round uint, index uint) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != height || ps.Round != round {
		return
	}

	ps.ProposalBlockBitArray.SetIndex(uint(index), true)
}

func (ps *PeerState) SetHasProposalPOLPart(height uint, round uint, index uint) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != height || ps.Round != round {
		return
	}

	ps.ProposalPOLBitArray.SetIndex(uint(index), true)
}

func (ps *PeerState) EnsureVoteBitArrays(height uint, numValidators uint) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != height {
		return
	}

	if ps.Prevotes.IsZero() {
		ps.Prevotes = NewBitArray(numValidators)
	}
	if ps.Precommits.IsZero() {
		ps.Precommits = NewBitArray(numValidators)
	}
	if ps.Commits.IsZero() {
		ps.Commits = NewBitArray(numValidators)
	}
}

func (ps *PeerState) SetHasVote(vote *Vote, index uint) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	ps.setHasVote(vote.Height, vote.Round, vote.Type, index)
}

func (ps *PeerState) setHasVote(height uint, round uint, type_ byte, index uint) {
	if ps.Height == height+1 && type_ == VoteTypeCommit {
		// Special case for LastCommits.
		ps.LastCommits.SetIndex(index, true)
		return
	} else if ps.Height != height {
		// Does not apply.
		return
	}

	switch type_ {
	case VoteTypePrevote:
		ps.Prevotes.SetIndex(index, true)
	case VoteTypePrecommit:
		ps.Precommits.SetIndex(index, true)
	case VoteTypeCommit:
		if round < ps.Round {
			ps.Prevotes.SetIndex(index, true)
			ps.Precommits.SetIndex(index, true)
		}
		ps.Commits.SetIndex(index, true)
	default:
		panic("Invalid vote type")
	}
}

func (ps *PeerState) ApplyNewRoundStepMessage(msg *NewRoundStepMessage, rs *RoundState) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	// Just remember these values.
	psHeight := ps.Height
	psRound := ps.Round
	//psStep := ps.Step

	startTime := time.Now().Add(-1 * time.Duration(msg.SecondsSinceStartTime) * time.Second)
	ps.Height = msg.Height
	ps.Round = msg.Round
	ps.Step = msg.Step
	ps.StartTime = startTime
	if psHeight != msg.Height || psRound != msg.Round {
		ps.Proposal = false
		ps.ProposalBlockParts = PartSetHeader{}
		ps.ProposalBlockBitArray = BitArray{}
		ps.ProposalPOLParts = PartSetHeader{}
		ps.ProposalPOLBitArray = BitArray{}
		// We'll update the BitArray capacity later.
		ps.Prevotes = BitArray{}
		ps.Precommits = BitArray{}
	}
	if psHeight != msg.Height {
		// Shift Commits to LastCommits
		if psHeight+1 == msg.Height {
			ps.LastCommits = ps.Commits
		} else {
			ps.LastCommits = BitArray{}
		}
		// We'll update the BitArray capacity later.
		ps.Commits = BitArray{}
	}
}

func (ps *PeerState) ApplyCommitStepMessage(msg *CommitStepMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != msg.Height {
		return
	}

	ps.ProposalBlockParts = msg.BlockParts
	ps.ProposalBlockBitArray = msg.BlockBitArray
}

func (ps *PeerState) ApplyHasVoteMessage(msg *HasVoteMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	// Special case for LastCommits
	if ps.Height == msg.Height+1 && msg.Type == VoteTypeCommit {
		ps.LastCommits.SetIndex(msg.Index, true)
		return
	} else if ps.Height != msg.Height {
		return
	}

	ps.setHasVote(msg.Height, msg.Round, msg.Type, msg.Index)
}

//-----------------------------------------------------------------------------
// Messages

const (
	msgTypeUnknown      = byte(0x00)
	msgTypeNewRoundStep = byte(0x01)
	msgTypeCommitStep   = byte(0x02)
	msgTypeProposal     = byte(0x11)
	msgTypePart         = byte(0x12) // both block & POL
	msgTypeVote         = byte(0x13)
	msgTypeHasVote      = byte(0x14)
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
		msg = ReadBinary(&NewRoundStepMessage{}, r, n, err)
	case msgTypeCommitStep:
		msg = ReadBinary(&CommitStepMessage{}, r, n, err)
	// Messages of data
	case msgTypeProposal:
		msg = ReadBinary(&Proposal{}, r, n, err)
	case msgTypePart:
		msg = ReadBinary(&PartMessage{}, r, n, err)
	case msgTypeVote:
		msg = ReadBinary(&VoteMessage{}, r, n, err)
	case msgTypeHasVote:
		msg = ReadBinary(&HasVoteMessage{}, r, n, err)
	default:
		msg = nil
	}
	return
}

//-------------------------------------

type NewRoundStepMessage struct {
	Height                uint
	Round                 uint
	Step                  RoundStep
	SecondsSinceStartTime uint
}

func (m *NewRoundStepMessage) TypeByte() byte { return msgTypeNewRoundStep }

func (m *NewRoundStepMessage) String() string {
	return fmt.Sprintf("[NewRoundStep %v/%v/%X]", m.Height, m.Round, m.Step)
}

//-------------------------------------

type CommitStepMessage struct {
	Height        uint
	BlockParts    PartSetHeader
	BlockBitArray BitArray
}

func (m *CommitStepMessage) TypeByte() byte { return msgTypeCommitStep }

func (m *CommitStepMessage) String() string {
	return fmt.Sprintf("[CommitStep %v %v %v]", m.Height, m.BlockParts, m.BlockBitArray)
}

//-------------------------------------

const (
	partTypeProposalBlock = byte(0x01)
	partTypeProposalPOL   = byte(0x02)
)

type PartMessage struct {
	Height uint
	Round  uint
	Type   byte
	Part   *Part
}

func (m *PartMessage) TypeByte() byte { return msgTypePart }

func (m *PartMessage) String() string {
	return fmt.Sprintf("[Part %v/%v T:%X %v]", m.Height, m.Round, m.Type, m.Part)
}

//-------------------------------------

type VoteMessage struct {
	ValidatorIndex uint
	Vote           *Vote
}

func (m *VoteMessage) TypeByte() byte { return msgTypeVote }

func (m *VoteMessage) String() string {
	return fmt.Sprintf("[Vote ValidatorIndex:%v Vote:%v]", m.ValidatorIndex, m.Vote)
}

//-------------------------------------

type HasVoteMessage struct {
	Height uint
	Round  uint
	Type   byte
	Index  uint
}

func (m *HasVoteMessage) TypeByte() byte { return msgTypeHasVote }

func (m *HasVoteMessage) String() string {
	return fmt.Sprintf("[HasVote %v/%v T:%X]", m.Height, m.Round, m.Type)
}
