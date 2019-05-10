package merkle

import (
	"math/bits"
)

// SimpleHashFromByteSlices computes a Merkle tree where the leaves are the byte slice,
// in the provided order.
func SimpleHashFromByteSlices(items [][]byte) []byte {
	switch len(items) {
	case 0:
		return nil
	case 1:
		return leafHash(items[0])
	default:
		k := getSplitPoint(len(items))
		left := SimpleHashFromByteSlices(items[:k])
		right := SimpleHashFromByteSlices(items[k:])
		return innerHash(left, right)
	}
}

// SimpleHashFromMap computes a Merkle tree from sorted map.
// Like calling SimpleHashFromHashers with
// `item = []byte(Hash(key) | Hash(value))`,
// sorted by `item`.
func SimpleHashFromMap(m map[string][]byte) []byte {
	sm := newSimpleMap()
	for k, v := range m {
		sm.Set(k, v)
	}
	return sm.Hash()
}

// getSplitPoint returns the largest power of 2 less than length
func getSplitPoint(length int) int {
	if length < 1 {
		panic("Trying to split a tree with size < 1")
	}
	uLength := uint(length)
	bitlen := bits.Len(uLength)
	k := 1 << uint(bitlen-1)
	if k == length {
		k >>= 1
	}
	return k
}
