package crypto

import (
	"crypto/cipher"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"sync"

	"golang.org/x/crypto/chacha20poly1305"
)

// NOTE: This is ignored for now until we have time
// to properly review the MixEntropy function - https://github.com/tendermint/tendermint/issues/2099.
//
// The randomness here is derived from xoring a chacha20 keystream with
// output from crypto/rand's OS Entropy Reader. (Due to fears of the OS'
// entropy being backdoored)
//
// For forward secrecy of produced randomness, the internal chacha key is hashed
// and thereby rotated after each call.
var gRandInfo *randInfo

func init() {
	gRandInfo = &randInfo{}

	// TODO: uncomment after reviewing MixEntropy -
	// https://github.com/tendermint/tendermint/issues/2099
	// gRandInfo.MixEntropy(randBytes(32)) // Init
}

// WARNING: This function needs review - https://github.com/tendermint/tendermint/issues/2099.
// Mix additional bytes of randomness, e.g. from hardware, user-input, etc.
// It is OK to call it multiple times.  It does not diminish security.
func MixEntropy(seedBytes []byte) {
	gRandInfo.MixEntropy(seedBytes)
}

// This only uses the OS's randomness
func randBytes(numBytes int) []byte {
	b := make([]byte, numBytes)
	_, err := crand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

// This only uses the OS's randomness
func CRandBytes(numBytes int) []byte {
	return randBytes(numBytes)
}

/* TODO: uncomment after reviewing MixEntropy - https://github.com/tendermint/tendermint/issues/2099
// This uses the OS and the Seed(s).
func CRandBytes(numBytes int) []byte {
	return randBytes(numBytes)
		b := make([]byte, numBytes)
		_, err := gRandInfo.Read(b)
		if err != nil {
			panic(err)
		}
		return b
}
*/

// CRandHex returns a hex encoded string that's floor(numDigits/2) * 2 long.
//
// Note: CRandHex(24) gives 96 bits of randomness that
// are usually strong enough for most purposes.
func CRandHex(numDigits int) string {
	return hex.EncodeToString(CRandBytes(numDigits / 2))
}

// Returns a crand.Reader.
func CReader() io.Reader {
	return crand.Reader
}

/* TODO: uncomment after reviewing MixEntropy - https://github.com/tendermint/tendermint/issues/2099
// Returns a crand.Reader mixed with user-supplied entropy
func CReader() io.Reader {
	return gRandInfo
}
*/

//--------------------------------------------------------------------------------

type randInfo struct {
	mtx       sync.Mutex
	seedBytes [chacha20poly1305.KeySize]byte
	chacha    cipher.AEAD
	reader    io.Reader
}

// You can call this as many times as you'd like.
// XXX/TODO: review - https://github.com/tendermint/tendermint/issues/2099
func (ri *randInfo) MixEntropy(seedBytes []byte) {
	ri.mtx.Lock()
	defer ri.mtx.Unlock()
	// Make new ri.seedBytes using passed seedBytes and current ri.seedBytes:
	// ri.seedBytes = sha256( seedBytes || ri.seedBytes )
	h := sha256.New()
	h.Write(seedBytes)
	h.Write(ri.seedBytes[:])
	hashBytes := h.Sum(nil)
	copy(ri.seedBytes[:], hashBytes)
	chacha, err := chacha20poly1305.New(ri.seedBytes[:])
	if err != nil {
		panic("Initializing chacha20 failed")
	}
	ri.chacha = chacha
	// Create new reader
	ri.reader = &cipher.StreamReader{S: ri, R: crand.Reader}
}

func (ri *randInfo) XORKeyStream(dst, src []byte) {
	// nonce being 0 is safe due to never re-using a key.
	emptyNonce := make([]byte, 12)
	tmpDst := ri.chacha.Seal([]byte{}, emptyNonce, src, []byte{0})
	// this removes the poly1305 tag as well, since chacha is a stream cipher
	// and we truncate at input length.
	copy(dst, tmpDst[:len(src)])
	// hash seedBytes for forward secrecy, and initialize new chacha instance
	newSeed := sha256.Sum256(ri.seedBytes[:])
	chacha, err := chacha20poly1305.New(newSeed[:])
	if err != nil {
		panic("Initializing chacha20 failed")
	}
	ri.chacha = chacha
}

func (ri *randInfo) Read(b []byte) (n int, err error) {
	ri.mtx.Lock()
	n, err = ri.reader.Read(b)
	ri.mtx.Unlock()
	return
}
