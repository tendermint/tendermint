package account

import (
	"errors"
	"io"
	"reflect"

	"github.com/tendermint/go-ed25519"
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

/*
Each account has an PubKey which determines access to funds of the account.
Transaction inputs include Signatures of the account's associated PubKey.
*/
type PubKey interface {
	Address() []byte
	VerifyBytes(msg []byte, sig Signature) bool
}

const (
	PubKeyTypeUnknown = byte(0x00)
	PubKeyTypeEd25519 = byte(0x01)
)

func PubKeyDecoder(r io.Reader, n *int64, err *error) interface{} {
	switch t := ReadByte(r, n, err); t {
	case PubKeyTypeUnknown:
		return PubKeyUnknown{}
	case PubKeyTypeEd25519:
		return ReadBinary(&PubKeyEd25519{}, r, n, err)
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

type PubKeyUnknown struct {
}

func (key PubKeyUnknown) TypeByte() byte { return PubKeyTypeUnknown }

func (key PubKeyUnknown) Address() []byte {
	panic("PubKeyUnknown has no address")
}

func (key PubKeyUnknown) VerifyBytes(msg []byte, sig_ Signature) bool {
	panic("PubKeyUnknown cannot verify messages")
}

//-------------------------------------

type PubKeyEd25519 struct {
	PubKey []byte
}

func (key PubKeyEd25519) TypeByte() byte { return PubKeyTypeEd25519 }

func (key PubKeyEd25519) Address() []byte { return BinaryRipemd160(key.PubKey) }

func (key PubKeyEd25519) ValidateBasic() error {
	if len(key.PubKey) != ed25519.PublicKeySize {
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
		PubKey:    key.PubKey,
		Signature: []byte(sig.Bytes),
	}
	return ed25519.VerifyBatch([]*ed25519.Verify{v1})
}
