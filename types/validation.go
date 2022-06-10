package types

import (
	"fmt"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	"github.com/tendermint/tendermint/crypto/ed25519"
)

// ValidateHash returns an error if the hash is not empty, but its
// size != crypto.HashSize.
func ValidateHash(h []byte) error {
	if len(h) > 0 && len(h) != crypto.HashSize {
		return fmt.Errorf("expected size to be %d bytes, got %d bytes",
			crypto.HashSize,
			len(h),
		)
	}
	return nil
}

// ValidateAppHash returns an error if the hash is not empty, but its
// size != tmhash.Size.
func ValidateAppHash(h []byte) error {
	if len(h) > 0 && (len(h) < crypto.SmallAppHashSize || len(h) > crypto.LargeAppHashSize) {
		return fmt.Errorf("expected size to be at between %d and %d bytes, got %d bytes",
			crypto.SmallAppHashSize,
			crypto.LargeAppHashSize,
			len(h),
		)
	}
	return nil
}

// ValidateSignatureSize returns an error if the signature is not empty, but its
// size != hash.Size.
func ValidateSignatureSize(keyType crypto.KeyType, h []byte) error {
	var signatureSize int // default
	switch keyType {
	case crypto.Ed25519:
		signatureSize = ed25519.SignatureSize
	case crypto.BLS12381:
		signatureSize = bls12381.SignatureSize
	default:
		panic("key type unknown")
	}
	if len(h) > 0 && len(h) != signatureSize {
		return fmt.Errorf("expected size to be %d bytes, got %d bytes",
			bls12381.SignatureSize,
			len(h),
		)
	}
	return nil
}
