package tmhash

import (
	"crypto/sha256"
)

const (
	Size          = sha256.Size
	BlockSize     = sha256.BlockSize
	TruncatedSize = 20
)

// Sum returns the SHA256 of the bz.
func Sum(bz []byte) []byte {
	h := sha256.Sum256(bz)
	return h[:]
}

//-------------------------------------------------------------

// SumTruncated returns the first 20 bytes of SHA256 of the bz.
func SumTruncated(bz []byte) []byte {
	hash := sha256.Sum256(bz)
	return hash[:TruncatedSize]
}
