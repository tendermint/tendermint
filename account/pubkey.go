package account

import (
	"errors"
	"reflect"

	"github.com/tendermint/go-ed25519"
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

// PubKey is part of Account and Validator.
type PubKey interface {
	Address() []byte
	VerifyBytes(msg []byte, sig Signature) bool
	TypeByte() byte
}

// Types of PubKey implementations
const (
	PubKeyTypeNil     = byte(0x00)
	PubKeyTypeEd25519 = byte(0x01)
)

//-------------------------------------
// for binary.readReflect

func PubKeyDecoder(r Unreader, n *int64, err *error) interface{} {
	switch t := PeekByte(r, n, err); t {
	case PubKeyTypeNil:
		return PubKeyNil{}
	case PubKeyTypeEd25519:
		return ReadBinary(PubKeyEd25519{}, r, n, err)
	default:
		*err = Errorf("Unknown PubKey type %X", t)
		return nil
	}
}

var _ = RegisterType(&TypeInfo{
	Type:    reflect.TypeOf((*PubKey)(nil)).Elem(),
	Decoder: PubKeyDecoder,
})

//-------------------------------------

// Implements PubKey
type PubKeyNil struct{}

func (key PubKeyNil) TypeByte() byte { return PubKeyTypeNil }

func (key PubKeyNil) Address() []byte {
	panic("PubKeyNil has no address")
}

func (key PubKeyNil) VerifyBytes(msg []byte, sig_ Signature) bool {
	panic("PubKeyNil cannot verify messages")
}

//-------------------------------------

// Implements PubKey
type PubKeyEd25519 []byte

func (key PubKeyEd25519) TypeByte() byte { return PubKeyTypeEd25519 }

// TODO: Or should this just be BinaryRipemd160(key)? (The difference is the TypeByte.)
func (key PubKeyEd25519) Address() []byte { return BinaryRipemd160([]byte(key)) }

func (key PubKeyEd25519) ValidateBasic() error {
	if len(key) != ed25519.PublicKeySize {
		return errors.New("Invalid PubKeyEd25519 key size")
	}
	return nil
}

func (key PubKeyEd25519) VerifyBytes(msg []byte, sig_ Signature) bool {
	sig, ok := sig_.(SignatureEd25519)
	if !ok {
		panic("PubKeyEd25519 expects an SignatureEd25519 signature")
	}
	v1 := &ed25519.Verify{
		Message:   msg,
		PubKey:    key,
		Signature: sig,
	}
	return ed25519.VerifyBatch([]*ed25519.Verify{v1})
}
