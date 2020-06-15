package types

import (
	"time"

	tmproto "github.com/tendermint/tendermint/proto/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

// Canonical* wraps the structs in types for amino encoding them for use in SignBytes / the Signable interface.

// TimeFormat is used for generating the sigs
const TimeFormat = time.RFC3339Nano

//-----------------------------------
// Canonicalize the structs

func CanonicalizeBlockID(bid tmproto.BlockID) *tmproto.CanonicalBlockID {
	rbid, _ := BlockIDFromProto(&bid)
	var cbid *tmproto.CanonicalBlockID
	if rbid == nil || rbid.IsZero() {
		cbid = nil
	} else {
		c := tmproto.CanonicalBlockID{
			Hash:        bid.Hash,
			PartsHeader: CanonicalizePartSetHeader(bid.PartsHeader),
		}
		cbid = &c
	}

	return cbid
}

func CanonicalizePartSetHeader(psh tmproto.PartSetHeader) tmproto.CanonicalPartSetHeader {
	return tmproto.CanonicalPartSetHeader(psh)
}

func CanonicalizeProposal(chainID string, proposal *tmproto.Proposal) tmproto.CanonicalProposal {
	return tmproto.CanonicalProposal{
		Type:      tmproto.ProposalType,
		Height:    proposal.Height,
		Round:     int64(proposal.Round),
		POLRound:  int64(proposal.PolRound),
		BlockID:   CanonicalizeBlockID(proposal.BlockID),
		Timestamp: proposal.Timestamp,
		ChainID:   chainID,
	}
}

func CanonicalizeVote(chainID string, vote *tmproto.Vote) tmproto.CanonicalVote {
	return tmproto.CanonicalVote{
		Type:      vote.Type,
		Height:    vote.Height,
		Round:     int64(vote.Round),
		BlockID:   CanonicalizeBlockID(vote.BlockID),
		Timestamp: vote.Timestamp,
		ChainID:   chainID,
	}
}

// CanonicalTime can be used to stringify time in a canonical way.
func CanonicalTime(t time.Time) string {
	// Note that sending time over amino resets it to
	// local time, we need to force UTC here, so the
	// signatures match
	return tmtime.Canonical(t).Format(TimeFormat)
}
