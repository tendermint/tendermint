package types

import (
	"fmt"
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------------------

// PeerRoundState contains the known state of a peer.
// NOTE: Read-only when returned by PeerState.GetRoundState().
type PeerRoundState struct {
	Height                   int64               `json:"height"`                      // Height peer is at
	Round                    int                 `json:"round"`                       // Round peer is at, -1 if unknown.
	Step                     RoundStepType       `json:"step"`                        // Step peer is at
	StartTime                time.Time           `json:"start_time"`                  // Estimated start of round 0 at this height
	Proposal                 bool                `json:"proposal"`                    // True if peer has proposal for this round
	ProposalBlockPartsHeader types.PartSetHeader `json:"proposal_block_parts_header"` //
	ProposalBlockParts       *cmn.BitArray       `json:"proposal_block_parts"`        //
	ProposalPOLRound         int                 `json:"proposal_pol_round"`          // Proposal's POL round. -1 if none.
	ProposalPOL              *cmn.BitArray       `json:"proposal_pol"`                // nil until ProposalPOLMessage received.
	Prevotes                 *cmn.BitArray       `json:"prevotes"`                    // All votes peer has for this round
	Precommits               *cmn.BitArray       `json:"precommits"`                  // All precommits peer has for this round
	LastCommitRound          int                 `json:"last_commit_round"`           // Round of commit for last height. -1 if none.
	LastCommit               *cmn.BitArray       `json:"last_commit"`                 // All commit precommits of commit for last height.
	CatchupCommitRound       int                 `json:"catchup_commit_round"`        // Round that we have commit for. Not necessarily unique. -1 if none.
	CatchupCommit            *cmn.BitArray       `json:"catchup_commit"`              // All commit precommits peer has for this height & CatchupCommitRound
}

// String returns a string representation of the PeerRoundState
func (prs PeerRoundState) String() string {
	return prs.StringIndented("")
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
		indent, prs.ProposalBlockPartsHeader, prs.ProposalBlockParts,
		indent, prs.ProposalPOL, prs.ProposalPOLRound,
		indent, prs.Prevotes,
		indent, prs.Precommits,
		indent, prs.LastCommit, prs.LastCommitRound,
		indent, prs.CatchupCommit, prs.CatchupCommitRound,
		indent)
}
