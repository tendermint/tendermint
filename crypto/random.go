package crypto

import (
	"crypto/rand"
	"encoding/hex"
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

// CRandHex returns a hex encoded string that's floor(numDigits/2) * 2 long.
//
// Note: CRandHex(24) gives 96 bits of randomness that
// are usually strong enough for most purposes.
func CRandHex(numDigits int) string {
	return hex.EncodeToString(CRandBytes(numDigits / 2))
}
