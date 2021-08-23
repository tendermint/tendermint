package types

import (
	"time"

	tmtime "github.com/tendermint/tendermint/libs/time"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
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
	if rbid == nil || rbid.IsNil() {
		cbid = nil
	} else {
		cbid = &tmproto.CanonicalBlockID{
			Hash:          bid.Hash,
			PartSetHeader: CanonicalizePartSetHeader(bid.PartSetHeader),
		}
	}

	return cbid
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

func GetVoteExtensionToSign(ext *tmproto.VoteExtension) *tmproto.VoteExtensionToSign {
	if ext == nil {
		return nil
	}
	return &tmproto.VoteExtensionToSign{
		AppDataToSign: ext.AppDataToSign,
	}
}

// CanonicalizeVote transforms the given Vote to a CanonicalVote, which does
// not contain ValidatorIndex and ValidatorAddress fields.
func CanonicalizeVote(chainID string, vote *tmproto.Vote) tmproto.CanonicalVote {
	return tmproto.CanonicalVote{
		Type:          vote.Type,
		Height:        vote.Height,       // encoded as sfixed64
		Round:         int64(vote.Round), // encoded as sfixed64
		BlockID:       CanonicalizeBlockID(vote.BlockID),
		Timestamp:     vote.Timestamp,
		ChainID:       chainID,
		VoteExtension: GetVoteExtensionToSign(vote.VoteExtension),
	}
}

// CanonicalTime can be used to stringify time in a canonical way.
func CanonicalTime(t time.Time) string {
	// Note that sending time over amino resets it to
	// local time, we need to force UTC here, so the
	// signatures match
	return tmtime.Canonical(t).Format(TimeFormat)
}
