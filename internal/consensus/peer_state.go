package consensus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	cstypes "github.com/tendermint/tendermint/internal/consensus/types"
	"github.com/tendermint/tendermint/libs/bits"
	"github.com/tendermint/tendermint/libs/log"
	tmtime "github.com/tendermint/tendermint/libs/time"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

var (
	ErrPeerStateHeightRegression = errors.New("peer state height regression")
	ErrPeerStateInvalidStartTime = errors.New("peer state invalid startTime")
	ErrPeerStateSetNilVote       = errors.New("peer state set a nil vote")
	ErrPeerStateInvalidVoteIndex = errors.New("peer sent a vote with an invalid vote index")
)

// peerStateStats holds internal statistics for a peer.
type peerStateStats struct {
	Votes      int `json:"votes,string"`
	BlockParts int `json:"block_parts,string"`
}

func (pss peerStateStats) String() string {
	return fmt.Sprintf("peerStateStats{votes: %d, blockParts: %d}", pss.Votes, pss.BlockParts)
}

// PeerState contains the known state of a peer, including its connection and
// threadsafe access to its PeerRoundState.
// NOTE: THIS GETS DUMPED WITH rpc/core/consensus.go.
// Be mindful of what you Expose.
type PeerState struct {
	peerID types.NodeID
	logger log.Logger

	// NOTE: Modify below using setters, never directly.
	mtx     sync.RWMutex
	cancel  context.CancelFunc
	running bool
	PRS     cstypes.PeerRoundState `json:"round_state"`
	Stats   *peerStateStats        `json:"stats"`
}

// NewPeerState returns a new PeerState for the given node ID.
func NewPeerState(logger log.Logger, peerID types.NodeID) *PeerState {
	return &PeerState{
		peerID: peerID,
		logger: logger,
		PRS: cstypes.PeerRoundState{
			Round:              -1,
			ProposalPOLRound:   -1,
			LastCommitRound:    -1,
			CatchupCommitRound: -1,
		},
		Stats: &peerStateStats{},
	}
}

// SetRunning sets the running state of the peer.
func (ps *PeerState) SetRunning(v bool) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	ps.running = v
}

// IsRunning returns true if a PeerState is considered running where multiple
// broadcasting goroutines exist for the peer.
func (ps *PeerState) IsRunning() bool {
	ps.mtx.RLock()
	defer ps.mtx.RUnlock()

	return ps.running
}

// GetRoundState returns a shallow copy of the PeerRoundState. There's no point
// in mutating it since it won't change PeerState.
func (ps *PeerState) GetRoundState() *cstypes.PeerRoundState {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	prs := ps.PRS.Copy()
	return &prs
}

// ToJSON returns a json of PeerState.
func (ps *PeerState) ToJSON() ([]byte, error) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return json.Marshal(ps)
}

// GetHeight returns an atomic snapshot of the PeerRoundState's height used by
// the mempool to ensure peers are caught up before broadcasting new txs.
func (ps *PeerState) GetHeight() int64 {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	return ps.PRS.Height
}

// SetHasProposal sets the given proposal as known for the peer.
func (ps *PeerState) SetHasProposal(proposal *types.Proposal) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.PRS.Height != proposal.Height || ps.PRS.Round != proposal.Round {
		return
	}

	if ps.PRS.Proposal {
		return
	}

	ps.PRS.Proposal = true

	// ps.PRS.ProposalBlockParts is set due to NewValidBlockMessage
	if ps.PRS.ProposalBlockParts != nil {
		return
	}

	ps.PRS.ProposalBlockPartSetHeader = proposal.BlockID.PartSetHeader
	ps.PRS.ProposalBlockParts = bits.NewBitArray(int(proposal.BlockID.PartSetHeader.Total))
	ps.PRS.ProposalPOLRound = proposal.POLRound
	ps.PRS.ProposalPOL = nil // Nil until ProposalPOLMessage received.
}

// InitProposalBlockParts initializes the peer's proposal block parts header
// and bit array.
func (ps *PeerState) InitProposalBlockParts(partSetHeader types.PartSetHeader) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.PRS.ProposalBlockParts != nil {
		return
	}

	ps.PRS.ProposalBlockPartSetHeader = partSetHeader
	ps.PRS.ProposalBlockParts = bits.NewBitArray(int(partSetHeader.Total))
}

// SetHasProposalBlockPart sets the given block part index as known for the peer.
func (ps *PeerState) SetHasProposalBlockPart(height int64, round int32, index int) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.PRS.Height != height || ps.PRS.Round != round {
		return
	}

	ps.PRS.ProposalBlockParts.SetIndex(index, true)
}

// PickVoteToSend picks a vote to send to the peer. It will return true if a
// vote was picked.
//
// NOTE: `votes` must be the correct Size() for the Height().
func (ps *PeerState) PickVoteToSend(votes types.VoteSetReader) (*types.Vote, bool) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if votes.Size() == 0 {
		return nil, false
	}

	var (
		height    = votes.GetHeight()
		round     = votes.GetRound()
		votesType = tmproto.SignedMsgType(votes.Type())
		size      = votes.Size()
	)

	// lazily set data using 'votes'
	if votes.IsCommit() {
		ps.ensureCatchupCommitRound(height, round, size)
	}

	ps.ensureVoteBitArrays(height, size)

	psVotes := ps.getVoteBitArray(height, round, votesType)
	if psVotes == nil {
		return nil, false // not something worth sending
	}

	if index, ok := votes.BitArray().Sub(psVotes).PickRandom(); ok {
		vote := votes.GetByIndex(int32(index))
		if vote != nil {
			return vote, true
		}
	}

	return nil, false
}

func (ps *PeerState) getVoteBitArray(height int64, round int32, votesType tmproto.SignedMsgType) *bits.BitArray {
	if !types.IsVoteTypeValid(votesType) {
		return nil
	}

	if ps.PRS.Height == height {
		if ps.PRS.Round == round {
			switch votesType {
			case tmproto.PrevoteType:
				return ps.PRS.Prevotes

			case tmproto.PrecommitType:
				return ps.PRS.Precommits
			}
		}

		if ps.PRS.CatchupCommitRound == round {
			switch votesType {
			case tmproto.PrevoteType:
				return nil

			case tmproto.PrecommitType:
				return ps.PRS.CatchupCommit
			}
		}

		if ps.PRS.ProposalPOLRound == round {
			switch votesType {
			case tmproto.PrevoteType:
				return ps.PRS.ProposalPOL

			case tmproto.PrecommitType:
				return nil
			}
		}

		return nil
	}
	if ps.PRS.Height == height+1 {
		if ps.PRS.LastCommitRound == round {
			switch votesType {
			case tmproto.PrevoteType:
				return nil

			case tmproto.PrecommitType:
				return ps.PRS.LastCommit
			}
		}

		return nil
	}

	return nil
}

// 'round': A round for which we have a +2/3 commit.
func (ps *PeerState) ensureCatchupCommitRound(height int64, round int32, numValidators int) {
	if ps.PRS.Height != height {
		return
	}

	/*
		NOTE: This is wrong, 'round' could change.
		e.g. if orig round is not the same as block LastCommit round.
		if ps.CatchupCommitRound != -1 && ps.CatchupCommitRound != round {
			panic(fmt.Sprintf(
				"Conflicting CatchupCommitRound. Height: %v,
				Orig: %v,
				New: %v",
				height,
				ps.CatchupCommitRound,
				round))
		}
	*/

	if ps.PRS.CatchupCommitRound == round {
		return // Nothing to do!
	}

	ps.PRS.CatchupCommitRound = round
	if round == ps.PRS.Round {
		ps.PRS.CatchupCommit = ps.PRS.Precommits
	} else {
		ps.PRS.CatchupCommit = bits.NewBitArray(numValidators)
	}
}

// EnsureVoteBitArrays ensures the bit-arrays have been allocated for tracking
// what votes this peer has received.
// NOTE: It's important to make sure that numValidators actually matches
// what the node sees as the number of validators for height.
func (ps *PeerState) EnsureVoteBitArrays(height int64, numValidators int) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	ps.ensureVoteBitArrays(height, numValidators)
}

func (ps *PeerState) ensureVoteBitArrays(height int64, numValidators int) {
	if ps.PRS.Height == height {
		if ps.PRS.Prevotes == nil {
			ps.PRS.Prevotes = bits.NewBitArray(numValidators)
		}
		if ps.PRS.Precommits == nil {
			ps.PRS.Precommits = bits.NewBitArray(numValidators)
		}
		if ps.PRS.CatchupCommit == nil {
			ps.PRS.CatchupCommit = bits.NewBitArray(numValidators)
		}
		if ps.PRS.ProposalPOL == nil {
			ps.PRS.ProposalPOL = bits.NewBitArray(numValidators)
		}
	} else if ps.PRS.Height == height+1 {
		if ps.PRS.LastCommit == nil {
			ps.PRS.LastCommit = bits.NewBitArray(numValidators)
		}
	}
}

// RecordVote increments internal votes related statistics for this peer.
// It returns the total number of added votes.
func (ps *PeerState) RecordVote() int {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	ps.Stats.Votes++

	return ps.Stats.Votes
}

// VotesSent returns the number of blocks for which peer has been sending us
// votes.
func (ps *PeerState) VotesSent() int {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	return ps.Stats.Votes
}

// RecordBlockPart increments internal block part related statistics for this peer.
// It returns the total number of added block parts.
func (ps *PeerState) RecordBlockPart() int {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	ps.Stats.BlockParts++
	return ps.Stats.BlockParts
}

// BlockPartsSent returns the number of useful block parts the peer has sent us.
func (ps *PeerState) BlockPartsSent() int {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	return ps.Stats.BlockParts
}

// SetHasVote sets the given vote as known by the peer
func (ps *PeerState) SetHasVote(vote *types.Vote) error {
	// sanity check
	if vote == nil {
		return ErrPeerStateSetNilVote
	}
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	return ps.setHasVote(vote.Height, vote.Round, vote.Type, vote.ValidatorIndex)
}

// setHasVote will return an error when the index exceeds the bitArray length
func (ps *PeerState) setHasVote(height int64, round int32, voteType tmproto.SignedMsgType, index int32) error {
	logger := ps.logger.With(
		"peerH/R", fmt.Sprintf("%d/%d", ps.PRS.Height, ps.PRS.Round),
		"H/R", fmt.Sprintf("%d/%d", height, round),
	)

	logger.Debug("setHasVote", "type", voteType, "index", index)

	// NOTE: some may be nil BitArrays -> no side effects
	psVotes := ps.getVoteBitArray(height, round, voteType)
	if psVotes != nil {
		if ok := psVotes.SetIndex(int(index), true); !ok {
			// https://github.com/tendermint/tendermint/issues/2871
			return ErrPeerStateInvalidVoteIndex
		}
	}
	return nil
}

// ApplyNewRoundStepMessage updates the peer state for the new round.
func (ps *PeerState) ApplyNewRoundStepMessage(msg *NewRoundStepMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	// ignore duplicates or decreases
	if CompareHRS(msg.Height, msg.Round, msg.Step, ps.PRS.Height, ps.PRS.Round, ps.PRS.Step) <= 0 {
		return
	}

	var (
		psHeight             = ps.PRS.Height
		psRound              = ps.PRS.Round
		psCatchupCommitRound = ps.PRS.CatchupCommitRound
		psCatchupCommit      = ps.PRS.CatchupCommit
		startTime            = tmtime.Now().Add(-1 * time.Duration(msg.SecondsSinceStartTime) * time.Second)
	)

	ps.PRS.Height = msg.Height
	ps.PRS.Round = msg.Round
	ps.PRS.Step = msg.Step
	ps.PRS.StartTime = startTime

	if psHeight != msg.Height || psRound != msg.Round {
		ps.PRS.Proposal = false
		ps.PRS.ProposalBlockPartSetHeader = types.PartSetHeader{}
		ps.PRS.ProposalBlockParts = nil
		ps.PRS.ProposalPOLRound = -1
		ps.PRS.ProposalPOL = nil

		// we'll update the BitArray capacity later
		ps.PRS.Prevotes = nil
		ps.PRS.Precommits = nil
	}

	if psHeight == msg.Height && psRound != msg.Round && msg.Round == psCatchupCommitRound {
		// Peer caught up to CatchupCommitRound.
		// Preserve psCatchupCommit!
		// NOTE: We prefer to use prs.Precommits if
		// pr.Round matches pr.CatchupCommitRound.
		ps.PRS.Precommits = psCatchupCommit
	}

	if psHeight != msg.Height {
		// shift Precommits to LastCommit
		if psHeight+1 == msg.Height && psRound == msg.LastCommitRound {
			ps.PRS.LastCommitRound = msg.LastCommitRound
			ps.PRS.LastCommit = ps.PRS.Precommits
		} else {
			ps.PRS.LastCommitRound = msg.LastCommitRound
			ps.PRS.LastCommit = nil
		}

		// we'll update the BitArray capacity later
		ps.PRS.CatchupCommitRound = -1
		ps.PRS.CatchupCommit = nil
	}
}

// ApplyNewValidBlockMessage updates the peer state for the new valid block.
func (ps *PeerState) ApplyNewValidBlockMessage(msg *NewValidBlockMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.PRS.Height != msg.Height {
		return
	}

	if ps.PRS.Round != msg.Round && !msg.IsCommit {
		return
	}

	ps.PRS.ProposalBlockPartSetHeader = msg.BlockPartSetHeader
	ps.PRS.ProposalBlockParts = msg.BlockParts
}

// ApplyProposalPOLMessage updates the peer state for the new proposal POL.
func (ps *PeerState) ApplyProposalPOLMessage(msg *ProposalPOLMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.PRS.Height != msg.Height {
		return
	}
	if ps.PRS.ProposalPOLRound != msg.ProposalPOLRound {
		return
	}

	// TODO: Merge onto existing ps.PRS.ProposalPOL?
	// We might have sent some prevotes in the meantime.
	ps.PRS.ProposalPOL = msg.ProposalPOL
}

// ApplyHasVoteMessage updates the peer state for the new vote.
func (ps *PeerState) ApplyHasVoteMessage(msg *HasVoteMessage) error {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.PRS.Height != msg.Height {
		return nil
	}

	return ps.setHasVote(msg.Height, msg.Round, msg.Type, msg.Index)
}

// ApplyVoteSetBitsMessage updates the peer state for the bit-array of votes
// it claims to have for the corresponding BlockID.
// `ourVotes` is a BitArray of votes we have for msg.BlockID
// NOTE: if ourVotes is nil (e.g. msg.Height < rs.Height),
// we conservatively overwrite ps's votes w/ msg.Votes.
func (ps *PeerState) ApplyVoteSetBitsMessage(msg *VoteSetBitsMessage, ourVotes *bits.BitArray) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	votes := ps.getVoteBitArray(msg.Height, msg.Round, msg.Type)
	if votes != nil {
		if ourVotes == nil {
			votes.Update(msg.Votes)
		} else {
			otherVotes := votes.Sub(ourVotes)
			hasVotes := otherVotes.Or(msg.Votes)
			votes.Update(hasVotes)
		}
	}
}

// String returns a string representation of the PeerState
func (ps *PeerState) String() string {
	return ps.StringIndented("")
}

// StringIndented returns a string representation of the PeerState
func (ps *PeerState) StringIndented(indent string) string {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return fmt.Sprintf(`PeerState{
%s  Key        %v
%s  RoundState %v
%s  Stats      %v
%s}`,
		indent, ps.peerID,
		indent, ps.PRS.StringIndented(indent+"  "),
		indent, ps.Stats,
		indent,
	)
}
