package client_test

import (
	"math/rand"

	meapp "github.com/tendermint/merkleeyes/app"

	wire "github.com/tendermint/go-wire"
)

// MakeTxKV returns a text transaction, allong with expected key, value pair
func MakeTxKV() ([]byte, []byte, []byte) {
	k := RandAsciiBytes(8)
	v := RandAsciiBytes(8)
	return k, v, makeSet(k, v)
}

// blatently copied from merkleeyes/app/app_test.go
// constructs a "set" transaction
func makeSet(key, value []byte) []byte {
	tx := make([]byte, 1+wire.ByteSliceSize(key)+wire.ByteSliceSize(value))
	buf := tx
	buf[0] = meapp.WriteSet // Set TypeByte
	buf = buf[1:]
	n, err := wire.PutByteSlice(buf, key)
	if err != nil {
		panic(err)
	}
	buf = buf[n:]
	n, err = wire.PutByteSlice(buf, value)
	if err != nil {
		panic(err)
	}
	return tx
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandAsciiBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return b
}
