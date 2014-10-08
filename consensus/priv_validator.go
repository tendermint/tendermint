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
	pv.SignSignable(proposal)
}

// Double signing results in an error.
func (pv *PrivValidator) SignVote(vote *Vote) {
	//TODO: prevent double signing.
	pv.SignSignable(vote)
}
