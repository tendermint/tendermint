package crypto

import (
	"crypto/rand"
)

// This only uses the OS's randomness
func CRandBytes(numBytes int) []byte {
	b := make([]byte, numBytes)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}
