package account

import (
	"bytes"

	"github.com/tendermint/tendermint/Godeps/_workspace/src/github.com/tendermint/ed25519"
	"github.com/tendermint/tendermint/Godeps/_workspace/src/github.com/tendermint/ed25519/extra25519"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

// PubKey is part of Account and Validator.
type PubKey interface {
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
type PubKeyEd25519 [32]byte

// TODO: Or should this just be BinaryRipemd160(key)? (The difference is the TypeByte.)
func (pubKey PubKeyEd25519) Address() []byte { return binary.BinaryRipemd160(pubKey[:]) }

// TODO: Consider returning a reason for failure, or logging a runtime type mismatch.
func (pubKey PubKeyEd25519) VerifyBytes(msg []byte, sig_ Signature) bool {
	sig, ok := sig_.(SignatureEd25519)
	if !ok {
		return false
	}
	pubKeyBytes := [32]byte(pubKey)
	sigBytes := [64]byte(sig)
	return ed25519.Verify(&pubKeyBytes, msg, &sigBytes)
}

// For use with golang/crypto/nacl/box
// If error, returns nil.
func (pubKey PubKeyEd25519) ToCurve25519() *[32]byte {
	keyCurve25519, pubKeyBytes := new([32]byte), [32]byte(pubKey)
	ok := extra25519.PublicKeyToCurve25519(keyCurve25519, &pubKeyBytes)
	if !ok {
		return nil
	}
	return keyCurve25519
}

func (pubKey PubKeyEd25519) String() string {
	return Fmt("PubKeyEd25519{%X}", pubKey[:])
}

// Must return the full bytes in hex.
// Used for map keying, etc.
func (pubKey PubKeyEd25519) KeyString() string {
	return Fmt("%X", pubKey[:])
}

func (pubKey PubKeyEd25519) Equals(other PubKey) bool {
	if otherEd, ok := other.(PubKeyEd25519); ok {
		return bytes.Equal(pubKey[:], otherEd[:])
	} else {
		return false
	}
}
