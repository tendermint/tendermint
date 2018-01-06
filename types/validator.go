package types

import (
	"bytes"
	"fmt"
	"io"

	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/go-wire/data"
	cmn "github.com/tendermint/tmlibs/common"
)

// Volatile state for each Validator
// NOTE: The Accum is not included in Validator.Hash();
// make sure to update that method if changes are made here
type Validator struct {
	Address     data.Bytes    `json:"address"`
	PubKey      crypto.PubKey `json:"pub_key"`
	VotingPower int64         `json:"voting_power"`

	Accum int64 `json:"accum"`
}

func NewValidator(pubKey crypto.PubKey, votingPower int64) *Validator {
	return &Validator{
		Address:     pubKey.Address(),
		PubKey:      pubKey,
		VotingPower: votingPower,
		Accum:       0,
	}
}

// Creates a new copy of the validator so we can mutate accum.
// Panics if the validator is nil.
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
			cmn.PanicSanity("Cannot compare identical validators")
			return nil
		}
	}
}

func (v *Validator) String() string {
	if v == nil {
		return "nil-Validator"
	}
	return fmt.Sprintf("Validator{%v %v VP:%v A:%v}",
		v.Address,
		v.PubKey,
		v.VotingPower,
		v.Accum)
}

// Hash computes the unique ID of a validator with a given voting power.
// It excludes the Accum value, which changes with every round.
func (v *Validator) Hash() []byte {
	return wire.BinaryRipemd160(struct {
		Address     data.Bytes
		PubKey      crypto.PubKey
		VotingPower int64
	}{
		v.Address,
		v.PubKey,
		v.VotingPower,
	})
}

//-------------------------------------

var ValidatorCodec = validatorCodec{}

type validatorCodec struct{}

func (vc validatorCodec) Encode(o interface{}, w io.Writer, n *int, err *error) {
	wire.WriteBinary(o.(*Validator), w, n, err)
}

func (vc validatorCodec) Decode(r io.Reader, n *int, err *error) interface{} {
	return wire.ReadBinary(&Validator{}, r, 0, n, err)
}

func (vc validatorCodec) Compare(o1 interface{}, o2 interface{}) int {
	cmn.PanicSanity("ValidatorCodec.Compare not implemented")
	return 0
}

//--------------------------------------------------------------------------------
// For testing...

// RandValidator returns a randomized validator, useful for testing.
// UNSTABLE
func RandValidator(randPower bool, minPower int64) (*Validator, TestSigner) {
	privVal := GenSigner()
	votePower := minPower
	if randPower {
		votePower += int64(cmn.RandUint32())
	}
	val := NewValidator(privVal.PubKey(), votePower)
	return val, privVal
}

type TestSigner interface {
	Address() data.Bytes
	PubKey() crypto.PubKey
	Sign([]byte) (crypto.Signature, error)
}

func GenSigner() TestSigner {
	return &DefaultTestSigner{
		crypto.GenPrivKeyEd25519().Wrap(),
	}
}

type DefaultTestSigner struct {
	crypto.PrivKey
}

func (ds *DefaultTestSigner) Address() data.Bytes {
	return ds.PubKey().Address()
}

func (ds *DefaultTestSigner) PubKey() crypto.PubKey {
	return ds.PrivKey.PubKey()
}

func (ds *DefaultTestSigner) Sign(msg []byte) (crypto.Signature, error) {
	return ds.PrivKey.Sign(msg), nil
}

//-------------------------------------------------------------

type PrivValidatorsByAddress []TestSigner

func (pvs PrivValidatorsByAddress) Len() int {
	return len(pvs)
}

func (pvs PrivValidatorsByAddress) Less(i, j int) bool {
	return bytes.Compare(pvs[i].Address(), pvs[j].Address()) == -1
}

func (pvs PrivValidatorsByAddress) Swap(i, j int) {
	it := pvs[i]
	pvs[i] = pvs[j]
	pvs[j] = it
}

//---------------------------------------------------------------------
