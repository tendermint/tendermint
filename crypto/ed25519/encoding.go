package ed25519

import (
	"bytes"
	"fmt"

	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto"
)

const (
	PrivKeyAminoName = "tendermint/PrivKeyEd25519"
	PubKeyAminoName  = "tendermint/PubKeyEd25519"

	// Size of an Edwards25519 signature. Namely the size of a compressed
	// Edwards25519 point, and a field element. Both of which are 32 bytes.
	SignatureSize = 64
)

var (
	prefixPrivKeyEd25519      = []byte{0xA3, 0x28, 0x89, 0x10}
	lengthPrivKeyEd25519 byte = 0x40

	prefixPubKeyEd25519      = []byte{0x16, 0x24, 0xDE, 0x64}
	lengthPubKeyEd25519 byte = 0x20
)

// RegisterCodec registers the Ed25519 key types with the provided amino codec.
func RegisterCodec(c *amino.Codec) {
	c.RegisterInterface((*crypto.PubKey)(nil), nil)
	c.RegisterConcrete(PubKeyEd25519{}, PubKeyAminoName, nil)

	c.RegisterInterface((*crypto.PrivKey)(nil), nil)
	c.RegisterConcrete(PrivKeyEd25519{}, PrivKeyAminoName, nil)
}

// MarshalBinary attempts to marshal a PrivKeyEd25519 type that is backwards
// compatible with Amino.
//
// NOTE: Amino will not delegate MarshalBinaryBare calls to types that implement
// it. For now, clients must call MarshalBinary directly on the type to get the
// custom compatible encoding.
func (privKey PrivKeyEd25519) MarshalBinary() ([]byte, error) {
	lbz := []byte{lengthPrivKeyEd25519}
	p := len(prefixPrivKeyEd25519)
	l := len(lbz)
	bz := make([]byte, p+l+len(privKey[:]))

	copy(bz[:p], prefixPrivKeyEd25519)
	copy(bz[p:p+l], lbz)
	copy(bz[p+l:], privKey[:])

	return bz, nil
}

// UnmarshalBinary attempts to unmarshal provided amino compatbile bytes into a
// PrivKeyEd25519 reference. An error is returned if the encoding is invalid.
//
// NOTE: Amino will not delegate UnmarshalBinaryBare calls to types that implement
// it. For now, clients must call UnmarshalBinary directly on the type to get the
// custom compatible decoding.
func (privKey *PrivKeyEd25519) UnmarshalBinary(bz []byte) error {
	lbz := []byte{lengthPrivKeyEd25519}
	p := len(prefixPrivKeyEd25519)
	l := len(lbz)

	if !bytes.Equal(bz[:p], prefixPrivKeyEd25519) {
		return fmt.Errorf("invalid prefix; expected: %X, got: %X", prefixPrivKeyEd25519, bz[:p])
	}
	if !bytes.Equal(bz[p:p+l], lbz) {
		return fmt.Errorf("invalid encoding length; expected: %X, got: %X", lbz, bz[p:p+l])
	}
	if len(bz[p+l:]) != int(lengthPrivKeyEd25519) {
		return fmt.Errorf("invalid key length; expected: %d, got: %d", int(lengthPrivKeyEd25519), len(bz[p+l:]))
	}

	copy(privKey[:], bz[p+l:])
	return nil
}

// MarshalBinary attempts to marshal a PubKeyEd25519 type that is backwards
// compatible with Amino.
//
// NOTE: Amino will not delegate MarshalBinaryBare calls to types that implement
// it. For now, clients must call MarshalBinary directly on the type to get the
// custom compatible encoding.
func (pubKey PubKeyEd25519) MarshalBinary() ([]byte, error) {
	lbz := []byte{lengthPubKeyEd25519}
	p := len(prefixPubKeyEd25519)
	l := len(lbz)
	bz := make([]byte, p+l+len(pubKey[:]))

	copy(bz[:p], prefixPubKeyEd25519)
	copy(bz[p:p+l], lbz)
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
	lbz := []byte{lengthPubKeyEd25519}
	p := len(prefixPubKeyEd25519)
	l := len(lbz)

	if !bytes.Equal(bz[:p], prefixPubKeyEd25519) {
		return fmt.Errorf("invalid prefix; expected: %X, got: %X", prefixPubKeyEd25519, bz[:p])
	}
	if !bytes.Equal(bz[p:p+l], lbz) {
		return fmt.Errorf("invalid encoding length; expected: %X, got: %X", lbz, bz[p:p+l])
	}
	if len(bz[p+l:]) != int(lengthPubKeyEd25519) {
		return fmt.Errorf("invalid key length; expected: %d, got: %d", int(lengthPubKeyEd25519), len(bz[p+l:]))
	}

	copy(pubKey[:], bz[p+l:])
	return nil
}
