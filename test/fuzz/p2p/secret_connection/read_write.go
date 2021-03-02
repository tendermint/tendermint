package secretconnection

import (
	"bytes"
	"fmt"
	"io"
	"log"

	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/async"
	sc "github.com/tendermint/tendermint/p2p/conn"
)

func Fuzz(data []byte) int {
	if len(data) == 0 {
		return -1
	}

	fooConn, barConn := makeSecretConnPair()

	// Run Write in a separate goroutine because if data is greater than 1024
	// bytes, each Write must be followed by Read (see io.Pipe documentation).
	go func() {
		// Copy data because Write modifies the slice.
		dataToWrite := make([]byte, len(data))
		copy(dataToWrite, data)

		n, err := fooConn.Write(dataToWrite)
		if err != nil {
			panic(err)
		}
		if n < len(data) {
			panic(fmt.Sprintf("wanted to write %d bytes, but %d was written", len(data), n))
		}
	}()

	dataRead := make([]byte, len(data))
	totalRead := 0
	for totalRead < len(data) {
		buf := make([]byte, len(data)-totalRead)
		m, err := barConn.Read(buf)
		if err != nil {
			panic(err)
		}
		copy(dataRead[totalRead:], buf[:m])
		totalRead += m
		log.Printf("total read: %d", totalRead)
	}

	if !bytes.Equal(data, dataRead) {
		panic("bytes written != read")
	}

	return 1
}

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

// Each returned ReadWriteCloser is akin to a net.Connection
func makeKVStoreConnPair() (fooConn, barConn kvstoreConn) {
	barReader, fooWriter := io.Pipe()
	fooReader, barWriter := io.Pipe()
	return kvstoreConn{fooReader, fooWriter}, kvstoreConn{barReader, barWriter}
}

func makeSecretConnPair() (fooSecConn, barSecConn *sc.SecretConnection) {
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
			fooSecConn, err = sc.MakeSecretConnection(fooConn, fooPrvKey)
			if err != nil {
				log.Printf("failed to establish SecretConnection for foo: %v", err)
				return nil, true, err
			}
			remotePubBytes := fooSecConn.RemotePubKey()
			if !remotePubBytes.Equals(barPubKey) {
				err = fmt.Errorf("unexpected fooSecConn.RemotePubKey.  Expected %v, got %v",
					barPubKey, fooSecConn.RemotePubKey())
				log.Print(err)
				return nil, true, err
			}
			return nil, false, nil
		},
		func(_ int) (val interface{}, abort bool, err error) {
			barSecConn, err = sc.MakeSecretConnection(barConn, barPrvKey)
			if barSecConn == nil {
				log.Printf("failed to establish SecretConnection for bar: %v", err)
				return nil, true, err
			}
			remotePubBytes := barSecConn.RemotePubKey()
			if !remotePubBytes.Equals(fooPubKey) {
				err = fmt.Errorf("unexpected barSecConn.RemotePubKey.  Expected %v, got %v",
					fooPubKey, barSecConn.RemotePubKey())
				log.Print(err)
				return nil, true, err
			}
			return nil, false, nil
		},
	)

	if trs.FirstError() != nil {
		log.Fatalf("unexpected error: %v", trs.FirstError())
	}
	if !ok {
		log.Fatal("Unexpected task abortion")
	}

	return fooSecConn, barSecConn
}
