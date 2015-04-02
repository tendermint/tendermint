package account

import (
	"github.com/tendermint/ed25519"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

// PrivKey is part of PrivAccount and state.PrivValidator.
type PrivKey interface {
	TypeByte() byte
	Sign(msg []byte) Signature
	PubKey() PubKey
}

// Types of PrivKey implementations
const (
	PrivKeyTypeEd25519 = byte(0x01)
)

// for binary.readReflect
var _ = binary.RegisterInterface(
	struct{ PrivKey }{},
	binary.ConcreteType{PrivKeyEd25519{}},
)

//-------------------------------------

// Implements PrivKey
type PrivKeyEd25519 []byte

func (privKey PrivKeyEd25519) TypeByte() byte { return PrivKeyTypeEd25519 }

func (privKey PrivKeyEd25519) Sign(msg []byte) Signature {
	pubKey := privKey.PubKey().(PubKeyEd25519)
	privKeyBytes := new([64]byte)
	copy(privKeyBytes[:32], privKey[:])
	copy(privKeyBytes[32:], pubKey[:])
	signatureBytes := ed25519.Sign(privKeyBytes, msg)
	return SignatureEd25519(signatureBytes[:])
}

func (key PrivKeyEd25519) PubKey() PubKey {
	keyBytes := new([64]byte)
	copy(keyBytes[:], key[:])
	return PubKeyEd25519(ed25519.MakePublicKey(keyBytes)[:])
}

func (key PrivKeyEd25519) String() string {
	return Fmt("PrivKeyEd25519{*****}")
}
