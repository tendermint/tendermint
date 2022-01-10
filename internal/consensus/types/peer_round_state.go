package types

import (
	"fmt"
	"time"

	"github.com/tendermint/tendermint/libs/bits"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------------------

// PeerRoundState contains the known state of a peer.
// NOTE: Read-only when returned by PeerState.GetRoundState().
type PeerRoundState struct {
	Height int64         `json:"height,string"` // Height peer is at
	Round  int32         `json:"round"`         // Round peer is at, -1 if unknown.
	Step   RoundStepType `json:"step"`          // Step peer is at

	// Estimated start of round 0 at this height
	StartTime time.Time `json:"start_time"`

	// True if peer has proposal for this round
	Proposal                   bool                `json:"proposal"`
	ProposalBlockPartSetHeader types.PartSetHeader `json:"proposal_block_part_set_header"`
	ProposalBlockParts         *bits.BitArray      `json:"proposal_block_parts"`
	// Proposal's POL round. -1 if none.
	ProposalPOLRound int32 `json:"proposal_pol_round"`

	// nil until ProposalPOLMessage received.
	ProposalPOL     *bits.BitArray `json:"proposal_pol"`
	Prevotes        *bits.BitArray `json:"prevotes"`          // All votes peer has for this round
	Precommits      *bits.BitArray `json:"precommits"`        // All precommits peer has for this round
	LastCommitRound int32          `json:"last_commit_round"` // Round of commit for last height. -1 if none.
	LastCommit      *bits.BitArray `json:"last_commit"`       // All commit precommits of commit for last height.

	// Round that we have commit for. Not necessarily unique. -1 if none.
	CatchupCommitRound int32 `json:"catchup_commit_round"`

	// All commit precommits peer has for this height & CatchupCommitRound
	CatchupCommit *bits.BitArray `json:"catchup_commit"`
}

// String returns a string representation of the PeerRoundState
func (prs PeerRoundState) String() string {
	return prs.StringIndented("")
}

// Copy provides a deep copy operation. Because many of the fields in
// the PeerRound struct are pointers, we need an explicit deep copy
// operation to avoid a non-obvious shared data situation.
func (prs PeerRoundState) Copy() PeerRoundState {
	// this works because it's not a pointer receiver so it's
	// already, effectively a copy.

	headerHash := prs.ProposalBlockPartSetHeader.Hash.Bytes()

	hashCopy := make([]byte, len(headerHash))
	copy(hashCopy, headerHash)
	prs.ProposalBlockPartSetHeader = types.PartSetHeader{
		Total: prs.ProposalBlockPartSetHeader.Total,
		Hash:  hashCopy,
	}
	prs.ProposalBlockParts = prs.ProposalBlockParts.Copy()
	prs.ProposalPOL = prs.ProposalPOL.Copy()
	prs.Prevotes = prs.Prevotes.Copy()
	prs.Precommits = prs.Precommits.Copy()
	prs.LastCommit = prs.LastCommit.Copy()
	prs.CatchupCommit = prs.CatchupCommit.Copy()

	return prs
}

// StringIndented returns a string representation of the PeerRoundState
func (prs PeerRoundState) StringIndented(indent string) string {
	return fmt.Sprintf(`PeerRoundState{
%s  %v/%v/%v @%v
%s  Proposal %v -> %v
%s  POL      %v (round %v)
%s  Prevotes   %v
%s  Precommits %v
%s  LastCommit %v (round %v)
%s  Catchup    %v (round %v)
%s}`,
		indent, prs.Height, prs.Round, prs.Step, prs.StartTime,
		indent, prs.ProposalBlockPartSetHeader, prs.ProposalBlockParts,
		indent, prs.ProposalPOL, prs.ProposalPOLRound,
		indent, prs.Prevotes,
		indent, prs.Precommits,
		indent, prs.LastCommit, prs.LastCommitRound,
		indent, prs.CatchupCommit, prs.CatchupCommitRound,
		indent)
}
