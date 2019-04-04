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

func SimpleHashFromByteSlicesIterative(input [][]byte) []byte {
	items := make([][]byte, len(input))

	for i, leaf := range input {
		items[i] = leafHash(leaf)
	}

	size := len(items)
	for {
		switch size {
		case 0:
			return nil
		case 1:
			return items[0]
		default:
			rp := 0 // read position
			wp := 0 // write position
			for rp < size {
				if rp+1 < size {
					items[wp] = innerHash(items[rp], items[rp+1])
					rp += 2
				} else {
					items[wp] = items[rp]
					rp += 1
				}
				wp += 1
			}
			size = wp
		}
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
