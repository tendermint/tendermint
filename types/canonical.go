package types

import (
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	tmtime "github.com/tendermint/tendermint/types/time"
)

// Canonical* wraps the structs in types for amino encoding them for use in SignBytes / the Signable interface.

// TimeFormat is used for generating the sigs
const TimeFormat = time.RFC3339Nano

type CanonicalBlockID struct {
	Hash        cmn.HexBytes           `json:"hash,omitempty"`
	PartsHeader CanonicalPartSetHeader `json:"parts,omitempty"`
}

type CanonicalPartSetHeader struct {
	Hash  cmn.HexBytes `json:"hash,omitempty"`
	Total int          `json:"total,omitempty"`
}

type CanonicalProposal struct {
	Version          uint64                 `json:"version" binary:"fixed64"`
	Height           int64                  `json:"height" binary:"fixed64"`
	Round            int64                  `json:"round" binary:"fixed64"`
	POLRound         int64                  `json:"pol_round" binary:"fixed64"`
	Timestamp        time.Time              `json:"timestamp"`
	ChainID          string                 `json:"@chain_id"`
	BlockPartsHeader CanonicalPartSetHeader `json:"block_parts_header"`
	POLBlockID       CanonicalBlockID       `json:"pol_block_id"`
}

type CanonicalVote struct {
	Version   uint64           `json:"version" binary:"fixed64"`
	Height    int64            `json:"height" binary:"fixed64"`
	Round     int64            `json:"round"  binary:"fixed64"`
	VoteType  byte             `json:"type"`
	Timestamp time.Time        `json:"timestamp"`
	BlockID   CanonicalBlockID `json:"block_id"`
	ChainID   string           `json:"@chain_id"`
}

type CanonicalHeartbeat struct {
	Version          uint64  `json:"version" binary:"fixed64"`
	Height           int64   `json:"height" binary:"fixed64"`
	Round            int     `json:"height" binary:"fixed64"`
	Sequence         int     `json:"sequence" binary:"fixed64"`
	ChainID          string  `json:"@chain_id"`
	ValidatorAddress Address `json:"validator_address"`
	ValidatorIndex   int     `json:"validator_index"`
}

//-----------------------------------
// Canonicalize the structs

func CanonicalizeBlockID(blockID BlockID) CanonicalBlockID {
	return CanonicalBlockID{
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
		Height:           proposal.Height,
		Round:            int64(proposal.Round), // cast int->int64 to make amino encode it fixed64 (does not work for int)
		POLRound:         int64(proposal.POLRound),
		Timestamp:        proposal.Timestamp,
		ChainID:          chainID,
		BlockPartsHeader: CanonicalizePartSetHeader(proposal.BlockPartsHeader),
		POLBlockID:       CanonicalizeBlockID(proposal.POLBlockID),
	}
}

func CanonicalizeVote(chainID string, vote *Vote) CanonicalVote {
	return CanonicalVote{
		Height:    vote.Height,
		Round:     int64(vote.Round), // cast int->int64 to make amino encode it fixed64 (does not work for int)
		VoteType:  vote.Type,
		Timestamp: vote.Timestamp,
		BlockID:   CanonicalizeBlockID(vote.BlockID),
		ChainID:   chainID,
	}
}

func CanonicalizeHeartbeat(chainID string, heartbeat *Heartbeat) CanonicalHeartbeat {
	return CanonicalHeartbeat{
		Height:           heartbeat.Height,
		Round:            heartbeat.Round,
		Sequence:         heartbeat.Sequence,
		ChainID:          chainID,
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
