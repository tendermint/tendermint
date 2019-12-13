package ed25519

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/pkg/errors"
	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto"
)

var cdc = amino.NewCodec()

const (
	PrivKeyAminoName = "tendermint/PrivKeyEd25519"
	PubKeyAminoName  = "tendermint/PubKeyEd25519"

	// Size of an Edwards25519 signature. Namely the size of a compressed
	// Edwards25519 point, and a field element. Both of which are 32 bytes.
	SignatureSize = 64
)

var (
	prefixPubKeyEd25519 = []byte{0x16, 0x24, 0xDE, 0x64}
	lengthPubKeyEd25519 = []byte{0x20}

	prefixPrivKeyEd25519 = []byte{0xA3, 0x28, 0x89, 0x10}
	lengthPrivKeyEd25519 = []byte{0x40}
)

func init() {
	RegisterCodec(cdc)
	cdc.Seal()
}

// RegisterCodec registers the Ed25519 key types with the provided amino codec.
func RegisterCodec(c *amino.Codec) {
	c.RegisterInterface((*crypto.PubKey)(nil), nil)
	c.RegisterConcrete(PubKeyEd25519{}, PubKeyAminoName, nil)

	c.RegisterInterface((*crypto.PrivKey)(nil), nil)
	c.RegisterConcrete(PrivKeyEd25519{}, PrivKeyAminoName, nil)
}

// MarshalBinary attempts to marshal a PubKeyEd25519 type that is backwards
// compatible with Amino. It will return the raw encoded bytes or an error if the
// type is not registered.
//
// NOTE: Amino will not delegate MarshalBinaryBare calls to types that implement
// it. For now, clients must call MarshalBinary directly on the type to get the
// custom compatible encoding.
func (pubKey PubKeyEd25519) MarshalBinary() ([]byte, error) {
	p := len(prefixPubKeyEd25519)
	l := len(lengthPubKeyEd25519)
	bz := make([]byte, p+l+len(pubKey[:]))

	copy(bz[:p], prefixPubKeyEd25519)
	copy(bz[p:p+l], lengthPubKeyEd25519)
	copy(bz[p+l:], pubKey[:])

	return bz, nil
}

// UnmarshalBinary attempts to unmarshal provided amino compatbile bytes into a
// PubKeyEd25519 reference. An error is returned if the encoding is invalid.
//
// NOTE: Amino will not delegate UnmarshalBinaryBare calls to types that implement
// it. For now, clients must call UnmarshalBinary directly on the type to get the
// custom compatible decoding.
func (pubKey *PubKeyEd25519) UnmarshalBinary(bz []byte) error {
	p := len(prefixPubKeyEd25519)
	l := len(lengthPubKeyEd25519)

	if !bytes.Equal(bz[:p], prefixPubKeyEd25519) {
		return fmt.Errorf("invalid prefix; expected: %X, got: %X", prefixPubKeyEd25519, bz[:p])
	}
	if !bytes.Equal(bz[p:p+l], lengthPubKeyEd25519) {
		return fmt.Errorf("invalid encoding length; expected: %X, got: %X", lengthPubKeyEd25519, bz[p:p+l])
	}

	var el byte
	if err := binary.Read(bytes.NewReader(lengthPubKeyEd25519), binary.BigEndian, &el); err != nil {
		return errors.Wrap(err, "failed to read lengthPubKeyEd25519")
	}
	if len(bz[p+l:]) != int(el) {
		return fmt.Errorf("invalid key length; expected: %X, got: %X", lengthPubKeyEd25519, bz[p+l:])
	}

	copy(pubKey[:], bz[p+l:])
	return nil
}
