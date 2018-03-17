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
	*mrand.Rand
}

var grand *Rand

func init() {
	grand = New()
	grand.init()
}

func New() *Rand {
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
	r.Rand = mrand.New(mrand.NewSource(seed))
}

//----------------------------------------
// Global functions

func Seed(seed int64) {
	grand.Seed(seed)
}

func RandStr(length int) string {
	return grand.RandStr(length)
}

func RandUint16() uint16 {
	return grand.RandUint16()
}

func RandUint32() uint32 {
	return grand.RandUint32()
}

func RandUint64() uint64 {
	return grand.RandUint64()
}

func RandUint() uint {
	return grand.RandUint()
}

func RandInt16() int16 {
	return grand.RandInt16()
}

func RandInt32() int32 {
	return grand.RandInt32()
}

func RandInt64() int64 {
	return grand.RandInt64()
}

func RandInt() int {
	return grand.RandInt()
}

func RandInt31() int32 {
	return grand.RandInt31()
}

func RandInt63() int64 {
	return grand.RandInt63()
}

func RandUint16Exp() uint16 {
	return grand.RandUint16Exp()
}

func RandUint32Exp() uint32 {
	return grand.RandUint32Exp()
}

func RandUint64Exp() uint64 {
	return grand.RandUint64Exp()
}

func RandFloat32() float32 {
	return grand.RandFloat32()
}

func RandTime() time.Time {
	return grand.RandTime()
}

func RandBytes(n int) []byte {
	return grand.RandBytes(n)
}

func RandIntn(n int) int {
	return grand.RandIntn(n)
}

func RandPerm(n int) []int {
	return grand.RandPerm(n)
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
func (r *Rand) RandStr(length int) string {
	chars := []byte{}
MAIN_LOOP:
	for {
		val := r.RandInt63()
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
func (r *Rand) RandUint16() uint16 {
	return uint16(r.RandUint32() & (1<<16 - 1))
}

// It is not safe for cryptographic usage.
func (r *Rand) RandUint32() uint32 {
	r.Lock()
	u32 := r.Uint32()
	r.Unlock()
	return u32
}

// It is not safe for cryptographic usage.
func (r *Rand) RandUint64() uint64 {
	return uint64(r.RandUint32())<<32 + uint64(r.RandUint32())
}

// It is not safe for cryptographic usage.
func (r *Rand) RandUint() uint {
	r.Lock()
	i := r.Int()
	r.Unlock()
	return uint(i)
}

// It is not safe for cryptographic usage.
func (r *Rand) RandInt16() int16 {
	return int16(r.RandUint32() & (1<<16 - 1))
}

// It is not safe for cryptographic usage.
func (r *Rand) RandInt32() int32 {
	return int32(r.RandUint32())
}

// It is not safe for cryptographic usage.
func (r *Rand) RandInt64() int64 {
	return int64(r.RandUint64())
}

// It is not safe for cryptographic usage.
func (r *Rand) RandInt() int {
	r.Lock()
	i := r.Int()
	r.Unlock()
	return i
}

// It is not safe for cryptographic usage.
func (r *Rand) RandInt31() int32 {
	r.Lock()
	i31 := r.Int31()
	r.Unlock()
	return i31
}

// It is not safe for cryptographic usage.
func (r *Rand) RandInt63() int64 {
	r.Lock()
	i63 := r.Int63()
	r.Unlock()
	return i63
}

// Distributed pseudo-exponentially to test for various cases
// It is not safe for cryptographic usage.
func (r *Rand) RandUint16Exp() uint16 {
	bits := r.RandUint32() % 16
	if bits == 0 {
		return 0
	}
	n := uint16(1 << (bits - 1))
	n += uint16(r.RandInt31()) & ((1 << (bits - 1)) - 1)
	return n
}

// Distributed pseudo-exponentially to test for various cases
// It is not safe for cryptographic usage.
func (r *Rand) RandUint32Exp() uint32 {
	bits := r.RandUint32() % 32
	if bits == 0 {
		return 0
	}
	n := uint32(1 << (bits - 1))
	n += uint32(r.RandInt31()) & ((1 << (bits - 1)) - 1)
	return n
}

// Distributed pseudo-exponentially to test for various cases
// It is not safe for cryptographic usage.
func (r *Rand) RandUint64Exp() uint64 {
	bits := r.RandUint32() % 64
	if bits == 0 {
		return 0
	}
	n := uint64(1 << (bits - 1))
	n += uint64(r.RandInt63()) & ((1 << (bits - 1)) - 1)
	return n
}

// It is not safe for cryptographic usage.
func (r *Rand) RandFloat32() float32 {
	r.Lock()
	f32 := r.Float32()
	r.Unlock()
	return f32
}

// It is not safe for cryptographic usage.
func (r *Rand) RandTime() time.Time {
	return time.Unix(int64(r.RandUint64Exp()), 0)
}

// RandBytes returns n random bytes from the OS's source of entropy ie. via crypto/rand.
// It is not safe for cryptographic usage.
func (r *Rand) RandBytes(n int) []byte {
	// cRandBytes isn't guaranteed to be fast so instead
	// use random bytes generated from the internal PRNG
	bs := make([]byte, n)
	for i := 0; i < len(bs); i++ {
		bs[i] = byte(r.RandInt() & 0xFF)
	}
	return bs
}

// RandIntn returns, as an int, a non-negative pseudo-random number in [0, n).
// It panics if n <= 0.
// It is not safe for cryptographic usage.
func (r *Rand) RandIntn(n int) int {
	r.Lock()
	i := r.Intn(n)
	r.Unlock()
	return i
}

// RandPerm returns a pseudo-random permutation of n integers in [0, n).
// It is not safe for cryptographic usage.
func (r *Rand) RandPerm(n int) []int {
	r.Lock()
	perm := r.Perm(n)
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
