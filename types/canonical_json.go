package types

import (
	"time"

	wire "github.com/tendermint/tendermint/wire"
	cmn "github.com/tendermint/tmlibs/common"
)

// canonical json is wire's json for structs with fields in alphabetical order

// TimeFormat is used for generating the sigs
const TimeFormat = wire.RFC3339Millis

type CanonicalJSONBlockID struct {
	Hash        cmn.HexBytes               `json:"hash,omitempty"`
	PartsHeader CanonicalJSONPartSetHeader `json:"parts,omitempty"`
}

type CanonicalJSONPartSetHeader struct {
	Hash  cmn.HexBytes `json:"hash"`
	Total int          `json:"total"`
}

type CanonicalJSONProposal struct {
	BlockPartsHeader CanonicalJSONPartSetHeader `json:"block_parts_header"`
	Height           int64                      `json:"height"`
	POLBlockID       CanonicalJSONBlockID       `json:"pol_block_id"`
	POLRound         int                        `json:"pol_round"`
	Round            int                        `json:"round"`
	Timestamp        string                     `json:"timestamp"`
}

type CanonicalJSONVote struct {
	BlockID   CanonicalJSONBlockID `json:"block_id"`
	Height    int64                `json:"height"`
	Round     int                  `json:"round"`
	Timestamp string               `json:"timestamp"`
	Type      byte                 `json:"type"`
}

type CanonicalJSONHeartbeat struct {
	Height           int64   `json:"height"`
	Round            int     `json:"round"`
	Sequence         int     `json:"sequence"`
	ValidatorAddress Address `json:"validator_address"`
	ValidatorIndex   int     `json:"validator_index"`
}

//------------------------------------
// Messages including a "chain id" can only be applied to one chain, hence "Once"

type CanonicalJSONOnceProposal struct {
	ChainID  string                `json:"chain_id"`
	Proposal CanonicalJSONProposal `json:"proposal"`
}

type CanonicalJSONOnceVote struct {
	ChainID string            `json:"chain_id"`
	Vote    CanonicalJSONVote `json:"vote"`
}

type CanonicalJSONOnceHeartbeat struct {
	ChainID   string                 `json:"chain_id"`
	Heartbeat CanonicalJSONHeartbeat `json:"heartbeat"`
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

func CanonicalProposal(proposal *Proposal) CanonicalJSONProposal {
	return CanonicalJSONProposal{
		BlockPartsHeader: CanonicalPartSetHeader(proposal.BlockPartsHeader),
		Height:           proposal.Height,
		Timestamp:        CanonicalTime(proposal.Timestamp),
		POLBlockID:       CanonicalBlockID(proposal.POLBlockID),
		POLRound:         proposal.POLRound,
		Round:            proposal.Round,
	}
}

func CanonicalVote(vote *Vote) CanonicalJSONVote {
	return CanonicalJSONVote{
		BlockID:   CanonicalBlockID(vote.BlockID),
		Height:    vote.Height,
		Round:     vote.Round,
		Timestamp: CanonicalTime(vote.Timestamp),
		Type:      vote.Type,
	}
}

func CanonicalHeartbeat(heartbeat *Heartbeat) CanonicalJSONHeartbeat {
	return CanonicalJSONHeartbeat{
		heartbeat.Height,
		heartbeat.Round,
		heartbeat.Sequence,
		heartbeat.ValidatorAddress,
		heartbeat.ValidatorIndex,
	}
}

func CanonicalTime(t time.Time) string {
	// note that sending time over wire resets it to
	// local time, we need to force UTC here, so the
	// signatures match
	return t.UTC().Format(TimeFormat)
}
