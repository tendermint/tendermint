package consensus

import (
	db_ "github.com/tendermint/tendermint/db"
	. "github.com/tendermint/tendermint/state"
)

//-----------------------------------------------------------------------------

// TODO: Ensure that double signing never happens via an external persistent check.
type PrivValidator struct {
	PrivAccount
	db *db_.LevelDB
}

// Modifies the vote object in memory.
// Double signing results in an error.
func (pv *PrivValidator) SignVote(vote *Vote) error {
	return nil
}
