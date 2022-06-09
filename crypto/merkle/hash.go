package merkle

import (
	"hash"

	"github.com/tendermint/tendermint/crypto"
)

// TODO: make these have a large predefined capacity
var (
	leafPrefix  = []byte{0}
	innerPrefix = []byte{1}
)

// returns tmhash(<empty>)
func emptyHash() []byte {
	return crypto.Checksum([]byte{})
}

// returns tmhash(0x00 || leaf)
func leafHash(leaf []byte) []byte {
	return crypto.Checksum(append(leafPrefix, leaf...))
}

// returns tmhash(0x00 || leaf)
func leafHashOpt(s hash.Hash, leaf []byte) []byte {
	s.Reset()
	s.Write(leafPrefix)
	s.Write(leaf)
	return s.Sum(nil)
}

// returns tmhash(0x01 || left || right)
func innerHash(left []byte, right []byte) []byte {
	data := make([]byte, len(innerPrefix)+len(left)+len(right))
	n := copy(data, innerPrefix)
	n += copy(data[n:], left)
	copy(data[n:], right)
	return crypto.Checksum(data)[:]
}

func innerHashOpt(s hash.Hash, left []byte, right []byte) []byte {
	s.Reset()
	s.Write(innerPrefix)
	s.Write(left)
	s.Write(right)
	return s.Sum(nil)
}
