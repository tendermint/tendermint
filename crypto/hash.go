package crypto

import "golang.org/x/crypto/sha3"

func Sha256(bytes []byte) []byte {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(bytes)
	return hasher.Sum(nil)
}
