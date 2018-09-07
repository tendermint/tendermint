package types

import (
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	tmtime "github.com/tendermint/tendermint/types/time"
)

// Canonical json is amino's json for structs with fields in alphabetical order

// TimeFormat is used for generating the sigs
const TimeFormat = time.RFC3339Nano

type CanonicalJSONBlockID struct {
	Hash        cmn.HexBytes               `json:"hash,omitempty"`
	PartsHeader CanonicalJSONPartSetHeader `json:"parts,omitempty"`
}

type CanonicalJSONPartSetHeader struct {
	Hash  cmn.HexBytes `json:"hash,omitempty"`
	Total int          `json:"total,omitempty"`
}

type CanonicalJSONProposal struct {
	ChainID          string                     `json:"@chain_id"`
	Type             string                     `json:"@type"`
	BlockPartsHeader CanonicalJSONPartSetHeader `json:"block_parts_header"`
	Height           int64                      `json:"height"`
	POLBlockID       CanonicalJSONBlockID       `json:"pol_block_id"`
	POLRound         int                        `json:"pol_round"`
	Round            int                        `json:"round"`
	Timestamp        string                     `json:"timestamp"`
}

type CanonicalJSONVote struct {
	ChainID   string               `json:"@chain_id"`
	Type      string               `json:"@type"`
	BlockID   CanonicalJSONBlockID `json:"block_id"`
	Height    int64                `json:"height"`
	Round     int                  `json:"round"`
	Timestamp string               `json:"timestamp"`
	VoteType  byte                 `json:"type"`
}

type CanonicalJSONHeartbeat struct {
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

func CanonicalBlockID(blockID BlockID) CanonicalJSONBlockID {
	return CanonicalJSONBlockID{
		Hash:        blockID.Hash,
		PartsHeader: CanonicalPartSetHeader(blockID.PartsHeader),
	}
}

func CanonicalPartSetHeader(psh PartSetHeader) CanonicalJSONPartSetHeader {
	return CanonicalJSONPartSetHeader{
		psh.Hash,
		psh.Total,
	}
}

func CanonicalProposal(chainID string, proposal *Proposal) CanonicalJSONProposal {
	return CanonicalJSONProposal{
		ChainID:          chainID,
		Type:             "proposal",
		BlockPartsHeader: CanonicalPartSetHeader(proposal.BlockPartsHeader),
		Height:           proposal.Height,
		Timestamp:        CanonicalTime(proposal.Timestamp),
		POLBlockID:       CanonicalBlockID(proposal.POLBlockID),
		POLRound:         proposal.POLRound,
		Round:            proposal.Round,
	}
}

func CanonicalVote(chainID string, vote *Vote) CanonicalJSONVote {
	return CanonicalJSONVote{
		ChainID:   chainID,
		Type:      "vote",
		BlockID:   CanonicalBlockID(vote.BlockID),
		Height:    vote.Height,
		Round:     vote.Round,
		Timestamp: CanonicalTime(vote.Timestamp),
		VoteType:  vote.Type,
	}
}

func CanonicalHeartbeat(chainID string, heartbeat *Heartbeat) CanonicalJSONHeartbeat {
	return CanonicalJSONHeartbeat{
		ChainID:          chainID,
		Type:             "heartbeat",
		Height:           heartbeat.Height,
		Round:            heartbeat.Round,
		Sequence:         heartbeat.Sequence,
		ValidatorAddress: heartbeat.ValidatorAddress,
		ValidatorIndex:   heartbeat.ValidatorIndex,
	}
}

func CanonicalTime(t time.Time) string {
	// Note that sending time over amino resets it to
	// local time, we need to force UTC here, so the
	// signatures match
	return tmtime.Canonical(t).Format(TimeFormat)
}
