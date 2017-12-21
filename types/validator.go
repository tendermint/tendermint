package types

import (
	"fmt"
	"io"
	"strings"

	crypto "github.com/libp2p/go-libp2p-crypto"
	lpeer "github.com/libp2p/go-libp2p-peer"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/go-wire/data"
	cmn "github.com/tendermint/tmlibs/common"
)

// Volatile state for each Validator
// NOTE: The Accum is not included in Validator.Hash();
// make sure to update that method if changes are made here
type Validator struct {
	Address     string `json:"address"`
	PubKey      string `json:"pub_key"`
	VotingPower int64  `json:"voting_power"`

	Accum int64 `json:"accum"`
}

func NewValidator(pubKey crypto.PubKey, votingPower int64) *Validator {
	pubKeyBytes, err := pubKey.Bytes()
	if err != nil {
		panic(err)
	}

	pubKeyStr, err := data.Encoder.Marshal(pubKeyBytes)
	if err != nil {
		panic(err)
	}

	peerId, err := lpeer.IDFromPublicKey(pubKey)
	if err != nil {
		panic(err)
	}

	return &Validator{
		Address:     peerId.Pretty(),
		PubKey:      string(pubKeyStr),
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

// ParsePubKey parses the validator public key.
func (v *Validator) ParsePubKey() (crypto.PubKey, error) {
	pubKeyStr := v.PubKey
	var dat []byte
	if err := data.Encoder.Unmarshal(&dat, []byte(pubKeyStr)); err != nil {
		return nil, err
	}
	pubKey, err := crypto.UnmarshalPublicKey(dat)
	if err != nil {
		return nil, err
	}
	return pubKey, nil
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
		addrCmp := strings.Compare(v.Address, other.Address)
		if addrCmp < 0 {
			return v
		} else if addrCmp > 0 {
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
		Address     string
		PubKey      string
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
func RandValidator(randPower bool, minPower int64) (*Validator, *PrivValidatorFS) {
	_, tempFilePath := cmn.Tempfile("priv_validator_")
	privVal := GenPrivValidatorFS(tempFilePath)
	votePower := minPower
	if randPower {
		votePower += int64(cmn.RandUint32())
	}
	val := NewValidator(privVal.GetPubKey(), votePower)
	return val, privVal
}
