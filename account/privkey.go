package account

import (
	"errors"
	"io"
	"reflect"

	"github.com/tendermint/go-ed25519"
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

// PrivKey is part of PrivAccount and state.PrivValidator.
type PrivKey interface {
	Sign(msg []byte) Signature
}

// Types of PrivKey implementations
const (
	PrivKeyTypeEd25519 = byte(0x01)
)

//-------------------------------------
// For binary.readReflect

func PrivKeyDecoder(r io.Reader, n *int64, err *error) interface{} {
	switch t := ReadByte(r, n, err); t {
	case PrivKeyTypeEd25519:
		return ReadBinary(PrivKeyEd25519{}, r, n, err)
	default:
		*err = Errorf("Unknown PrivKey type %X", t)
		return nil
	}
}

var _ = RegisterType(&TypeInfo{
	Type:    reflect.TypeOf((*PrivKey)(nil)).Elem(),
	Decoder: PrivKeyDecoder,
})

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
	return SignatureEd25519{signature}
}
