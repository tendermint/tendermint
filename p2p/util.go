package p2p

import (
	"crypto/sha256"
)

// doubleSha256 calculates sha256(sha256(b)) and returns the resulting bytes.
func doubleSha256(b []byte) []byte {
	hasher := sha256.New()
	_, _ = hasher.Write(b) // error ignored
	sum := hasher.Sum(nil)
	hasher.Reset()
	_, _ = hasher.Write(sum) // error ignored
	return hasher.Sum(nil)
}
