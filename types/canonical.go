package types

import (
	"time"

	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

// Canonical* wraps the structs in types for amino encoding them for use in SignBytes / the Signable interface.

// TimeFormat is used for generating the sigs
const TimeFormat = time.RFC3339Nano

//-----------------------------------
// Canonicalize the structs

func CanonicalizeBlockID(bid tmproto.BlockID) *tmproto.CanonicalBlockID {
	rbid, err := BlockIDFromProto(&bid)
	if err != nil {
		panic(err)
	}
	var cbid *tmproto.CanonicalBlockID
	if rbid == nil || rbid.IsZero() {
		cbid = nil
	} else {
		cbid = &tmproto.CanonicalBlockID{
			Hash:          bid.Hash,
			PartSetHeader: CanonicalizePartSetHeader(bid.PartSetHeader),
		}
	}

	return cbid
}

func CanonicalizeStateID(sid tmproto.StateID) *tmproto.CanonicalStateID {
	rsid, err := StateIDFromProto(&sid)
	if err != nil {
		panic(err)
	}
	var csid *tmproto.CanonicalStateID
	if rsid == nil || rsid.IsZero() {
		csid = nil
	} else {
		csid = &tmproto.CanonicalStateID{
			LastAppHash: sid.LastAppHash,
		}
	}

	return csid
}

// CanonicalizeVote transforms the given PartSetHeader to a CanonicalPartSetHeader.
func CanonicalizePartSetHeader(psh tmproto.PartSetHeader) tmproto.CanonicalPartSetHeader {
	return tmproto.CanonicalPartSetHeader(psh)
}

// CanonicalizeVote transforms the given Proposal to a CanonicalProposal.
func CanonicalizeProposal(chainID string, proposal *tmproto.Proposal) tmproto.CanonicalProposal {
	return tmproto.CanonicalProposal{
		Type:      tmproto.ProposalType,
		Height:    proposal.Height,       // encoded as sfixed64
		Round:     int64(proposal.Round), // encoded as sfixed64
		POLRound:  int64(proposal.PolRound),
		BlockID:   CanonicalizeBlockID(proposal.BlockID),
		Timestamp: proposal.Timestamp,
		ChainID:   chainID,
	}
}

// CanonicalizeVote transforms the given Vote to a CanonicalVote, which does
// not contain ValidatorIndex and ValidatorProTxHash fields.
func CanonicalizeVote(chainID string, vote *tmproto.Vote) tmproto.CanonicalVote {
	return tmproto.CanonicalVote{
		Type:    vote.Type,
		Height:  vote.Height,       // encoded as sfixed64
		Round:   int64(vote.Round), // encoded as sfixed64
		BlockID: CanonicalizeBlockID(vote.BlockID),
		StateID: CanonicalizeStateID(vote.StateID),
		ChainID: chainID,
	}
}

// CanonicalizeVote transforms the given Vote to a CanonicalVote, which does
// not contain ValidatorIndex and ValidatorProTxHash fields.
func CanonicalizeStateVote(vote *tmproto.Vote) tmproto.CanonicalStateVote {
	return tmproto.CanonicalStateVote{
		Height:  vote.Height, // encoded as sfixed64
		StateID: CanonicalizeStateID(vote.StateID),
	}
}

// CanonicalTime can be used to stringify time in a canonical way.
func CanonicalTime(t time.Time) string {
	// Note that sending time over amino resets it to
	// local time, we need to force UTC here, so the
	// signatures match
	return tmtime.Canonical(t).Format(TimeFormat)
}
