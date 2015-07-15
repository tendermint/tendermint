package p2p

import (
	"bytes"
	"io"
	"sync"
	"testing"

	acm "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
)

type dummyReadWriter struct {
	io.Reader
	io.Writer
}

// Each returned ReadWriter is akin to a net.Connection
func makeReadWriterPair() (foo, bar io.ReadWriter) {
	barReader, fooWriter := io.Pipe()
	fooReader, barWriter := io.Pipe()
	return dummyReadWriter{fooReader, fooWriter}, dummyReadWriter{barReader, barWriter}
}

func TestSecretConnectionHandshake(t *testing.T) {
	foo, bar := makeReadWriterPair()
	fooPrvKey := acm.PrivKeyEd25519(CRandBytes(32))
	fooPubKey := fooPrvKey.PubKey().(acm.PubKeyEd25519)
	barPrvKey := acm.PrivKeyEd25519(CRandBytes(32))
	barPubKey := barPrvKey.PubKey().(acm.PubKeyEd25519)

	var fooConn, barConn *SecretConnection
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		var err error
		fooConn, err = MakeSecretConnection(foo, fooPrvKey)
		if err != nil {
			t.Errorf("Failed to establish SecretConnection for foo: %v", err)
			return
		}
		if !bytes.Equal(fooConn.RemotePubKey(), fooPubKey) {
			t.Errorf("Unexpected fooConn.RemotePubKey.  Expected %X, got %X",
				fooPubKey, fooConn.RemotePubKey())
		}
	}()
	go func() {
		defer wg.Done()
		var err error
		barConn, err = MakeSecretConnection(bar, barPrvKey)
		if barConn == nil {
			t.Errorf("Failed to establish SecretConnection for bar: %v", err)
			return
		}
		if !bytes.Equal(barConn.RemotePubKey(), barPubKey) {
			t.Errorf("Unexpected barConn.RemotePubKey.  Expected %X, got %X",
				barPubKey, barConn.RemotePubKey())
		}
	}()
	wg.Wait()
}

func BenchmarkSecretConnection(b *testing.B) {
	b.StopTimer()
	b.StartTimer()

	for i := 0; i < b.N; i++ {
	}

	b.StopTimer()
}
