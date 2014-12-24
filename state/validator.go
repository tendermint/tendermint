package state

import (
	"bytes"
	"fmt"
	"io"

	. "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/block"
)

// Persistent (mostly) static data for each Validator
type ValidatorInfo struct {
	Address         []byte
	PubKey          PubKeyEd25519
	UnbondTo        []*TxOutput
	FirstBondHeight uint
	FirstBondAmount uint64

	// If destroyed:
	DestroyedHeight uint
	DestroyedAmount uint64

	// If released:
	ReleasedHeight uint
}

func (valInfo *ValidatorInfo) Copy() *ValidatorInfo {
	valInfoCopy := *valInfo
	return &valInfoCopy
}

func ValidatorInfoEncoder(o interface{}, w io.Writer, n *int64, err *error) {
	WriteBinary(o.(*ValidatorInfo), w, n, err)
}

func ValidatorInfoDecoder(r io.Reader, n *int64, err *error) interface{} {
	return ReadBinary(&ValidatorInfo{}, r, n, err)
}

var ValidatorInfoCodec = Codec{
	Encode: ValidatorInfoEncoder,
	Decode: ValidatorInfoDecoder,
}

//-----------------------------------------------------------------------------

// Volatile state for each Validator
// Also persisted with the state, but fields change
// every height|round so they don't go in merkle.Tree
type Validator struct {
	Address          []byte
	PubKey           PubKeyEd25519
	BondHeight       uint
	UnbondHeight     uint
	LastCommitHeight uint
	VotingPower      uint64
	Accum            int64
}

// Creates a new copy of the validator so we can mutate accum.
func (v *Validator) Copy() *Validator {
	vCopy := *v
	return &vCopy
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
		if bytes.Compare(v.Address, other.Address) < 0 {
			return v
		} else if bytes.Compare(v.Address, other.Address) > 0 {
			return other
		} else {
			panic("Cannot compare identical validators")
		}
	}
}

func (v *Validator) String() string {
	return fmt.Sprintf("Validator{%X %v %v-%v-%v VP:%v A:%v}",
		v.Address,
		v.PubKey,
		v.BondHeight,
		v.LastCommitHeight,
		v.UnbondHeight,
		v.VotingPower,
		v.Accum)
}

func (v *Validator) Hash() []byte {
	return BinarySha256(v)
}

//-------------------------------------

var ValidatorCodec = validatorCodec{}

type validatorCodec struct{}

func (vc validatorCodec) Encode(o interface{}, w io.Writer, n *int64, err *error) {
	WriteBinary(o.(*Validator), w, n, err)
}

func (vc validatorCodec) Decode(r io.Reader, n *int64, err *error) interface{} {
	return ReadBinary(&Validator{}, r, n, err)
}

func (vc validatorCodec) Compare(o1 interface{}, o2 interface{}) int {
	panic("ValidatorCodec.Compare not implemented")
}
