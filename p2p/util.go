package p2p

import (
	"crypto/sha256"
)

// doubleSha256 calculates sha256(sha256(b)) and returns the resulting bytes.
func doubleSha256(b []byte) []byte {
	hasher := sha256.New()
	hasher.Write(b) // nolint: errcheck, gas
	sum := hasher.Sum(nil)
	hasher.Reset()
	hasher.Write(sum) // nolint: errcheck, gas
	return hasher.Sum(nil)
}
