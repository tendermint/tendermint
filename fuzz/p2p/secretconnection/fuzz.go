package secretconnection

import (
	"bytes"
	"fmt"
	"io"

	"github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/p2p"
)

var privKey = crypto.GenPrivKeyEd25519()

type rwNopCloser struct {
	io.ReadWriter
}

func (rwn *rwNopCloser) Close() error { return nil }

func Fuzz(data []byte) int {
	rwc := &rwNopCloser{new(bytes.Buffer)}
	sc, err := p2p.MakeSecretConnection(rwc, privKey)
	if err != nil {
		panic(fmt.Errorf("for some reason the connection making failed, err: %v", err))
	}
	defer sc.Close()
	n, err := sc.Write(data)
	if err != nil {
		panic(err)
	}
	if g, w := n, len(data); g != w {
		panic(fmt.Errorf("n: got %d; want %d", g, w))
	}
	return 1
}
