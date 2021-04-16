package rand

import (
	crand "crypto/rand"
	"encoding/binary"
	mrand "math/rand"
)

const (
	strChars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz" // 62 characters
)

// NewRand returns a prng, that is seeded with OS randomness.
// The OS randomness is obtained from crypto/rand, however, like with any math/rand.Rand
// object none of the provided methods are suitable for cryptographic usage.
//
// As this returns a math/rand.Rand instance all methods on
// that object are suitable for concurrent use.
func NewRand() *mrand.Rand {
	var seed int64
	binary.Read(crand.Reader, binary.BigEndian, &seed)
	return mrand.New(mrand.NewSource(seed))
}

// Str constructs a random alphanumeric string of given length
// from a freshly instantiated prng.
func Str(length int) string {
	rand := NewRand()
	if length <= 0 {
		return ""
	}

	chars := make([]byte, 0, length)
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
					return string(chars)
				}
				val >>= 6
			}
		}
	}
}

// Bytes returns n random bytes generated from a freshly instantiated prng.
func Bytes(n int) []byte {
	rand := NewRand()
	bs := make([]byte, n)
	for i := 0; i < len(bs); i++ {
		bs[i] = byte(rand.Int() & 0xFF)
	}
	return bs
}
