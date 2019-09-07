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
	ProposalPOLRound         int                 `json:"proposal_pol_round"`          // Proposal's POL round. -1 if none.
	LastCommitRound          int                 `json:"last_commit_round"`           // Round of commit for last height. -1 if none.
	CatchupCommitRound       int                 `json:"catchup_commit_round"`        // Round that we have commit for. Not necessarily unique. -1 if none.
	ProposalBlockParts       *cmn.BitArray       `json:"proposal_block_parts"`        //
	ProposalPOL              *cmn.BitArray       `json:"proposal_pol"`                // nil until ProposalPOLMessage received.
	Prevotes                 *cmn.BitArray       `json:"prevotes"`                    // All votes peer has for this round
	Precommits               *cmn.BitArray       `json:"precommits"`                  // All precommits peer has for this round
	LastCommit               *cmn.BitArray       `json:"last_commit"`                 // All commit precommits of commit for last height.
	CatchupCommit            *cmn.BitArray       `json:"catchup_commit"`              // All commit precommits peer has for this height & CatchupCommitRound
	ProposalBlockPartsHeader types.PartSetHeader `json:"proposal_block_parts_header"` //
	Proposal                 bool                `json:"proposal"`                    // True if peer has proposal for this round
	Step                     RoundStepType       `json:"step"`                        // Step peer is at
	StartTime                time.Time           `json:"start_time"`                  // Estimated start of round 0 at this height
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

//-----------------------------------------------------------
// These methods are for Protobuf Compatibility

// Size returns the size of the amino encoding, in bytes.
func (ps *PeerRoundState) Size() int {
	bs, _ := ps.Marshal()
	return len(bs)
}

// Marshal returns the amino encoding.
func (ps *PeerRoundState) Marshal() ([]byte, error) {
	return cdc.MarshalBinaryBare(ps)
}

// MarshalTo calls Marshal and copies to the given buffer.
func (ps *PeerRoundState) MarshalTo(data []byte) (int, error) {
	bs, err := ps.Marshal()
	if err != nil {
		return -1, err
	}
	return copy(data, bs), nil
}

// Unmarshal deserializes from amino encoded form.
func (ps *PeerRoundState) Unmarshal(bs []byte) error {
	return cdc.UnmarshalBinaryBare(bs, ps)
}
