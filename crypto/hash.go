package crypto

import (
	"crypto/sha256"
)

func Sha256(bytes []byte) []byte {
	hasher := sha256.New()
	_, _ = hasher.Write(bytes)
	return hasher.Sum(nil)
}
