package ed25519

import (
	"bytes"
	"fmt"

	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto"
)

var cdc = amino.NewCodec()

func init() {
	cdc.RegisterInterface((*crypto.PubKey)(nil), nil)
	cdc.RegisterConcrete(PubKey{},
		PubKeyAminoName, nil)

	cdc.RegisterInterface((*crypto.PrivKey)(nil), nil)
	cdc.RegisterConcrete(PrivKey{},
		PrivKeyAminoName, nil)
}

// -------------------------------------

var (
	prefixPrivKeyEd25519      = []byte{0xA3, 0x28, 0x89, 0x10}
	lengthPrivKeyEd25519 byte = 0x40

	prefixPubKeyEd25519      = []byte{0x16, 0x24, 0xDE, 0x64}
	lengthPubKeyEd25519 byte = 0x20
)

// Marshal attempts to marshal a PrivKeyEd25519 type that is backwards
// compatible with Amino.
func (privKey PrivKey) AminoMarshal() ([]byte, error) {
	lbz := []byte{lengthPrivKeyEd25519}
	p := len(prefixPrivKeyEd25519)
	l := len(lbz)
	bz := make([]byte, p+l+len(privKey))

	copy(bz[:p], prefixPrivKeyEd25519)
	copy(bz[p:p+l], lbz)
	copy(bz[p+l:], privKey[:])

	return bz, nil
}

// Unmarshal attempts to unmarshal provided amino compatbile bytes into a
// PrivKeyEd25519 reference. An error is returned if the encoding is invalid.
func (privKey *PrivKey) AminoUnmarshal(bz []byte) error {
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

	*privKey = bz[p+l:]
	return nil
}

// Marshal attempts to marshal a PubKeyEd25519 type that is backwards
// compatible with Amino.
func (pubKey PubKey) AminoMarshal() ([]byte, error) {
	lbz := []byte{lengthPubKeyEd25519}
	p := len(prefixPubKeyEd25519)
	l := len(lbz)
	bz := make([]byte, p+l+len(pubKey[:]))

	copy(bz[:p], prefixPubKeyEd25519)
	copy(bz[p:p+l], lbz)
	copy(bz[p+l:], pubKey[:])

	return bz, nil
}

// Unmarshal attempts to unmarshal provided amino compatbile bytes into a
// PubKeyEd25519 reference. An error is returned if the encoding is invalid.
func (pubKey *PubKey) AminoUnmarshal(bz []byte) error {
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

	*pubKey = bz[p+l:]
	return nil
}
