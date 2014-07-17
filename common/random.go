package common

import (
	crand "crypto/rand"
	"encoding/hex"
	"math/rand"
)

const (
	strChars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz" // 62 characters
)

func init() {
	// Seed math/rand with "secure" int64
	b := RandBytes(8)
	var seed uint64
	for i := 0; i < 8; i++ {
		seed |= uint64(b[i])
		seed <<= 8
	}
	rand.Seed(int64(seed))
}

// Constructs an alphanumeric string of given length.
// Not crypto safe
func RandStr(length int) string {
	chars := []byte{}
MAIN_LOOP:
	for {
		val := rand.Int63()
		for i := 0; i < 10; i++ {
			v := int(val & 0x3f) // rightmost 6 bits
			if v >= 62 {         // only 62 characters in strChars
				val >>= 6
				continue
			} else {
				chars = append(chars, strChars[v])
				if len(chars) == length {
					break MAIN_LOOP
				}
				val >>= 6
			}
		}
	}

	return string(chars)
}

// Crypto safe
func RandBytes(numBytes int) []byte {
	b := make([]byte, numBytes)
	_, err := crand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

// Crypto safe
// RandHex(24) gives 96 bits of randomness, strong enough for most purposes.
func RandHex(numDigits int) string {
	return hex.EncodeToString(RandBytes(numDigits / 2))
}
