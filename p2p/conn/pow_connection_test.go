package conn

import (
	"crypto/sha256"
	"encoding"
	"fmt"
	"io"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
	cmn "github.com/tendermint/tmlibs/common"
)

func makePoWConnPair(tb testing.TB) (fooPowConn, barPowConn *PowConnection) {

	var fooConn, barConn = makeKVStoreConnPair()
	minDifficulty, maxDifficulty := int8(5), int8(10)

	// Make connections from both sides in parallel.
	var trs, ok = cmn.Parallel(
		func(_ int) (val interface{}, err error, abort bool) {
			fooPowConn, err = MakePowConnection(fooConn, minDifficulty, maxDifficulty)
			if err != nil {
				tb.Errorf("Failed to establish PoWConnection for foo: %v", err)
				return nil, err, true
			}
			return nil, nil, false
		},
		func(_ int) (val interface{}, err error, abort bool) {
			barPowConn, err = MakePowConnection(barConn, minDifficulty, maxDifficulty)
			if barPowConn == nil {
				tb.Errorf("Failed to establish SecretConnection for bar: %v", err)
				return nil, err, true
			}
			return nil, nil, false
		},
	)

	require.Nil(tb, trs.FirstError())
	require.True(tb, ok, "Unexpected task abortion")

	return
}

func TestPowHandshake(t *testing.T) {
	fooPowConn, barPowConn := makeSecretConnPair(t)
	if err := fooPowConn.Close(); err != nil {
		t.Error(err)
	}
	if err := barPowConn.Close(); err != nil {
		t.Error(err)
	}
}

func TestPowConnectionReadWrite(t *testing.T) {
	fooConn, barConn := makeKVStoreConnPair()
	fooWrites, barWrites := []string{}, []string{}
	fooReads, barReads := []string{}, []string{}
	minDifficulty, maxDifficulty := int8(5), int8(10)

	// Pre-generate the things to write (for foo & bar)
	for i := 0; i < 100; i++ {
		fooWrites = append(fooWrites, cmn.RandStr((cmn.RandInt()%(dataMaxSize*5))+1))
		barWrites = append(barWrites, cmn.RandStr((cmn.RandInt()%(dataMaxSize*5))+1))
	}

	// A helper that will run with (fooConn, fooWrites, fooReads) and vice versa
	genNodeRunner := func(id string, nodeConn kvstoreConn, nodeWrites []string, nodeReads *[]string) cmn.Task {
		return func(_ int) (interface{}, error, bool) {
			nodePowConn, err := MakePowConnection(nodeConn, minDifficulty, maxDifficulty)
			if err != nil {
				t.Errorf("Failed to establish SecretConnection for node: %v", err)
				return nil, err, true
			}
			// In parallel, handle some reads and writes.
			var trs, ok = cmn.Parallel(
				func(_ int) (interface{}, error, bool) {
					// Node writes:
					for _, nodeWrite := range nodeWrites {
						n, err := nodePowConn.Write([]byte(nodeWrite))
						if err != nil {
							t.Errorf("Failed to write to nodeSecretConn: %v", err)
							return nil, err, true
						}
						if n != len(nodeWrite) {
							err = fmt.Errorf("Failed to write all bytes. Expected %v, wrote %v", len(nodeWrite), n)
							t.Error(err)
							return nil, err, true
						}
					}
					if err := nodeConn.PipeWriter.Close(); err != nil {
						t.Error(err)
						return nil, err, true
					}
					return nil, nil, false
				},
				func(_ int) (interface{}, error, bool) {
					// Node reads:
					readBuffer := make([]byte, dataMaxSize)
					for {
						n, err := nodePowConn.Read(readBuffer)
						if err == io.EOF {
							return nil, nil, false
						} else if err != nil {
							t.Errorf("Failed to read from nodeSecretConn: %v", err)
							return nil, err, true
						}
						*nodeReads = append(*nodeReads, string(readBuffer[:n]))
					}
					if err := nodeConn.PipeReader.Close(); err != nil {
						t.Error(err)
						return nil, err, true
					}
					return nil, nil, false
				},
			)
			require.True(t, ok, "Unexpected task abortion")

			// If error:
			if trs.FirstError() != nil {
				return nil, trs.FirstError(), true
			}

			// Otherwise:
			return nil, nil, false
		}
	}

	// Run foo & bar in parallel
	var trs, ok = cmn.Parallel(
		genNodeRunner("foo", fooConn, fooWrites, &fooReads),
		genNodeRunner("bar", barConn, barWrites, &barReads),
	)
	require.Nil(t, trs.FirstError())
	require.True(t, ok, "unexpected task abortion")

	// A helper to ensure that the writes and reads match.
	// Additionally, small writes (<= dataMaxSize) must be atomically read.
	compareWritesReads := func(writes []string, reads []string) {
		for {
			// Pop next write & corresponding reads
			var read, write string = "", writes[0]
			var readCount = 0
			for _, readChunk := range reads {
				read += readChunk
				readCount++
				if len(write) <= len(read) {
					break
				}
				if len(write) <= dataMaxSize {
					break // atomicity of small writes
				}
			}
			// Compare
			if write != read {
				t.Errorf("Expected to read %X, got %X", write, read)
			}
			// Iterate
			writes = writes[1:]
			reads = reads[readCount:]
			if len(writes) == 0 {
				break
			}
		}
	}

	compareWritesReads(fooWrites, barReads)
	compareWritesReads(barWrites, fooReads)

}

// block size is 512 bits, so reusing the state with the nonce doesn't impact much
func BenchmarkPoWWithStateReuse(b *testing.B) {
	difficulty := int8(10)

	for i := 0; i < b.N; i++ {
		nonce := new([32]byte)
		rand.Read(nonce[:])
		hasher := sha256.New()
		hasher.Write(nonce[:])
		nonceState, _ := hasher.(encoding.BinaryMarshaler).MarshalBinary()
		counter := new([32]byte)
		for {
			hasher.(encoding.BinaryUnmarshaler).UnmarshalBinary(nonceState)
			hasher.Write(counter[:])
			sum := hasher.Sum(nil)
			if validatePoWHash(difficulty, sum) {
				break
			}
			incrCounter(counter)
		}
	}
}

func BenchmarkPoWWithNoStateReuse(b *testing.B) {
	difficulty := int8(10)

	for i := 0; i < b.N; i++ {
		nonce := new([32]byte)
		rand.Read(nonce[:])
		counter := new([32]byte)
		for {
			if validatePoW(difficulty, nonce, counter) {
				break
			}
			incrCounter(counter)
		}
	}
}
