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
	Hash        cmn.HexBytes
	PartsHeader CanonicalPartSetHeader
}

type CanonicalPartSetHeader struct {
	Hash  cmn.HexBytes
	Total int
}

type CanonicalProposal struct {
	Version          uint64 `binary:"fixed64"`
	Height           int64  `binary:"fixed64"`
	Round            int64  `binary:"fixed64"`
	POLRound         int64  `binary:"fixed64"`
	Type             byte
	Timestamp        time.Time
	BlockPartsHeader CanonicalPartSetHeader
	POLBlockID       CanonicalBlockID
	ChainID          string
}

type CanonicalVote struct {
	Version   uint64 `binary:"fixed64"`
	Height    int64  `binary:"fixed64"`
	Round     int64  `binary:"fixed64"`
	VoteType  byte
	Timestamp time.Time
	BlockID   CanonicalBlockID
	ChainID   string
}

type CanonicalHeartbeat struct {
	Version          uint64 `binary:"fixed64"`
	Height           int64  `binary:"fixed64"`
	Round            int    `binary:"fixed64"`
	Sequence         int    `binary:"fixed64"`
	ValidatorAddress Address
	ValidatorIndex   int
	ChainID          string
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
		Type:             byte(ProposalType),
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
