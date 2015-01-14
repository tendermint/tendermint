package account

import (
	"errors"

	"github.com/tendermint/go-ed25519"
	. "github.com/tendermint/tendermint/binary"
)

// PrivKey is part of PrivAccount and state.PrivValidator.
type PrivKey interface {
	Sign(msg []byte) Signature
	PubKey() PubKey
}

// Types of PrivKey implementations
const (
	PrivKeyTypeEd25519 = byte(0x01)
)

// for binary.readReflect
var _ = RegisterInterface(
	struct{ PrivKey }{},
	ConcreteType{PrivKeyEd25519{}},
)

//-------------------------------------

// Implements PrivKey
type PrivKeyEd25519 []byte

func (key PrivKeyEd25519) TypeByte() byte { return PrivKeyTypeEd25519 }

func (key PrivKeyEd25519) ValidateBasic() error {
	if len(key) != ed25519.PrivateKeySize {
		return errors.New("Invalid PrivKeyEd25519 privkey size")
	}
	return nil
}

func (key PrivKeyEd25519) Sign(msg []byte) Signature {
	signature := ed25519.SignMessage(msg, key, ed25519.MakePubKey(key))
	return SignatureEd25519(signature)
}

func (key PrivKeyEd25519) PubKey() PubKey {
	return PubKeyEd25519(ed25519.MakePubKey(key))
}
