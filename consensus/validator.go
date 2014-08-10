package consensus

import (
	"io"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	//. "github.com/tendermint/tendermint/common"
	db_ "github.com/tendermint/tendermint/db"
)

// Holds state for a Validator at a given height+round.
// Meant to be discarded every round of the consensus protocol.
type Validator struct {
	Account
	BondHeight  UInt32
	VotingPower UInt64
	Accum       Int64
}

// Used to persist the state of ConsensusStateControl.
func ReadValidator(r io.Reader) *Validator {
	return &Validator{
		Account: Account{
			Id:     ReadUInt64(r),
			PubKey: ReadByteSlice(r),
		},
		BondHeight:  ReadUInt32(r),
		VotingPower: ReadUInt64(r),
		Accum:       ReadInt64(r),
	}
}

// Creates a new copy of the validator so we can mutate accum.
func (v *Validator) Copy() *Validator {
	return &Validator{
		Account:     v.Account,
		BondHeight:  v.BondHeight,
		VotingPower: v.VotingPower,
		Accum:       v.Accum,
	}
}

// Used to persist the state of ConsensusStateControl.
func (v *Validator) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(&v.Id, w, n, err)
	n, err = WriteTo(&v.PubKey, w, n, err)
	n, err = WriteTo(&v.BondHeight, w, n, err)
	n, err = WriteTo(&v.VotingPower, w, n, err)
	n, err = WriteTo(&v.Accum, w, n, err)
	return
}

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
