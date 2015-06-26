package state

import (
	"bytes"
	"fmt"
	"io"

	"github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	"github.com/tendermint/tendermint/types"
)

// Persistent (mostly) static data for each Validator
type ValidatorInfo struct {
	Address         []byte                `json:"address"`
	PubKey          account.PubKeyEd25519 `json:"pub_key"`
	UnbondTo        []*types.TxOutput     `json:"unbond_to"`
	FirstBondHeight int                   `json:"first_bond_height"`
	FirstBondAmount int64                 `json:"first_bond_amount"`
	DestroyedHeight int                   `json:"destroyed_height"` // If destroyed
	DestroyedAmount int64                 `json:"destroyed_amount"` // If destroyed
	ReleasedHeight  int                   `json:"released_height"`  // If released
}

func (valInfo *ValidatorInfo) Copy() *ValidatorInfo {
	valInfoCopy := *valInfo
	return &valInfoCopy
}

func ValidatorInfoEncoder(o interface{}, w io.Writer, n *int64, err *error) {
	binary.WriteBinary(o.(*ValidatorInfo), w, n, err)
}

func ValidatorInfoDecoder(r io.Reader, n *int64, err *error) interface{} {
	return binary.ReadBinary(&ValidatorInfo{}, r, n, err)
}

var ValidatorInfoCodec = binary.Codec{
	Encode: ValidatorInfoEncoder,
	Decode: ValidatorInfoDecoder,
}

//-----------------------------------------------------------------------------

// Volatile state for each Validator
// Also persisted with the state, but fields change
// every height|round so they don't go in merkle.Tree
type Validator struct {
	Address          []byte                `json:"address"`
	PubKey           account.PubKeyEd25519 `json:"pub_key"`
	BondHeight       int                   `json:"bond_height"`
	UnbondHeight     int                   `json:"unbond_height"`
	LastCommitHeight int                   `json:"last_commit_height"`
	VotingPower      int64                 `json:"voting_power"`
	Accum            int64                 `json:"accum"`
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
	return binary.BinarySha256(v)
}

//-------------------------------------

var ValidatorCodec = validatorCodec{}

type validatorCodec struct{}

func (vc validatorCodec) Encode(o interface{}, w io.Writer, n *int64, err *error) {
	binary.WriteBinary(o.(*Validator), w, n, err)
}

func (vc validatorCodec) Decode(r io.Reader, n *int64, err *error) interface{} {
	return binary.ReadBinary(&Validator{}, r, n, err)
}

func (vc validatorCodec) Compare(o1 interface{}, o2 interface{}) int {
	panic("ValidatorCodec.Compare not implemented")
}
