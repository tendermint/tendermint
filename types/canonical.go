package types

import (
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	tmtime "github.com/tendermint/tendermint/types/time"
)

// Canonical* wraps the structs in types for amino encoding the them for use in SignBytes / the Signable interface.

// TimeFormat is used for generating the sigs
const TimeFormat = time.RFC3339Nano

type CanonicallockID struct {
	Hash        cmn.HexBytes           `json:"hash,omitempty"`
	PartsHeader CanonicalPartSetHeader `json:"parts,omitempty"`
}

type CanonicalPartSetHeader struct {
	Hash  cmn.HexBytes `json:"hash,omitempty"`
	Total int          `json:"total,omitempty"`
}

type CanonicalProposal struct {
	ChainID          string                 `json:"@chain_id"`
	Type             string                 `json:"@type"`
	BlockPartsHeader CanonicalPartSetHeader `json:"block_parts_header"`
	Height           int64                  `json:"height"`
	POLBlockID       CanonicallockID        `json:"pol_block_id"`
	POLRound         int                    `json:"pol_round"`
	Round            int                    `json:"round"`
	Timestamp        time.Time              `json:"timestamp"`
}

type CanonicalVote struct {
	ChainID   string          `json:"@chain_id"`
	Type      string          `json:"@type"`
	BlockID   CanonicallockID `json:"block_id"`
	Height    int64           `json:"height"`
	Round     int             `json:"round"`
	Timestamp time.Time       `json:"timestamp"`
	VoteType  byte            `json:"type"`
}

type CanonicalHeartbeat struct {
	ChainID          string  `json:"@chain_id"`
	Type             string  `json:"@type"`
	Height           int64   `json:"height"`
	Round            int     `json:"round"`
	Sequence         int     `json:"sequence"`
	ValidatorAddress Address `json:"validator_address"`
	ValidatorIndex   int     `json:"validator_index"`
}

//-----------------------------------
// Canonicalize the structs

func CanonicalizeBlockID(blockID BlockID) CanonicallockID {
	return CanonicallockID{
		Hash:        blockID.Hash,
		PartsHeader: CanonicalizePartSetHeader(blockID.PartsHeader),
	}
}

func CanonicalizePartSetHeader(psh PartSetHeader) CanonicalPartSetHeader {
	return CanonicalPartSetHeader{
		psh.Hash,
		psh.Total,
	}
}

func CanonicalizeProposal(chainID string, proposal *Proposal) CanonicalProposal {
	return CanonicalProposal{
		ChainID:          chainID,
		Type:             "proposal",
		BlockPartsHeader: CanonicalizePartSetHeader(proposal.BlockPartsHeader),
		Height:           proposal.Height,
		Timestamp:        proposal.Timestamp,
		POLBlockID:       CanonicalizeBlockID(proposal.POLBlockID),
		POLRound:         proposal.POLRound,
		Round:            proposal.Round,
	}
}

func CanonicalizeVote(chainID string, vote *Vote) CanonicalVote {
	return CanonicalVote{
		ChainID:   chainID,
		Type:      "vote",
		BlockID:   CanonicalizeBlockID(vote.BlockID),
		Height:    vote.Height,
		Round:     vote.Round,
		Timestamp: vote.Timestamp,
		VoteType:  vote.Type,
	}
}

func CanonicalizeHeartbeat(chainID string, heartbeat *Heartbeat) CanonicalHeartbeat {
	return CanonicalHeartbeat{
		ChainID:          chainID,
		Type:             "heartbeat",
		Height:           heartbeat.Height,
		Round:            heartbeat.Round,
		Sequence:         heartbeat.Sequence,
		ValidatorAddress: heartbeat.ValidatorAddress,
		ValidatorIndex:   heartbeat.ValidatorIndex,
	}
}

// CanonicalTime can be used to stringify time in a canonical way.
func CanonicalTime(t time.Time) string {
	// Note that sending time over amino resets it to
	// local time, we need to force UTC here, so the
	// signatures match
	return tmtime.Canonical(t).Format(TimeFormat)
}
