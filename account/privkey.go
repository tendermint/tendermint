package account

import (
	"errors"

	"github.com/tendermint/go-ed25519"
	. "github.com/tendermint/tendermint/binary"
)

// PrivKey is part of PrivAccount and state.PrivValidator.
type PrivKey interface {
	Sign(msg []byte) Signature
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
type PrivKeyEd25519 struct {
	PubKey  []byte
	PrivKey []byte
}

func (key PrivKeyEd25519) TypeByte() byte { return PrivKeyTypeEd25519 }

func (key PrivKeyEd25519) ValidateBasic() error {
	if len(key.PubKey) != ed25519.PublicKeySize {
		return errors.New("Invalid PrivKeyEd25519 pubkey size")
	}
	if len(key.PrivKey) != ed25519.PrivateKeySize {
		return errors.New("Invalid PrivKeyEd25519 privkey size")
	}
	return nil
}

func (key PrivKeyEd25519) Sign(msg []byte) Signature {
	signature := ed25519.SignMessage(msg, key.PrivKey, key.PubKey)
	return SignatureEd25519(signature)
}
