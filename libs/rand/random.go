package rand

import (
	mrand "math/rand"
)

const (
	strChars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz" // 62 characters
)

// Str constructs a random alphanumeric string of given length
// from math/rand's global default Source.
func Str(length int) string {
	if length <= 0 {
		return ""
	}

	chars := make([]byte, 0, length)
	for {
		// nolint:gosec // G404: Use of weak random number generator
		val := mrand.Int63()
		for i := 0; i < 10; i++ {
			v := int(val & 0x3f) // rightmost 6 bits
			if v >= 62 {         // only 62 characters in strChars
				val >>= 6
				continue
			} else {
				chars = append(chars, strChars[v])
				if len(chars) == length {
					return string(chars)
				}
				val >>= 6
			}
		}
	}
}

// Bytes returns n random bytes generated from math/rand's global default Source.
func Bytes(n int) []byte {
	bs := make([]byte, n)
	for i := 0; i < len(bs); i++ {
		// nolint:gosec // G404: Use of weak random number generator
		bs[i] = byte(mrand.Int() & 0xFF)
	}
	return bs
}
