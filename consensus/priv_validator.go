package consensus

import (
	. "github.com/tendermint/tendermint/blocks"
	db_ "github.com/tendermint/tendermint/db"
	"github.com/tendermint/tendermint/state"
)

//-----------------------------------------------------------------------------

type PrivValidator struct {
	state.PrivAccount
	db *db_.LevelDB
}

// Double signing results in a panic.
func (pv *PrivValidator) Sign(o Signable) {
	switch o.(type) {
	case *Proposal:
		//TODO: prevent double signing && test.
		pv.PrivAccount.Sign(o.(*Proposal))
	case *Vote:
		//TODO: prevent double signing && test.
		pv.PrivAccount.Sign(o.(*Vote))
	}
}
