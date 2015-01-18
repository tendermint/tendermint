package account

import (
	"errors"

	"github.com/tendermint/ed25519"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

// PubKey is part of Account and Validator.
type PubKey interface {
	TypeByte() byte
	IsNil() bool
	Address() []byte
	VerifyBytes(msg []byte, sig Signature) bool
}

// Types of PubKey implementations
const (
	PubKeyTypeNil     = byte(0x00)
	PubKeyTypeEd25519 = byte(0x01)
)

// for binary.readReflect
var _ = binary.RegisterInterface(
	struct{ PubKey }{},
	binary.ConcreteType{PubKeyNil{}},
	binary.ConcreteType{PubKeyEd25519{}},
)

//-------------------------------------

// Implements PubKey
type PubKeyNil struct{}

func (key PubKeyNil) TypeByte() byte { return PubKeyTypeNil }

func (key PubKeyNil) IsNil() bool { return true }

func (key PubKeyNil) Address() []byte {
	panic("PubKeyNil has no address")
}

func (key PubKeyNil) VerifyBytes(msg []byte, sig_ Signature) bool {
	panic("PubKeyNil cannot verify messages")
}

func (key PubKeyEd25519) ValidateBasic() error {
	if len(key) != ed25519.PublicKeySize {
		return errors.New("Invalid PubKeyEd25519 key size")
	}
	return nil
}

func (key PubKeyNil) String() string {
	return "PubKeyNil{}"
}

//-------------------------------------

// Implements PubKey
type PubKeyEd25519 []byte

func (pubKey PubKeyEd25519) TypeByte() byte { return PubKeyTypeEd25519 }

func (pubKey PubKeyEd25519) IsNil() bool { return false }

// TODO: Or should this just be BinaryRipemd160(key)? (The difference is the TypeByte.)
func (pubKey PubKeyEd25519) Address() []byte { return binary.BinaryRipemd160(pubKey) }

func (pubKey PubKeyEd25519) VerifyBytes(msg []byte, sig_ Signature) bool {
	sig, ok := sig_.(SignatureEd25519)
	if !ok {
		panic("PubKeyEd25519 expects an SignatureEd25519 signature")
	}
	pubKeyBytes := new([32]byte)
	copy(pubKeyBytes[:], pubKey)
	sigBytes := new([64]byte)
	copy(sigBytes[:], sig)
	return ed25519.Verify(pubKeyBytes, msg, sigBytes)
}

func (pubKey PubKeyEd25519) String() string {
	return Fmt("PubKeyEd25519{%X}", pubKey)
}
