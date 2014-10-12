package state

import (
	"io"

	. "github.com/tendermint/tendermint/binary"
)

// Holds state for a Validator at a given height+round.
// Meant to be discarded every round of the consensus protocol.
// TODO consider moving this to another common types package.
type Validator struct {
	Account
	BondHeight  uint32 // TODO: is this needed?
	VotingPower uint64
	Accum       int64
}

// Used to persist the state of ConsensusStateControl.
func ReadValidator(r io.Reader, n *int64, err *error) *Validator {
	return &Validator{
		Account:     ReadAccount(r, n, err),
		BondHeight:  ReadUInt32(r, n, err),
		VotingPower: ReadUInt64(r, n, err),
		Accum:       ReadInt64(r, n, err),
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
	WriteBinary(w, v.Account, &n, &err)
	WriteUInt32(w, v.BondHeight, &n, &err)
	WriteUInt64(w, v.VotingPower, &n, &err)
	WriteInt64(w, v.Accum, &n, &err)
	return
}

// Returns the one with higher Accum.
func (v *Validator) CompareAccum(other *Validator) *Validator {
	if v == nil {
		return other
	}
	if v.Accum > other.Accum {
		return v
	} else if v.Accum < other.Accum {
		return other
	} else {
		if v.Id < other.Id {
			return v
		} else if v.Id > other.Id {
			return other
		} else {
			panic("Cannot compare identical validators")
		}
	}
}

//-------------------------------------

var ValidatorCodec = validatorCodec{}

type validatorCodec struct{}

func (vc validatorCodec) Encode(o interface{}, w io.Writer, n *int64, err *error) {
	WriteBinary(w, o.(*Validator), n, err)
}

func (vc validatorCodec) Decode(r io.Reader, n *int64, err *error) interface{} {
	return ReadValidator(r, n, err)
}

func (vc validatorCodec) Compare(o1 interface{}, o2 interface{}) int {
	panic("ValidatorCodec.Compare not implemented")
}
