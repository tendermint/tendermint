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
	Height           int64                  `json:"height" binary:"fixed64"`
	Round            int64                  `json:"round" binary:"fixed64"`
	ChainID          string                 `json:"@chain_id"`
	BlockPartsHeader CanonicalPartSetHeader `json:"block_parts_header"`
	POLBlockID       CanonicalBlockID       `json:"pol_block_id"`
	POLRound         int                    `json:"pol_round"`
	Timestamp        time.Time              `json:"timestamp"`
}

type CanonicalVote struct {
	Height    int64            `amino:"write_empty" json:"height" binary:"fixed64"`
	Round     int64            `amino:"write_empty" json:"round"  binary:"fixed64"`
	VoteType  byte             `json:"type"`
	Timestamp time.Time        `json:"timestamp"`
	BlockID   CanonicalBlockID `json:"block_id"`
	ChainID   string           `json:"@chain_id"`
}

type CanonicalHeartbeat struct {
	Height           int64   `json:"height" binary:"fixed64"`
	Round            int     `json:"height" binary:"fixed64"`
	ChainID          string  `json:"@chain_id"`
	Type             string  `json:"@type"`
	Sequence         int     `json:"sequence"`
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
		ChainID:          chainID,
		BlockPartsHeader: CanonicalizePartSetHeader(proposal.BlockPartsHeader),
		Height:           proposal.Height,
		Timestamp:        proposal.Timestamp,
		POLBlockID:       CanonicalizeBlockID(proposal.POLBlockID),
		POLRound:         proposal.POLRound,
		Round:            int64(proposal.Round),
	}
}

func CanonicalizeVote(chainID string, vote *Vote) CanonicalVote {
	return CanonicalVote{
		ChainID: chainID,
		BlockID: CanonicalizeBlockID(vote.BlockID),
		Height:  vote.Height,
		// XXX make sure we don't cause any trouble by casting here; currently, amino doesn't support fixed size for int
		// XXX same for proposal etc
		Round:     int64(vote.Round),
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
