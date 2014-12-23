package account

import (
	"errors"
	"io"
	"reflect"

	"github.com/tendermint/go-ed25519"
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

// PubKey is part of Account and Validator.
type PubKey interface {
	Address() []byte
	VerifyBytes(msg []byte, sig Signature) bool
}

// Types of PubKey implementations
const (
	PubKeyTypeUnknown = byte(0x00) // For pay-to-pubkey-hash txs.
	PubKeyTypeEd25519 = byte(0x01)
)

//-------------------------------------
// for binary.readReflect

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

// Implements PubKey
// For pay-to-pubkey-hash txs, where the TxOutput PubKey
// is not known in advance, only its hash (address).
type PubKeyUnknown struct {
	address []byte
}

func NewPubKeyUnknown(address []byte) PubKeyUnknown { return PubKeyUnknown{address} }

func (key PubKeyUnknown) TypeByte() byte { return PubKeyTypeUnknown }

func (key PubKeyUnknown) Address() []byte {
	return key.address
}

func (key PubKeyUnknown) VerifyBytes(msg []byte, sig_ Signature) bool {
	panic("PubKeyUnknown cannot verify messages")
}

//-------------------------------------

// Implements PubKey
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
