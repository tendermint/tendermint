package merkle

import (
	"github.com/tendermint/tendermint/crypto/tmhash"
)

// TODO: make these have a large predefined capacity
var (
	leafPrefix  = []byte{0}
	innerPrefix = []byte{1}
)

func leafHash(bz []byte) []byte {
	return tmhash.Sum(append(leafPrefix, bz...))
}

func innerHash(bz []byte) []byte {
	return tmhash.Sum(append(innerPrefix, bz...))
}
