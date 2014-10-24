package consensus

import (
	. "github.com/tendermint/tendermint/blocks"
	db_ "github.com/tendermint/tendermint/db"
	"github.com/tendermint/tendermint/state"
)

//-----------------------------------------------------------------------------

type PrivValidator struct {
	db db_.DB
	state.PrivAccount
}

func NewPrivValidator(db db_.DB, priv *state.PrivAccount) *PrivValidator {
	return &PrivValidator{db, *priv}
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
