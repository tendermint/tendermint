package p2p

import (
	"crypto/sha256"
)

// doubleSha256 calculates sha256(sha256(b)) and returns the resulting bytes.
func doubleSha256(b []byte) []byte {
	hasher := sha256.New()
	_, err := hasher.Write(b)
	if err != nil {
		panic(err)
	}
	sum := hasher.Sum(nil)
	hasher.Reset()
	_, err = hasher.Write(sum)
	if err != nil {
		panic(err)
	}
	return hasher.Sum(nil)
}
