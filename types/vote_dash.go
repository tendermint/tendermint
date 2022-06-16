package types

import tmproto "github.com/tendermint/tendermint/proto/tendermint/types"

// PopulateSignsFromProto updates the signatures of the current Vote with values are taken from the Vote's protobuf
func (vote *Vote) PopulateSignsFromProto(pv *tmproto.Vote) error {
	vote.BlockSignature = pv.BlockSignature
	vote.StateSignature = pv.StateSignature
	return vote.VoteExtensions.CopySignsFromProto(pv.VoteExtensionsToMap())
}

// PopulateSignsToProto updates the signatures of the given protobuf Vote entity with values are taken from the current Vote's
func (vote *Vote) PopulateSignsToProto(pv *tmproto.Vote) error {
	pv.BlockSignature = vote.BlockSignature
	pv.StateSignature = vote.StateSignature
	return vote.VoteExtensions.CopySignsToProto(pv.VoteExtensionsToMap())
}

// GetVoteExtensionsSigns returns the list of signatures for given vote-extension type
func (vote *Vote) GetVoteExtensionsSigns(extType tmproto.VoteExtensionType) [][]byte {
	if vote.VoteExtensions == nil {
		return nil
	}
	sigs := make([][]byte, len(vote.VoteExtensions[extType]))
	for i, ext := range vote.VoteExtensions[extType] {
		sigs[i] = ext.Signature
	}
	return sigs
}
