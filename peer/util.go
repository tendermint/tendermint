package peer

import (
	"crypto/sha256"
)

// DoubleSha256 calculates sha256(sha256(b)) and returns the resulting bytes.
func DoubleSha256(b []byte) []byte {
	hasher := sha256.New()
	hasher.Write(b)
	sum := hasher.Sum(nil)
	hasher.Reset()
	hasher.Write(sum)
	return hasher.Sum(nil)
}
