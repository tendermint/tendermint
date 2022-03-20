package rand

import (
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	mrand "math/rand"
)

const (
	strChars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz" // 62 characters
)

func init() {
	Reseed()
}

// NewRand returns a prng, that is seeded with OS randomness.
// The OS randomness is obtained from crypto/rand, however, like with any math/rand.Rand
// object none of the provided methods are suitable for cryptographic usage.
//
// Note that the returned instance of math/rand's Rand is not
// suitable for concurrent use by multiple goroutines.
//
// For concurrent use, call Reseed to reseed math/rand's default source and
// use math/rand's top-level convenience functions instead.
func NewRand() *mrand.Rand {
	seed := crandSeed()
	// nolint:gosec // G404: Use of weak random number generator
	return mrand.New(mrand.NewSource(seed))
}

// Reseed conveniently re-seeds the default Source of math/rand with
// randomness obtained from crypto/rand.
//
// Note that this does not make math/rand suitable for cryptographic usage.
//
// Use math/rand's top-level convenience functions remain suitable
// for concurrent use by multiple goroutines.
func Reseed() {
	seed := crandSeed()
	mrand.Seed(seed)
}

// Str constructs a random alphanumeric string of given length
// from math/rand's global default Source.
func Str(length int) string { return buildString(length, mrand.Int63) }

// StrFromSource produces a random string of a specified length from
// the specified random source.
func StrFromSource(r *mrand.Rand, length int) string { return buildString(length, r.Int63) }

func buildString(length int, picker func() int64) string {
	if length <= 0 {
		return ""
	}

	chars := make([]byte, 0, length)
	for {
		val := picker()
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

func crandSeed() int64 {
	var seed int64
	err := binary.Read(crand.Reader, binary.BigEndian, &seed)
	if err != nil {
		panic(fmt.Sprintf("could nor read random seed from crypto/rand: %v", err))
	}
	return seed
}
