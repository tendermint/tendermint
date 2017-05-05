package p2p

import (
	"bytes"
	"io"
	"testing"

	"github.com/tendermint/go-crypto"
	cmn "github.com/tendermint/tmlibs/common"
)

type dummyConn struct {
	*io.PipeReader
	*io.PipeWriter
}

func (drw dummyConn) Close() (err error) {
	err2 := drw.PipeWriter.CloseWithError(io.EOF)
	err1 := drw.PipeReader.Close()
	if err2 != nil {
		return err
	}
	return err1
}

// Each returned ReadWriteCloser is akin to a net.Connection
func makeDummyConnPair() (fooConn, barConn dummyConn) {
	barReader, fooWriter := io.Pipe()
	fooReader, barWriter := io.Pipe()
	return dummyConn{fooReader, fooWriter}, dummyConn{barReader, barWriter}
}

func makeSecretConnPair(tb testing.TB) (fooSecConn, barSecConn *SecretConnection) {
	fooConn, barConn := makeDummyConnPair()
	fooPrvKey := crypto.GenPrivKeyEd25519()
	fooPubKey := fooPrvKey.PubKey().Unwrap().(crypto.PubKeyEd25519)
	barPrvKey := crypto.GenPrivKeyEd25519()
	barPubKey := barPrvKey.PubKey().Unwrap().(crypto.PubKeyEd25519)

	cmn.Parallel(
		func() {
			var err error
			fooSecConn, err = MakeSecretConnection(fooConn, fooPrvKey)
			if err != nil {
				tb.Errorf("Failed to establish SecretConnection for foo: %v", err)
				return
			}
			remotePubBytes := fooSecConn.RemotePubKey()
			if !bytes.Equal(remotePubBytes[:], barPubKey[:]) {
				tb.Errorf("Unexpected fooSecConn.RemotePubKey.  Expected %v, got %v",
					barPubKey, fooSecConn.RemotePubKey())
			}
		},
		func() {
			var err error
			barSecConn, err = MakeSecretConnection(barConn, barPrvKey)
			if barSecConn == nil {
				tb.Errorf("Failed to establish SecretConnection for bar: %v", err)
				return
			}
			remotePubBytes := barSecConn.RemotePubKey()
			if !bytes.Equal(remotePubBytes[:], fooPubKey[:]) {
				tb.Errorf("Unexpected barSecConn.RemotePubKey.  Expected %v, got %v",
					fooPubKey, barSecConn.RemotePubKey())
			}
		})

	return
}

func TestSecretConnectionHandshake(t *testing.T) {
	fooSecConn, barSecConn := makeSecretConnPair(t)
	fooSecConn.Close()
	barSecConn.Close()
}

func TestSecretConnectionReadWrite(t *testing.T) {
	fooConn, barConn := makeDummyConnPair()
	fooWrites, barWrites := []string{}, []string{}
	fooReads, barReads := []string{}, []string{}

	// Pre-generate the things to write (for foo & bar)
	for i := 0; i < 100; i++ {
		fooWrites = append(fooWrites, cmn.RandStr((cmn.RandInt()%(dataMaxSize*5))+1))
		barWrites = append(barWrites, cmn.RandStr((cmn.RandInt()%(dataMaxSize*5))+1))
	}

	// A helper that will run with (fooConn, fooWrites, fooReads) and vice versa
	genNodeRunner := func(nodeConn dummyConn, nodeWrites []string, nodeReads *[]string) func() {
		return func() {
			// Node handskae
			nodePrvKey := crypto.GenPrivKeyEd25519()
			nodeSecretConn, err := MakeSecretConnection(nodeConn, nodePrvKey)
			if err != nil {
				t.Errorf("Failed to establish SecretConnection for node: %v", err)
				return
			}
			// In parallel, handle reads and writes
			cmn.Parallel(
				func() {
					// Node writes
					for _, nodeWrite := range nodeWrites {
						n, err := nodeSecretConn.Write([]byte(nodeWrite))
						if err != nil {
							t.Errorf("Failed to write to nodeSecretConn: %v", err)
							return
						}
						if n != len(nodeWrite) {
							t.Errorf("Failed to write all bytes. Expected %v, wrote %v", len(nodeWrite), n)
							return
						}
					}
					nodeConn.PipeWriter.Close()
				},
				func() {
					// Node reads
					readBuffer := make([]byte, dataMaxSize)
					for {
						n, err := nodeSecretConn.Read(readBuffer)
						if err == io.EOF {
							return
						} else if err != nil {
							t.Errorf("Failed to read from nodeSecretConn: %v", err)
							return
						}
						*nodeReads = append(*nodeReads, string(readBuffer[:n]))
					}
					nodeConn.PipeReader.Close()
				})
		}
	}

	// Run foo & bar in parallel
	cmn.Parallel(
		genNodeRunner(fooConn, fooWrites, &fooReads),
		genNodeRunner(barConn, barWrites, &barReads),
	)

	// A helper to ensure that the writes and reads match.
	// Additionally, small writes (<= dataMaxSize) must be atomically read.
	compareWritesReads := func(writes []string, reads []string) {
		for {
			// Pop next write & corresponding reads
			var read, write string = "", writes[0]
			var readCount = 0
			for _, readChunk := range reads {
				read += readChunk
				readCount += 1
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

func BenchmarkSecretConnection(b *testing.B) {
	b.StopTimer()
	fooSecConn, barSecConn := makeSecretConnPair(b)
	fooWriteText := cmn.RandStr(dataMaxSize)
	// Consume reads from bar's reader
	go func() {
		readBuffer := make([]byte, dataMaxSize)
		for {
			_, err := barSecConn.Read(readBuffer)
			if err == io.EOF {
				return
			} else if err != nil {
				b.Fatalf("Failed to read from barSecConn: %v", err)
			}
		}
	}()

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, err := fooSecConn.Write([]byte(fooWriteText))
		if err != nil {
			b.Fatalf("Failed to write to fooSecConn: %v", err)
		}
	}
	b.StopTimer()

	fooSecConn.Close()
	//barSecConn.Close() race condition
}
