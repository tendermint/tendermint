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

type Rand struct {
	sync.Mutex
	rand *mrand.Rand
}

var grand *Rand

func init() {
	grand = NewRand()
	grand.init()
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

//----------------------------------------
// Global functions

func Seed(seed int64) {
	grand.Seed(seed)
}

func RandStr(length int) string {
	return grand.Str(length)
}

func RandUint16() uint16 {
	return grand.Uint16()
}

func RandUint32() uint32 {
	return grand.Uint32()
}

func RandUint64() uint64 {
	return grand.Uint64()
}

func RandUint() uint {
	return grand.Uint()
}

func RandInt16() int16 {
	return grand.Int16()
}

func RandInt32() int32 {
	return grand.Int32()
}

func RandInt64() int64 {
	return grand.Int64()
}

func RandInt() int {
	return grand.Int()
}

func RandInt31() int32 {
	return grand.Int31()
}

func RandInt31n(n int32) int32 {
	return grand.Int31n(n)
}

func RandInt63() int64 {
	return grand.Int63()
}

func RandInt63n(n int64) int64 {
	return grand.Int63n(n)
}

func RandUint16Exp() uint16 {
	return grand.Uint16Exp()
}

func RandUint32Exp() uint32 {
	return grand.Uint32Exp()
}

func RandUint64Exp() uint64 {
	return grand.Uint64Exp()
}

func RandFloat32() float32 {
	return grand.Float32()
}

func RandFloat64() float64 {
	return grand.Float64()
}

func RandTime() time.Time {
	return grand.Time()
}

func RandBytes(n int) []byte {
	return grand.Bytes(n)
}

func RandIntn(n int) int {
	return grand.Intn(n)
}

func RandPerm(n int) []int {
	return grand.Perm(n)
}

//----------------------------------------
// Rand methods

func (r *Rand) Seed(seed int64) {
	r.Lock()
	r.reset(seed)
	r.Unlock()
}

// Constructs an alphanumeric string of given length.
// It is not safe for cryptographic usage.
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

// It is not safe for cryptographic usage.
func (r *Rand) Uint16() uint16 {
	return uint16(r.Uint32() & (1<<16 - 1))
}

// It is not safe for cryptographic usage.
func (r *Rand) Uint32() uint32 {
	r.Lock()
	u32 := r.rand.Uint32()
	r.Unlock()
	return u32
}

// It is not safe for cryptographic usage.
func (r *Rand) Uint64() uint64 {
	return uint64(r.Uint32())<<32 + uint64(r.Uint32())
}

// It is not safe for cryptographic usage.
func (r *Rand) Uint() uint {
	r.Lock()
	i := r.rand.Int()
	r.Unlock()
	return uint(i)
}

// It is not safe for cryptographic usage.
func (r *Rand) Int16() int16 {
	return int16(r.Uint32() & (1<<16 - 1))
}

// It is not safe for cryptographic usage.
func (r *Rand) Int32() int32 {
	return int32(r.Uint32())
}

// It is not safe for cryptographic usage.
func (r *Rand) Int64() int64 {
	return int64(r.Uint64())
}

// It is not safe for cryptographic usage.
func (r *Rand) Int() int {
	r.Lock()
	i := r.rand.Int()
	r.Unlock()
	return i
}

// It is not safe for cryptographic usage.
func (r *Rand) Int31() int32 {
	r.Lock()
	i31 := r.rand.Int31()
	r.Unlock()
	return i31
}

// It is not safe for cryptographic usage.
func (r *Rand) Int31n(n int32) int32 {
	r.Lock()
	i31n := r.rand.Int31n(n)
	r.Unlock()
	return i31n
}

// It is not safe for cryptographic usage.
func (r *Rand) Int63() int64 {
	r.Lock()
	i63 := r.rand.Int63()
	r.Unlock()
	return i63
}

// It is not safe for cryptographic usage.
func (r *Rand) Int63n(n int64) int64 {
	r.Lock()
	i63n := r.rand.Int63n(n)
	r.Unlock()
	return i63n
}

// Distributed pseudo-exponentially to test for various cases
// It is not safe for cryptographic usage.
func (r *Rand) Uint16Exp() uint16 {
	bits := r.Uint32() % 16
	if bits == 0 {
		return 0
	}
	n := uint16(1 << (bits - 1))
	n += uint16(r.Int31()) & ((1 << (bits - 1)) - 1)
	return n
}

// Distributed pseudo-exponentially to test for various cases
// It is not safe for cryptographic usage.
func (r *Rand) Uint32Exp() uint32 {
	bits := r.Uint32() % 32
	if bits == 0 {
		return 0
	}
	n := uint32(1 << (bits - 1))
	n += uint32(r.Int31()) & ((1 << (bits - 1)) - 1)
	return n
}

// Distributed pseudo-exponentially to test for various cases
// It is not safe for cryptographic usage.
func (r *Rand) Uint64Exp() uint64 {
	bits := r.Uint32() % 64
	if bits == 0 {
		return 0
	}
	n := uint64(1 << (bits - 1))
	n += uint64(r.Int63()) & ((1 << (bits - 1)) - 1)
	return n
}

// It is not safe for cryptographic usage.
func (r *Rand) Float32() float32 {
	r.Lock()
	f32 := r.rand.Float32()
	r.Unlock()
	return f32
}

// It is not safe for cryptographic usage.
func (r *Rand) Float64() float64 {
	r.Lock()
	f64 := r.rand.Float64()
	r.Unlock()
	return f64
}

// It is not safe for cryptographic usage.
func (r *Rand) Time() time.Time {
	return time.Unix(int64(r.Uint64Exp()), 0)
}

// RandBytes returns n random bytes from the OS's source of entropy ie. via crypto/rand.
// It is not safe for cryptographic usage.
func (r *Rand) Bytes(n int) []byte {
	// cRandBytes isn't guaranteed to be fast so instead
	// use random bytes generated from the internal PRNG
	bs := make([]byte, n)
	for i := 0; i < len(bs); i++ {
		bs[i] = byte(r.Int() & 0xFF)
	}
	return bs
}

// RandIntn returns, as an int, a non-negative pseudo-random number in [0, n).
// It panics if n <= 0.
// It is not safe for cryptographic usage.
func (r *Rand) Intn(n int) int {
	r.Lock()
	i := r.rand.Intn(n)
	r.Unlock()
	return i
}

// RandPerm returns a pseudo-random permutation of n integers in [0, n).
// It is not safe for cryptographic usage.
func (r *Rand) Perm(n int) []int {
	r.Lock()
	perm := r.rand.Perm(n)
	r.Unlock()
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
