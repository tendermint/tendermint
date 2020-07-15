package conn

import (
	"bufio"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	"github.com/tendermint/tendermint/libs/async"
	tmos "github.com/tendermint/tendermint/libs/os"
	tmrand "github.com/tendermint/tendermint/libs/rand"
)

// Run go test -update from within this module
// to update the golden test vector file
var update = flag.Bool("update", false, "update .golden files")

type kvstoreConn struct {
	*io.PipeReader
	*io.PipeWriter
}

func (drw kvstoreConn) Close() (err error) {
	err2 := drw.PipeWriter.CloseWithError(io.EOF)
	err1 := drw.PipeReader.Close()
	if err2 != nil {
		return err
	}
	return err1
}

type privKeyWithNilPubKey struct {
	orig crypto.PrivKey
}

func (pk privKeyWithNilPubKey) Bytes() []byte                   { return pk.orig.Bytes() }
func (pk privKeyWithNilPubKey) Sign(msg []byte) ([]byte, error) { return pk.orig.Sign(msg) }
func (pk privKeyWithNilPubKey) PubKey() crypto.PubKey           { return nil }
func (pk privKeyWithNilPubKey) Equals(pk2 crypto.PrivKey) bool  { return pk.orig.Equals(pk2) }
func (pk privKeyWithNilPubKey) Type() string                    { return "privKeyWithNilPubKey" }

func TestSecretConnectionHandshake(t *testing.T) {
	fooSecConn, barSecConn := makeSecretConnPair(t)
	if err := fooSecConn.Close(); err != nil {
		t.Error(err)
	}
	if err := barSecConn.Close(); err != nil {
		t.Error(err)
	}
}

func TestConcurrentWrite(t *testing.T) {
	fooSecConn, barSecConn := makeSecretConnPair(t)
	fooWriteText := tmrand.Str(dataMaxSize)

	// write from two routines.
	// should be safe from race according to net.Conn:
	// https://golang.org/pkg/net/#Conn
	n := 100
	wg := new(sync.WaitGroup)
	wg.Add(3)
	go writeLots(t, wg, fooSecConn, fooWriteText, n)
	go writeLots(t, wg, fooSecConn, fooWriteText, n)

	// Consume reads from bar's reader
	readLots(t, wg, barSecConn, n*2)
	wg.Wait()

	if err := fooSecConn.Close(); err != nil {
		t.Error(err)
	}
}

func TestConcurrentRead(t *testing.T) {
	fooSecConn, barSecConn := makeSecretConnPair(t)
	fooWriteText := tmrand.Str(dataMaxSize)
	n := 100

	// read from two routines.
	// should be safe from race according to net.Conn:
	// https://golang.org/pkg/net/#Conn
	wg := new(sync.WaitGroup)
	wg.Add(3)
	go readLots(t, wg, fooSecConn, n/2)
	go readLots(t, wg, fooSecConn, n/2)

	// write to bar
	writeLots(t, wg, barSecConn, fooWriteText, n)
	wg.Wait()

	if err := fooSecConn.Close(); err != nil {
		t.Error(err)
	}
}

func TestSecretConnectionReadWrite(t *testing.T) {
	fooConn, barConn := makeKVStoreConnPair()
	fooWrites, barWrites := []string{}, []string{}
	fooReads, barReads := []string{}, []string{}

	// Pre-generate the things to write (for foo & bar)
	for i := 0; i < 100; i++ {
		fooWrites = append(fooWrites, tmrand.Str((tmrand.Int()%(dataMaxSize*5))+1))
		barWrites = append(barWrites, tmrand.Str((tmrand.Int()%(dataMaxSize*5))+1))
	}

	// A helper that will run with (fooConn, fooWrites, fooReads) and vice versa
	genNodeRunner := func(id string, nodeConn kvstoreConn, nodeWrites []string, nodeReads *[]string) async.Task {
		return func(_ int) (interface{}, bool, error) {
			// Initiate cryptographic private key and secret connection trhough nodeConn.
			nodePrvKey := ed25519.GenPrivKey()
			nodeSecretConn, err := MakeSecretConnection(nodeConn, nodePrvKey)
			if err != nil {
				t.Errorf("failed to establish SecretConnection for node: %v", err)
				return nil, true, err
			}
			// In parallel, handle some reads and writes.
			var trs, ok = async.Parallel(
				func(_ int) (interface{}, bool, error) {
					// Node writes:
					for _, nodeWrite := range nodeWrites {
						n, err := nodeSecretConn.Write([]byte(nodeWrite))
						if err != nil {
							t.Errorf("failed to write to nodeSecretConn: %v", err)
							return nil, true, err
						}
						if n != len(nodeWrite) {
							err = fmt.Errorf("failed to write all bytes. Expected %v, wrote %v", len(nodeWrite), n)
							t.Error(err)
							return nil, true, err
						}
					}
					if err := nodeConn.PipeWriter.Close(); err != nil {
						t.Error(err)
						return nil, true, err
					}
					return nil, false, nil
				},
				func(_ int) (interface{}, bool, error) {
					// Node reads:
					readBuffer := make([]byte, dataMaxSize)
					for {
						n, err := nodeSecretConn.Read(readBuffer)
						if err == io.EOF {
							if err := nodeConn.PipeReader.Close(); err != nil {
								t.Error(err)
								return nil, true, err
							}
							return nil, false, nil
						} else if err != nil {
							t.Errorf("failed to read from nodeSecretConn: %v", err)
							return nil, true, err
						}
						*nodeReads = append(*nodeReads, string(readBuffer[:n]))
					}
				},
			)
			assert.True(t, ok, "Unexpected task abortion")

			// If error:
			if trs.FirstError() != nil {
				return nil, true, trs.FirstError()
			}

			// Otherwise:
			return nil, false, nil
		}
	}

	// Run foo & bar in parallel
	var trs, ok = async.Parallel(
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
				t.Errorf("expected to read %X, got %X", write, read)
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

func TestDeriveSecretsAndChallengeGolden(t *testing.T) {
	goldenFilepath := filepath.Join("testdata", t.Name()+".golden")
	if *update {
		t.Logf("Updating golden test vector file %s", goldenFilepath)
		data := createGoldenTestVectors(t)
		err := tmos.WriteFile(goldenFilepath, []byte(data), 0644)
		require.NoError(t, err)
	}
	f, err := os.Open(goldenFilepath)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		params := strings.Split(line, ",")
		randSecretVector, err := hex.DecodeString(params[0])
		require.Nil(t, err)
		randSecret := new([32]byte)
		copy((*randSecret)[:], randSecretVector)
		locIsLeast, err := strconv.ParseBool(params[1])
		require.Nil(t, err)
		expectedRecvSecret, err := hex.DecodeString(params[2])
		require.Nil(t, err)
		expectedSendSecret, err := hex.DecodeString(params[3])
		require.Nil(t, err)

		recvSecret, sendSecret := deriveSecrets(randSecret, locIsLeast)
		require.Equal(t, expectedRecvSecret, (*recvSecret)[:], "Recv Secrets aren't equal")
		require.Equal(t, expectedSendSecret, (*sendSecret)[:], "Send Secrets aren't equal")
	}
}

func TestNilPubkey(t *testing.T) {
	var fooConn, barConn = makeKVStoreConnPair()
	defer fooConn.Close()
	defer barConn.Close()
	var fooPrvKey = ed25519.GenPrivKey()
	var barPrvKey = privKeyWithNilPubKey{ed25519.GenPrivKey()}

	go MakeSecretConnection(fooConn, fooPrvKey) //nolint:errcheck // ignore for tests

	_, err := MakeSecretConnection(barConn, barPrvKey)
	require.Error(t, err)
	assert.Equal(t, "toproto: key type <nil> is not supported", err.Error())
}

func TestNonEd25519Pubkey(t *testing.T) {
	var fooConn, barConn = makeKVStoreConnPair()
	defer fooConn.Close()
	defer barConn.Close()
	var fooPrvKey = ed25519.GenPrivKey()
	var barPrvKey = secp256k1.GenPrivKey()

	go MakeSecretConnection(fooConn, fooPrvKey) //nolint:errcheck // ignore for tests

	_, err := MakeSecretConnection(barConn, barPrvKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not supported")
}

func writeLots(t *testing.T, wg *sync.WaitGroup, conn io.Writer, txt string, n int) {
	defer wg.Done()
	for i := 0; i < n; i++ {
		_, err := conn.Write([]byte(txt))
		if err != nil {
			t.Errorf("failed to write to fooSecConn: %v", err)
			return
		}
	}
}

func readLots(t *testing.T, wg *sync.WaitGroup, conn io.Reader, n int) {
	readBuffer := make([]byte, dataMaxSize)
	for i := 0; i < n; i++ {
		_, err := conn.Read(readBuffer)
		assert.NoError(t, err)
	}
	wg.Done()
}

// Creates the data for a test vector file.
// The file format is:
// Hex(diffie_hellman_secret), loc_is_least, Hex(recvSecret), Hex(sendSecret), Hex(challenge)
func createGoldenTestVectors(t *testing.T) string {
	data := ""
	for i := 0; i < 32; i++ {
		randSecretVector := tmrand.Bytes(32)
		randSecret := new([32]byte)
		copy((*randSecret)[:], randSecretVector)
		data += hex.EncodeToString((*randSecret)[:]) + ","
		locIsLeast := tmrand.Bool()
		data += strconv.FormatBool(locIsLeast) + ","
		recvSecret, sendSecret := deriveSecrets(randSecret, locIsLeast)
		data += hex.EncodeToString((*recvSecret)[:]) + ","
		data += hex.EncodeToString((*sendSecret)[:]) + ","
	}
	return data
}

// Each returned ReadWriteCloser is akin to a net.Connection
func makeKVStoreConnPair() (fooConn, barConn kvstoreConn) {
	barReader, fooWriter := io.Pipe()
	fooReader, barWriter := io.Pipe()
	return kvstoreConn{fooReader, fooWriter}, kvstoreConn{barReader, barWriter}
}

func makeSecretConnPair(tb testing.TB) (fooSecConn, barSecConn *SecretConnection) {
	var (
		fooConn, barConn = makeKVStoreConnPair()
		fooPrvKey        = ed25519.GenPrivKey()
		fooPubKey        = fooPrvKey.PubKey()
		barPrvKey        = ed25519.GenPrivKey()
		barPubKey        = barPrvKey.PubKey()
	)

	// Make connections from both sides in parallel.
	var trs, ok = async.Parallel(
		func(_ int) (val interface{}, abort bool, err error) {
			fooSecConn, err = MakeSecretConnection(fooConn, fooPrvKey)
			if err != nil {
				tb.Errorf("failed to establish SecretConnection for foo: %v", err)
				return nil, true, err
			}
			remotePubBytes := fooSecConn.RemotePubKey()
			if !remotePubBytes.Equals(barPubKey) {
				err = fmt.Errorf("unexpected fooSecConn.RemotePubKey.  Expected %v, got %v",
					barPubKey, fooSecConn.RemotePubKey())
				tb.Error(err)
				return nil, true, err
			}
			return nil, false, nil
		},
		func(_ int) (val interface{}, abort bool, err error) {
			barSecConn, err = MakeSecretConnection(barConn, barPrvKey)
			if barSecConn == nil {
				tb.Errorf("failed to establish SecretConnection for bar: %v", err)
				return nil, true, err
			}
			remotePubBytes := barSecConn.RemotePubKey()
			if !remotePubBytes.Equals(fooPubKey) {
				err = fmt.Errorf("unexpected barSecConn.RemotePubKey.  Expected %v, got %v",
					fooPubKey, barSecConn.RemotePubKey())
				tb.Error(err)
				return nil, true, err
			}
			return nil, false, nil
		},
	)

	require.Nil(tb, trs.FirstError())
	require.True(tb, ok, "Unexpected task abortion")

	return fooSecConn, barSecConn
}

///////////////////////////////////////////////////////////////////////////////
// Benchmarks

func BenchmarkWriteSecretConnection(b *testing.B) {
	b.StopTimer()
	b.ReportAllocs()
	fooSecConn, barSecConn := makeSecretConnPair(b)
	randomMsgSizes := []int{
		dataMaxSize / 10,
		dataMaxSize / 3,
		dataMaxSize / 2,
		dataMaxSize,
		dataMaxSize * 3 / 2,
		dataMaxSize * 2,
		dataMaxSize * 7 / 2,
	}
	fooWriteBytes := make([][]byte, 0, len(randomMsgSizes))
	for _, size := range randomMsgSizes {
		fooWriteBytes = append(fooWriteBytes, tmrand.Bytes(size))
	}
	// Consume reads from bar's reader
	go func() {
		readBuffer := make([]byte, dataMaxSize)
		for {
			_, err := barSecConn.Read(readBuffer)
			if err == io.EOF {
				return
			} else if err != nil {
				b.Errorf("failed to read from barSecConn: %v", err)
				return
			}
		}
	}()

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		idx := tmrand.Intn(len(fooWriteBytes))
		_, err := fooSecConn.Write(fooWriteBytes[idx])
		if err != nil {
			b.Errorf("failed to write to fooSecConn: %v", err)
			return
		}
	}
	b.StopTimer()

	if err := fooSecConn.Close(); err != nil {
		b.Error(err)
	}
	//barSecConn.Close() race condition
}

func BenchmarkReadSecretConnection(b *testing.B) {
	b.StopTimer()
	b.ReportAllocs()
	fooSecConn, barSecConn := makeSecretConnPair(b)
	randomMsgSizes := []int{
		dataMaxSize / 10,
		dataMaxSize / 3,
		dataMaxSize / 2,
		dataMaxSize,
		dataMaxSize * 3 / 2,
		dataMaxSize * 2,
		dataMaxSize * 7 / 2,
	}
	fooWriteBytes := make([][]byte, 0, len(randomMsgSizes))
	for _, size := range randomMsgSizes {
		fooWriteBytes = append(fooWriteBytes, tmrand.Bytes(size))
	}
	go func() {
		for i := 0; i < b.N; i++ {
			idx := tmrand.Intn(len(fooWriteBytes))
			_, err := fooSecConn.Write(fooWriteBytes[idx])
			if err != nil {
				b.Errorf("failed to write to fooSecConn: %v, %v,%v", err, i, b.N)
				return
			}
		}
	}()

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		readBuffer := make([]byte, dataMaxSize)
		_, err := barSecConn.Read(readBuffer)

		if err == io.EOF {
			return
		} else if err != nil {
			b.Fatalf("Failed to read from barSecConn: %v", err)
		}
	}
	b.StopTimer()
}
