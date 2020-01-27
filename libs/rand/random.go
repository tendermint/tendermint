package rand

import (
	crand "crypto/rand"
	mrand "math/rand"
	"time"
)

const (
	strChars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz" // 62 characters
)

// Rand is a prng, that is seeded with OS randomness.
// The OS randomness is obtained from crypto/rand, however none of the provided
// methods are suitable for cryptographic usage.
// They all utilize math/rand's prng internally.
type Rand struct {
	rand *mrand.Rand
}

func NewRand() *Rand {
	rand := &Rand{}
	rand.init()
	return rand
}

func (r *Rand) init() {
	bz := cRandBytes(8)
	var seed uint64
	for i := 0; i < 8; i++ {
		seed |= uint64(bz[i])
		seed <<= 8
	}
	r.reset(int64(seed))
}

func (r *Rand) reset(seed int64) {
	r.rand = mrand.New(mrand.NewSource(seed))
}

// //----------------------------------------
// // Global functions

// func Seed(seed int64) {
// 	var lrand = NewRand()
// 	lrand.Seed(seed)
// }

// func Str(length int) string {
// 	var lrand = NewRand()
// 	return lrand.Str(length)
// }

// func Uint16() uint16 {
// 	var lrand = NewRand()
// 	return lrand.Uint16()
// }

// func Uint32() uint32 {
// 	var lrand = NewRand()
// 	return lrand.Uint32()
// }

// func Uint64() uint64 {
// 	var lrand = NewRand()
// 	return lrand.Uint64()
// }

// func Uint() uint {
// 	var lrand = NewRand()
// 	return lrand.Uint()
// }

// func Int16() int16 {
// 	var lrand = NewRand()
// 	return lrand.Int16()
// }

// func Int32() int32 {
// 	var lrand = NewRand()
// 	return lrand.Int32()
// }

// func Int64() int64 {
// 	var lrand = NewRand()
// 	return lrand.Int64()
// }

// func Int() int {
// 	var lrand = NewRand()
// 	return lrand.Int()
// }

// func Int31() int32 {
// 	var lrand = NewRand()
// 	return lrand.Int31()
// }

// func Int31n(n int32) int32 {
// 	var lrand = NewRand()
// 	return lrand.Int31n(n)
// }

// func Int63() int64 {
// 	var lrand = NewRand()
// 	return lrand.Int63()
// }

// func Int63n(n int64) int64 {
// 	var lrand = NewRand()
// 	return lrand.Int63n(n)
// }

// func Bool() bool {
// 	var lrand = NewRand()
// 	return lrand.Bool()
// }

// func Float32() float32 {
// 	var lrand = NewRand()
// 	return lrand.Float32()
// }

// func Float64() float64 {
// 	var lrand = NewRand()
// 	return lrand.Float64()
// }

// func Time() time.Time {
// 	var lrand = NewRand()
// 	return lrand.Time()
// }

// func Bytes(n int) []byte {
// 	var lrand = NewRand()
// 	return lrand.Bytes(n)
// }

// func Intn(n int) int {
// 	var lrand = NewRand()
// 	return lrand.Intn(n)
// }

// func Perm(n int) []int {
// 	var lrand = NewRand()
// 	return lrand.Perm(n)
// }

//----------------------------------------
// Rand methods

func (r *Rand) Seed(seed int64) {
	r.reset(seed)
}

// Str constructs a random alphanumeric string of given length.
func (r *Rand) Str(length int) string {
	chars := []byte{}
MAIN_LOOP:
	for {
		val := r.Int63()
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

func (r *Rand) Uint16() uint16 {
	return uint16(r.Uint32() & (1<<16 - 1))
}

func (r *Rand) Uint32() uint32 {
	u32 := r.rand.Uint32()
	return u32
}

func (r *Rand) Uint64() uint64 {
	return uint64(r.Uint32())<<32 + uint64(r.Uint32())
}

func (r *Rand) Uint() uint {
	i := r.rand.Int()
	return uint(i)
}

func (r *Rand) Int16() int16 {
	return int16(r.Uint32() & (1<<16 - 1))
}

func (r *Rand) Int32() int32 {
	return int32(r.Uint32())
}

func (r *Rand) Int64() int64 {
	return int64(r.Uint64())
}

func (r *Rand) Int() int {
	i := r.rand.Int()
	return i
}

func (r *Rand) Int31() int32 {
	i31 := r.rand.Int31()
	return i31
}

func (r *Rand) Int31n(n int32) int32 {
	i31n := r.rand.Int31n(n)
	return i31n
}

func (r *Rand) Int63() int64 {
	i63 := r.rand.Int63()
	return i63
}

func (r *Rand) Int63n(n int64) int64 {
	i63n := r.rand.Int63n(n)
	return i63n
}

func (r *Rand) Float32() float32 {
	f32 := r.rand.Float32()
	return f32
}

func (r *Rand) Float64() float64 {
	f64 := r.rand.Float64()
	return f64
}

func (r *Rand) Time() time.Time {
	return time.Unix(int64(r.Uint64()), 0)
}

// Bytes returns n random bytes generated from the internal
// prng.
func (r *Rand) Bytes(n int) []byte {
	// cRandBytes isn't guaranteed to be fast so instead
	// use random bytes generated from the internal PRNG
	bs := make([]byte, n)
	for i := 0; i < len(bs); i++ {
		bs[i] = byte(r.Int() & 0xFF)
	}
	return bs
}

// Intn returns, as an int, a uniform pseudo-random number in the range [0, n).
// It panics if n <= 0.
func (r *Rand) Intn(n int) int {
	i := r.rand.Intn(n)
	return i
}

// Bool returns a uniformly random boolean
func (r *Rand) Bool() bool {
	// See https://github.com/golang/go/issues/23804#issuecomment-365370418
	// for reasoning behind computing like this
	return r.Int63()%2 == 0
}

// Perm returns a pseudo-random permutation of n integers in [0, n).
func (r *Rand) Perm(n int) []int {
	perm := r.rand.Perm(n)
	return perm
}

// NOTE: This relies on the os's random number generator.
// For real security, we should salt that with some seed.
// See github.com/tendermint/tendermint/crypto for a more secure reader.
func cRandBytes(numBytes int) []byte {
	b := make([]byte, numBytes)
	_, err := crand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}
