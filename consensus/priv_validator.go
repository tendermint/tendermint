package consensus

import (
	. "github.com/tendermint/tendermint/blocks"
	db_ "github.com/tendermint/tendermint/db"
	. "github.com/tendermint/tendermint/state"
)

//-----------------------------------------------------------------------------

type PrivValidator struct {
	PrivAccount
	db *db_.LevelDB
}

// Double signing results in an error.
func (pv *PrivValidator) SignProposal(proposal *Proposal) {
	//TODO: prevent double signing.
	doc := GenProposalDocument(proposal.Height, proposal.Round, proposal.BlockPartsTotal,
		proposal.BlockPartsHash, proposal.POLPartsTotal, proposal.POLPartsHash)
	proposal.Signature = pv.Sign([]byte(doc))
}

// Double signing results in an error.
func (pv *PrivValidator) SignVote(vote *Vote) {
	//TODO: prevent double signing.
	doc := GenVoteDocument(vote.Type, vote.Height, vote.Round, vote.BlockHash)
	vote.Signature = pv.Sign([]byte(doc))
}
