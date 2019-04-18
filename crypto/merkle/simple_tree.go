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

// SimpleHashFromByteSliceIterative is an iterative alternative to
// SimpleHashFromByteSlice motivated by potential performance improvements.
// (#2611) had suggested that an iterative version of
// SimpleHashFromByteSlice would be faster, presumably because
// we can envision some overhead accumulating from stack
// frames and function calls. Additionally, a recursive algorithm risks
// hitting the stack limit and causing a stack overflow should the tree
// be too large.
//
// Provided here is an iterative alternative, a simple test to assert
// correctness and a benchmark. On the performance side, there appears to
// be no overall difference:
//
// BenchmarkSimpleHashAlternatives/recursive-4                20000 77677 ns/op
// BenchmarkSimpleHashAlternatives/iterative-4                20000 76802 ns/op
//
// On the surface it might seem that the additional overhead is due to
// the different allocation patterns of the implementations. The recursive
// version uses a single [][]byte slices which it then re-slices at each level of the tree.
// The iterative version reproduces [][]byte once within the function and
// then rewrites sub-slices of that array at each level of the tree.
//
// Experimenting by modifying the code to simply calculate the
// hash and not store the result show little to no difference in performance.
//
// These preliminary results suggest:
//
// 1. The performance of the SimpleHashFromByteSlice is pretty good
// 2. Go has low overhead for recursive functions
// 3. The performance of the SimpleHashFromByteSlice routine is dominated
//    by the actual hashing of data
//
// Although this work is in no way exhaustive, point #3 suggests that
// optimization of this routine would need to take an alternative
// approach to make significant improvements on the current performance.
//
// Finally, considering that the recursive implementation is easier to
// read, it might not be worthwhile to switch to a less intuitive
// implementation for so little benefit.
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
