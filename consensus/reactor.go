package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tendermint/tendermint/binary"
	bc "github.com/tendermint/tendermint/blockchain"
	. "github.com/tendermint/tendermint/common"
	. "github.com/tendermint/tendermint/consensus/types"
	"github.com/tendermint/tendermint/p2p"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

const (
	StateChannel = byte(0x20)
	DataChannel  = byte(0x21)
	VoteChannel  = byte(0x22)

	peerStateKey = "ConsensusReactor.peerState"

	peerGossipSleepDuration = 100 * time.Millisecond // Time to sleep if there's nothing to send.
)

//-----------------------------------------------------------------------------

// The reactor's underlying ConsensusState may change state at any time.
// We atomically copy the RoundState struct before using it.
type ConsensusReactor struct {
	sw      *p2p.Switch
	running uint32
	quit    chan struct{}

	blockStore *bc.BlockStore
	conS       *ConsensusState
}

func NewConsensusReactor(consensusState *ConsensusState, blockStore *bc.BlockStore) *ConsensusReactor {
	conR := &ConsensusReactor{
		blockStore: blockStore,
		quit:       make(chan struct{}),
		conS:       consensusState,
	}
	return conR
}

// Implements Reactor
func (conR *ConsensusReactor) Start(sw *p2p.Switch) {
	if atomic.CompareAndSwapUint32(&conR.running, 0, 1) {
		log.Info("Starting ConsensusReactor")
		conR.sw = sw
		conR.conS.Start()
		go conR.broadcastNewRoundStepRoutine()
	}
}

// Implements Reactor
func (conR *ConsensusReactor) Stop() {
	if atomic.CompareAndSwapUint32(&conR.running, 1, 0) {
		log.Info("Stopping ConsensusReactor")
		conR.conS.Stop()
		close(conR.quit)
	}
}

func (conR *ConsensusReactor) IsRunning() bool {
	return atomic.LoadUint32(&conR.running) == 0
}

// Implements Reactor
func (conR *ConsensusReactor) GetChannels() []*p2p.ChannelDescriptor {
	// TODO optimize
	return []*p2p.ChannelDescriptor{
		&p2p.ChannelDescriptor{
			Id:       StateChannel,
			Priority: 5,
		},
		&p2p.ChannelDescriptor{
			Id:       DataChannel,
			Priority: 5,
		},
		&p2p.ChannelDescriptor{
			Id:       VoteChannel,
			Priority: 5,
		},
	}
}

// Implements Reactor
func (conR *ConsensusReactor) AddPeer(peer *p2p.Peer) {
	if !conR.IsRunning() {
		return
	}

	// Create peerState for peer
	peerState := NewPeerState(peer)
	peer.Data.Set(peerStateKey, peerState)

	// Begin gossip routines for this peer.
	go conR.gossipDataRoutine(peer, peerState)
	go conR.gossipVotesRoutine(peer, peerState)

	// Send our state to peer.
	conR.sendNewRoundStepRoutine(peer)
}

// Implements Reactor
func (conR *ConsensusReactor) RemovePeer(peer *p2p.Peer, reason interface{}) {
	if !conR.IsRunning() {
		return
	}

	//peer.Data.Get(peerStateKey).(*PeerState).Disconnect()
}

// Implements Reactor
func (conR *ConsensusReactor) Receive(chId byte, peer *p2p.Peer, msgBytes []byte) {
	if !conR.IsRunning() {
		return
	}

	// Get round state
	rs := conR.conS.GetRoundState()
	ps := peer.Data.Get(peerStateKey).(*PeerState)
	_, msg_, err := DecodeMessage(msgBytes)
	if err != nil {
		log.Warn("Error decoding message", "channel", chId, "peer", peer, "msg", msg_, "error", err, "bytes", msgBytes)
		return
	}
	log.Debug("Receive", "channel", chId, "peer", peer, "msg", msg_, "bytes", msgBytes)

	switch chId {
	case StateChannel:
		switch msg := msg_.(type) {
		case *NewRoundStepMessage:
			ps.ApplyNewRoundStepMessage(msg, rs)
		case *CommitStepMessage:
			ps.ApplyCommitStepMessage(msg)
		case *HasVoteMessage:
			ps.ApplyHasVoteMessage(msg)
		default:
			// Ignore unknown message
		}

	case DataChannel:
		switch msg := msg_.(type) {
		case *Proposal:
			ps.SetHasProposal(msg)
			err = conR.conS.SetProposal(msg)

		case *PartMessage:
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

	case VoteChannel:
		switch msg := msg_.(type) {
		case *VoteMessage:
			vote := msg.Vote
			if rs.Height != vote.Height {
				return // Wrong height. Not necessarily a bad peer.
			}
			validatorIndex := msg.ValidatorIndex
			address, _ := rs.Validators.GetByIndex(validatorIndex)
			added, index, err := conR.conS.AddVote(address, vote)
			if err != nil {
				// If conflicting sig, broadcast evidence tx for slashing. Else punish peer.
				if errDupe, ok := err.(*types.ErrVoteConflictingSignature); ok {
					log.Warn("Found conflicting vote. Publish evidence")
					evidenceTx := &types.DupeoutTx{
						Address: address,
						VoteA:   *errDupe.VoteA,
						VoteB:   *errDupe.VoteB,
					}
					conR.conS.mempoolReactor.BroadcastTx(evidenceTx) // shouldn't need to check returned err
				} else {
					// Probably an invalid signature. Bad peer.
					log.Warn("Error attempting to add vote", "error", err)

					// TODO: punish peer
				}
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
				conR.sw.Broadcast(StateChannel, msg)
			}

		default:
			// Ignore unknown message
		}
	default:
		// Ignore unknown channel
	}

	if err != nil {
		log.Warn("Error in Receive()", "error", err)
	}
}

// Sets our private validator account for signing votes.
func (conR *ConsensusReactor) SetPrivValidator(priv *sm.PrivValidator) {
	conR.conS.SetPrivValidator(priv)
}

func (conR *ConsensusReactor) UpdateToState(state *sm.State) {
	conR.conS.updateToState(state, false)
}

//--------------------------------------

func makeRoundStepMessages(rs *RoundState) (nrsMsg *NewRoundStepMessage, csMsg *CommitStepMessage) {
	// Get seconds since beginning of height.
	timeElapsed := time.Now().Sub(rs.StartTime)

	// Broadcast NewRoundStepMessage
	nrsMsg = &NewRoundStepMessage{
		Height: rs.Height,
		Round:  rs.Round,
		Step:   rs.Step,
		SecondsSinceStartTime: uint(timeElapsed.Seconds()),
	}

	// If the step is commit, then also broadcast a CommitStepMessage.
	if rs.Step == RoundStepCommit {
		csMsg = &CommitStepMessage{
			Height:        rs.Height,
			BlockParts:    rs.ProposalBlockParts.Header(),
			BlockBitArray: rs.ProposalBlockParts.BitArray(),
		}
	}

	return
}

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

		nrsMsg, csMsg := makeRoundStepMessages(rs)
		if nrsMsg != nil {
			conR.sw.Broadcast(StateChannel, nrsMsg)
		}
		if csMsg != nil {
			conR.sw.Broadcast(StateChannel, csMsg)
		}
	}
}

func (conR *ConsensusReactor) sendNewRoundStepRoutine(peer *p2p.Peer) {
	rs := conR.conS.GetRoundState()
	nrsMsg, csMsg := makeRoundStepMessages(rs)
	if nrsMsg != nil {
		peer.Send(StateChannel, nrsMsg)
	}
	if csMsg != nil {
		peer.Send(StateChannel, nrsMsg)
	}
}

func (conR *ConsensusReactor) gossipDataRoutine(peer *p2p.Peer, ps *PeerState) {

OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if !peer.IsRunning() || !conR.IsRunning() {
			log.Info(Fmt("Stopping gossipDataRoutine for %v.", peer))
			return
		}
		rs := conR.conS.GetRoundState()
		prs := ps.GetRoundState()

		// Send proposal Block parts?
		// NOTE: if we or peer is at RoundStepCommit*, the round
		// won't necessarily match, but that's OK.
		if rs.ProposalBlockParts.HasHeader(prs.ProposalBlockParts) {
			//log.Debug("ProposalBlockParts matched", "blockParts", prs.ProposalBlockParts)
			if index, ok := rs.ProposalBlockParts.BitArray().Sub(prs.ProposalBlockBitArray.Copy()).PickRandom(); ok {
				part := rs.ProposalBlockParts.GetPart(index)
				msg := &PartMessage{
					Height: rs.Height,
					Round:  rs.Round,
					Type:   partTypeProposalBlock,
					Part:   part,
				}
				peer.Send(DataChannel, msg)
				ps.SetHasProposalBlockPart(rs.Height, rs.Round, index)
				continue OUTER_LOOP
			}
		}

		// If the peer is on a previous height, help catch up.
		if 0 < prs.Height && prs.Height < rs.Height {
			//log.Debug("Data catchup", "height", rs.Height, "peerHeight", prs.Height, "peerProposalBlockBitArray", prs.ProposalBlockBitArray)
			if index, ok := prs.ProposalBlockBitArray.Not().PickRandom(); ok {
				// Ensure that the peer's PartSetHeader is correct
				blockMeta := conR.blockStore.LoadBlockMeta(prs.Height)
				if !blockMeta.Parts.Equals(prs.ProposalBlockParts) {
					log.Debug("Peer ProposalBlockParts mismatch, sleeping",
						"peerHeight", prs.Height, "blockParts", blockMeta.Parts, "peerBlockParts", prs.ProposalBlockParts)
					time.Sleep(peerGossipSleepDuration)
					continue OUTER_LOOP
				}
				// Load the part
				part := conR.blockStore.LoadBlockPart(prs.Height, index)
				if part == nil {
					log.Warn("Could not load part", "index", index,
						"peerHeight", prs.Height, "blockParts", blockMeta.Parts, "peerBlockParts", prs.ProposalBlockParts)
					time.Sleep(peerGossipSleepDuration)
					continue OUTER_LOOP
				}
				// Send the part
				msg := &PartMessage{
					Height: prs.Height,
					Round:  prs.Round,
					Type:   partTypeProposalBlock,
					Part:   part,
				}
				peer.Send(DataChannel, msg)
				ps.SetHasProposalBlockPart(prs.Height, prs.Round, index)
				continue OUTER_LOOP
			} else {
				//log.Debug("No parts to send in catch-up, sleeping")
				time.Sleep(peerGossipSleepDuration)
				continue OUTER_LOOP
			}
		}

		// If height and round don't match, sleep.
		if rs.Height != prs.Height || rs.Round != prs.Round {
			//log.Debug("Peer Height|Round mismatch, sleeping", "peerHeight", prs.Height, "peerRound", prs.Round, "peer", peer)
			time.Sleep(peerGossipSleepDuration)
			continue OUTER_LOOP
		}

		// Send proposal?
		if rs.Proposal != nil && !prs.Proposal {
			msg := p2p.TypedMessage{msgTypeProposal, rs.Proposal}
			peer.Send(DataChannel, msg)
			ps.SetHasProposal(rs.Proposal)
			continue OUTER_LOOP
		}

		// Send proposal POL parts?
		if rs.ProposalPOLParts.HasHeader(prs.ProposalPOLParts) {
			if index, ok := rs.ProposalPOLParts.BitArray().Sub(prs.ProposalPOLBitArray.Copy()).PickRandom(); ok {
				msg := &PartMessage{
					Height: rs.Height,
					Round:  rs.Round,
					Type:   partTypeProposalPOL,
					Part:   rs.ProposalPOLParts.GetPart(index),
				}
				peer.Send(DataChannel, msg)
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
		if !peer.IsRunning() || !conR.IsRunning() {
			log.Info(Fmt("Stopping gossipVotesRoutine for %v.", peer))
			return
		}
		rs := conR.conS.GetRoundState()
		prs := ps.GetRoundState()

		trySendVote := func(voteSet *VoteSet, peerVoteSet BitArray) (sent bool) {
			if prs.Height == voteSet.Height() {
				// Initialize Prevotes/Precommits/Commits if needed
				ps.EnsureVoteBitArrays(prs.Height, voteSet.Size())
			}

			// TODO: give priority to our vote.
			if index, ok := voteSet.BitArray().Sub(peerVoteSet.Copy()).PickRandom(); ok {
				vote := voteSet.GetByIndex(index)
				// NOTE: vote may be a commit.
				msg := &VoteMessage{index, vote}
				peer.Send(VoteChannel, msg)
				ps.SetHasVote(vote, index)
				return true
			}
			return false
		}

		trySendCommitFromValidation := func(blockMeta *types.BlockMeta, validation *types.Validation, peerVoteSet BitArray) (sent bool) {
			// Initialize Commits if needed
			ps.EnsureVoteBitArrays(prs.Height, uint(len(validation.Commits)))

			if index, ok := validation.BitArray().Sub(prs.Commits.Copy()).PickRandom(); ok {
				commit := validation.Commits[index]
				log.Debug("Picked commit to send", "index", index, "commit", commit)
				// Reconstruct vote.
				vote := &types.Vote{
					Height:     prs.Height,
					Round:      commit.Round,
					Type:       types.VoteTypeCommit,
					BlockHash:  blockMeta.Hash,
					BlockParts: blockMeta.Parts,
					Signature:  commit.Signature,
				}
				msg := &VoteMessage{index, vote}
				peer.Send(VoteChannel, msg)
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

		// Catchup logic
		if prs.Height != 0 && !prs.HasAllCatchupCommits {

			// If peer is lagging by height 1, match our LastCommits or SeenValidation to peer's Commits.
			if rs.Height == prs.Height+1 && rs.LastCommits.Size() > 0 {
				// If there are lastcommits to send...
				if trySendVote(rs.LastCommits, prs.Commits) {
					continue OUTER_LOOP
				} else {
					ps.SetHasAllCatchupCommits(prs.Height)
				}
			}

			// Or, if peer is lagging by 1 and we don't have LastCommits, send SeenValidation.
			if rs.Height == prs.Height+1 && rs.LastCommits.Size() == 0 {
				// Load the blockMeta for block at prs.Height
				blockMeta := conR.blockStore.LoadBlockMeta(prs.Height)
				// Load the seen validation for prs.Height
				validation := conR.blockStore.LoadSeenValidation(prs.Height)
				log.Debug("Loaded SeenValidation for catch-up", "height", prs.Height, "blockMeta", blockMeta, "validation", validation)

				if trySendCommitFromValidation(blockMeta, validation, prs.Commits) {
					continue OUTER_LOOP
				} else {
					ps.SetHasAllCatchupCommits(prs.Height)
				}
			}

			// If peer is lagging by more than 1, send Validation.
			if rs.Height >= prs.Height+2 {
				// Load the blockMeta for block at prs.Height
				blockMeta := conR.blockStore.LoadBlockMeta(prs.Height)
				// Load the block validation for prs.Height+1,
				// which contains commit signatures for prs.Height.
				validation := conR.blockStore.LoadBlockValidation(prs.Height + 1)
				log.Debug("Loaded BlockValidation for catch-up", "height", prs.Height+1, "blockMeta", blockMeta, "validation", validation)

				if trySendCommitFromValidation(blockMeta, validation, prs.Commits) {
					continue OUTER_LOOP
				} else {
					ps.SetHasAllCatchupCommits(prs.Height)
				}
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
	Height                uint                // Height peer is at
	Round                 uint                // Round peer is at
	Step                  RoundStep           // Step peer is at
	StartTime             time.Time           // Estimated start of round 0 at this height
	Proposal              bool                // True if peer has proposal for this round
	ProposalBlockParts    types.PartSetHeader //
	ProposalBlockBitArray BitArray            // True bit -> has part
	ProposalPOLParts      types.PartSetHeader //
	ProposalPOLBitArray   BitArray            // True bit -> has part
	Prevotes              BitArray            // All votes peer has for this round
	Precommits            BitArray            // All precommits peer has for this round
	Commits               BitArray            // All commits peer has for this height
	LastCommits           BitArray            // All commits peer has for last height
	HasAllCatchupCommits  bool                // Used for catch-up
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

func (ps *PeerState) SetHasVote(vote *types.Vote, index uint) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	ps.setHasVote(vote.Height, vote.Round, vote.Type, index)
}

func (ps *PeerState) setHasVote(height uint, round uint, type_ byte, index uint) {
	if ps.Height == height+1 && type_ == types.VoteTypeCommit {
		// Special case for LastCommits.
		ps.LastCommits.SetIndex(index, true)
		return
	} else if ps.Height != height {
		// Does not apply.
		return
	}

	switch type_ {
	case types.VoteTypePrevote:
		ps.Prevotes.SetIndex(index, true)
	case types.VoteTypePrecommit:
		ps.Precommits.SetIndex(index, true)
	case types.VoteTypeCommit:
		if round < ps.Round {
			ps.Prevotes.SetIndex(index, true)
			ps.Precommits.SetIndex(index, true)
		}
		ps.Commits.SetIndex(index, true)
	default:
		panic("Invalid vote type")
	}
}

// When catching up, this helps keep track of whether
// we should send more commit votes from the block (validation) store
func (ps *PeerState) SetHasAllCatchupCommits(height uint) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height == height {
		ps.HasAllCatchupCommits = true
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
		ps.ProposalBlockParts = types.PartSetHeader{}
		ps.ProposalBlockBitArray = BitArray{}
		ps.ProposalPOLParts = types.PartSetHeader{}
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
		ps.HasAllCatchupCommits = false
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
	if ps.Height == msg.Height+1 && msg.Type == types.VoteTypeCommit {
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
func DecodeMessage(bz []byte) (msgType byte, msg interface{}, err error) {
	n := new(int64)
	// log.Debug(Fmt("decoding msg bytes: %X", bz))
	msgType = bz[0]
	r := bytes.NewReader(bz)
	switch msgType {
	// Messages for communicating state changes
	case msgTypeNewRoundStep:
		msg = binary.ReadBinary(&NewRoundStepMessage{}, r, n, &err)
	case msgTypeCommitStep:
		msg = binary.ReadBinary(&CommitStepMessage{}, r, n, &err)
	// Messages of data
	case msgTypeProposal:
		r.ReadByte() // Consume the byte
		msg = binary.ReadBinary(&Proposal{}, r, n, &err)
	case msgTypePart:
		msg = binary.ReadBinary(&PartMessage{}, r, n, &err)
	case msgTypeVote:
		msg = binary.ReadBinary(&VoteMessage{}, r, n, &err)
	case msgTypeHasVote:
		msg = binary.ReadBinary(&HasVoteMessage{}, r, n, &err)
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
	return fmt.Sprintf("[NewRoundStep H:%v R:%v S:%v]", m.Height, m.Round, m.Step)
}

//-------------------------------------

type CommitStepMessage struct {
	Height        uint
	BlockParts    types.PartSetHeader
	BlockBitArray BitArray
}

func (m *CommitStepMessage) TypeByte() byte { return msgTypeCommitStep }

func (m *CommitStepMessage) String() string {
	return fmt.Sprintf("[CommitStep H:%v BP:%v BA:%v]", m.Height, m.BlockParts, m.BlockBitArray)
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
	Part   *types.Part
}

func (m *PartMessage) TypeByte() byte { return msgTypePart }

func (m *PartMessage) String() string {
	return fmt.Sprintf("[Part H:%v R:%v T:%X P:%v]", m.Height, m.Round, m.Type, m.Part)
}

//-------------------------------------

type VoteMessage struct {
	ValidatorIndex uint
	Vote           *types.Vote
}

func (m *VoteMessage) TypeByte() byte { return msgTypeVote }

func (m *VoteMessage) String() string {
	return fmt.Sprintf("[Vote VI:%v V:%v]", m.ValidatorIndex, m.Vote)
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
