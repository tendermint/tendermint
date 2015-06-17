package account

import (
	"errors"
	"github.com/tendermint/tendermint/Godeps/_workspace/src/github.com/tendermint/ed25519"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

// PubKey is part of Account and Validator.
type PubKey interface {
	IsNil() bool
	Address() []byte
	VerifyBytes(msg []byte, sig Signature) bool
}

// Types of PubKey implementations
const (
	PubKeyTypeEd25519 = byte(0x01)
)

// for binary.readReflect
var _ = binary.RegisterInterface(
	struct{ PubKey }{},
	binary.ConcreteType{PubKeyEd25519{}, PubKeyTypeEd25519},
)

//-------------------------------------

// Implements PubKey
type PubKeyEd25519 []byte

func (pubKey PubKeyEd25519) IsNil() bool { return false }

// TODO: Or should this just be BinaryRipemd160(key)? (The difference is the TypeByte.)
func (pubKey PubKeyEd25519) Address() []byte { return binary.BinaryRipemd160(pubKey) }

// TODO: Consider returning a reason for failure, or logging a runtime type mismatch.
func (pubKey PubKeyEd25519) VerifyBytes(msg []byte, sig_ Signature) bool {
	sig, ok := sig_.(SignatureEd25519)
	if !ok {
		return false
	}
	pubKeyBytes := new([32]byte)
	copy(pubKeyBytes[:], pubKey)
	sigBytes := new([64]byte)
	copy(sigBytes[:], sig)
	return ed25519.Verify(pubKeyBytes, msg, sigBytes)
}

func (pubKey PubKeyEd25519) ValidateBasic() error {
	if len(pubKey) != ed25519.PublicKeySize {
		return errors.New("Invalid PubKeyEd25519 key size")
	}
	return nil
}

func (pubKey PubKeyEd25519) String() string {
	return Fmt("PubKeyEd25519{%X}", []byte(pubKey))
}
