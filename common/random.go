package common

import (
	crand "crypto/rand"
	mrand "math/rand"
	"sync"
	"time"
)

const (
	strChars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz" // 62 characters
)

// pseudo random number generator.
// seeded with OS randomness (crand)
var prng struct {
	sync.Mutex
	*mrand.Rand
}

func init() {
	b := cRandBytes(8)
	var seed uint64
	for i := 0; i < 8; i++ {
		seed |= uint64(b[i])
		seed <<= 8
	}
	prng.Rand = mrand.New(mrand.NewSource(int64(seed)))
}

// Constructs an alphanumeric string of given length.
func RandStr(length int) string {
	chars := []byte{}
MAIN_LOOP:
	for {
		val := prng.Int63()
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

func RandUint16() uint16 {
	return uint16(RandUint32() & (1<<16 - 1))
}

func RandUint32() uint32 {
	prng.Lock()
	u32 := prng.Uint32()
	prng.Unlock()
	return u32
}

func RandUint64() uint64 {
	return uint64(RandUint32())<<32 + uint64(RandUint32())
}

func RandUint() uint {
	prng.Lock()
	i := prng.Int()
	prng.Unlock()
	return uint(i)
}

func RandInt16() int16 {
	return int16(RandUint32() & (1<<16 - 1))
}

func RandInt32() int32 {
	return int32(RandUint32())
}

func RandInt64() int64 {
	return int64(RandUint64())
}

func RandInt() int {
	prng.Lock()
	i := prng.Int()
	prng.Unlock()
	return i
}

func RandInt31() int32 {
	prng.Lock()
	i31 := prng.Int31()
	prng.Unlock()
	return i31
}

func RandInt63() int64 {
	prng.Lock()
	i63 := prng.Int63()
	prng.Unlock()
	return i63
}

// Distributed pseudo-exponentially to test for various cases
func RandUint16Exp() uint16 {
	bits := RandUint32() % 16
	if bits == 0 {
		return 0
	}
	n := uint16(1 << (bits - 1))
	n += uint16(RandInt31()) & ((1 << (bits - 1)) - 1)
	return n
}

// Distributed pseudo-exponentially to test for various cases
func RandUint32Exp() uint32 {
	bits := RandUint32() % 32
	if bits == 0 {
		return 0
	}
	n := uint32(1 << (bits - 1))
	n += uint32(RandInt31()) & ((1 << (bits - 1)) - 1)
	return n
}

// Distributed pseudo-exponentially to test for various cases
func RandUint64Exp() uint64 {
	bits := RandUint32() % 64
	if bits == 0 {
		return 0
	}
	n := uint64(1 << (bits - 1))
	n += uint64(RandInt63()) & ((1 << (bits - 1)) - 1)
	return n
}

func RandFloat32() float32 {
	prng.Lock()
	f32 := prng.Float32()
	prng.Unlock()
	return f32
}

func RandTime() time.Time {
	return time.Unix(int64(RandUint64Exp()), 0)
}

// RandBytes returns n random bytes from the OS's source of entropy ie. via crypto/rand.
func RandBytes(n int) []byte {
	return cRandBytes(n)
}

// RandIntn returns, as an int, a non-negative pseudo-random number in [0, n).
// It panics if n <= 0
func RandIntn(n int) int {
	prng.Lock()
	i := prng.Intn(n)
	prng.Unlock()
	return i
}

// RandPerm returns a pseudo-random permutation of n integers in [0, n).
func RandPerm(n int) []int {
	prng.Lock()
	perm := prng.Perm(n)
	prng.Unlock()
	return perm
}

// NOTE: This relies on the os's random number generator.
// For real security, we should salt that with some seed.
// See github.com/tendermint/go-crypto for a more secure reader.
func cRandBytes(numBytes int) []byte {
	b := make([]byte, numBytes)
	_, err := crand.Read(b)
	if err != nil {
		PanicCrisis(err)
	}
	return b
}
