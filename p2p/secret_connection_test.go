package p2p

import (
	"bytes"
	"fmt"
	"io"
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
	Parallel(func() {
		var err error
		fooConn, err = MakeSecretConnection(foo, fooPrvKey)
		if err != nil {
			t.Errorf("Failed to establish SecretConnection for foo: %v", err)
			return
		}
		if !bytes.Equal(fooConn.RemotePubKey(), barPubKey) {
			t.Errorf("Unexpected fooConn.RemotePubKey.  Expected %v, got %v",
				barPubKey, fooConn.RemotePubKey())
		}
	}, func() {
		var err error
		barConn, err = MakeSecretConnection(bar, barPrvKey)
		if barConn == nil {
			t.Errorf("Failed to establish SecretConnection for bar: %v", err)
			return
		}
		if !bytes.Equal(barConn.RemotePubKey(), fooPubKey) {
			t.Errorf("Unexpected barConn.RemotePubKey.  Expected %v, got %v",
				fooPubKey, barConn.RemotePubKey())
		}
	})
}

func TestSecretConnectionReadWrite(t *testing.T) {
	foo, bar := makeReadWriterPair()
	fooPrvKey := acm.PrivKeyEd25519(CRandBytes(32))
	barPrvKey := acm.PrivKeyEd25519(CRandBytes(32))
	fooWrites, barWrites := []string{}, []string{}
	fooReads, barReads := []string{}, []string{}

	for i := 0; i < 2; i++ {
		fooWrites = append(fooWrites, RandStr((RandInt()%(dataMaxSize*5))+1))
		barWrites = append(barWrites, RandStr((RandInt()%(dataMaxSize*5))+1))
	}

	fmt.Println("fooWrotes", fooWrites, "\n")
	fmt.Println("barWrotes", barWrites, "\n")

	var fooConn, barConn *SecretConnection
	Parallel(func() {
		var err error
		fooConn, err = MakeSecretConnection(foo, fooPrvKey)
		if err != nil {
			t.Errorf("Failed to establish SecretConnection for foo: %v", err)
			return
		}
		Parallel(func() {
			for _, fooWrite := range fooWrites {
				fmt.Println("will write foo")
				n, err := fooConn.Write([]byte(fooWrite))
				if err != nil {
					t.Errorf("Failed to write to fooConn: %v", err)
					return
				}
				if n != len(fooWrite) {
					t.Errorf("Failed to write all bytes. Expected %v, wrote %v", len(fooWrite), n)
					return
				}
			}
			fmt.Println("Done writing foo")
			// TODO close foo
		}, func() {
			fmt.Println("TODO do foo reads")
		})
	}, func() {
		var err error
		barConn, err = MakeSecretConnection(bar, barPrvKey)
		if err != nil {
			t.Errorf("Failed to establish SecretConnection for bar: %v", err)
			return
		}
		Parallel(func() {
			readBuffer := make([]byte, dataMaxSize)
			for {
				fmt.Println("will read bar")
				n, err := barConn.Read(readBuffer)
				if err == io.EOF {
					return
				} else if err != nil {
					t.Errorf("Failed to read from barConn: %v", err)
					return
				}
				barReads = append(barReads, string(readBuffer[:n]))
			}
			// XXX This does not get called
			fmt.Println("Done reading bar")
		}, func() {
			fmt.Println("TODO do bar writes")
		})
	})

	fmt.Println("fooWrites", fooWrites)
	fmt.Println("barReads", barReads)
	fmt.Println("barWrites", barWrites)
	fmt.Println("fooReads", fooReads)
}

func BenchmarkSecretConnection(b *testing.B) {
	b.StopTimer()
	b.StartTimer()

	for i := 0; i < b.N; i++ {
	}

	b.StopTimer()
}
