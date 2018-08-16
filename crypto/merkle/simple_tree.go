package merkle

import (
	"fmt"
	"math/bits"

	"github.com/tendermint/tendermint/crypto/tmhash"
)

// SimpleHashFromTwoHashes is the basic operation of the Merkle tree: Hash(left | right).
func SimpleHashFromTwoHashes(left, right []byte) []byte {
	var hasher = tmhash.New()
	_, err := hasher.Write([]byte{1})
	if err != nil {
		panic(err)
	}
	_, err = hasher.Write(left)
	if err != nil {
		panic(err)
	}
	_, err = hasher.Write(right)
	if err != nil {
		panic(err)
	}
	return hasher.Sum(nil)
}

// SimpleHashFromByteSlices computes a Merkle tree from items that are byte slices.
func SimpleHashFromByteSlices(items [][]byte) []byte {
	hashes := make([][]byte, len(items))
	for i, item := range items {
		hash := tmhash.Sum(append([]byte{0}, item...))
		hashes[i] = hash
	}
	return simpleHashFromHashes(hashes)
}

// SimpleHashFromMap computes a Merkle tree from sorted map.
// Like calling SimpleHashFromHashers with
// `item = []byte(Hash(key) | Hash(value))`,
// sorted by `item`.
func SimpleHashFromMap(m map[string]Hasher) []byte {
	sm := newSimpleMap()
	for k, v := range m {
		sm.Set(k, v)
	}
	return sm.Hash()
}

//----------------------------------------------------------------

// Expects hashes!
func simpleHashFromHashes(hashes [][]byte) []byte {
	// Recursive impl.
	switch len(hashes) {
	case 0:
		return nil
	case 1:
		return hashes[0]
	default:
		k := getSplitPoint(len(hashes))
		left := simpleHashFromHashes(hashes[:k])
		right := simpleHashFromHashes(hashes[k:])
		return SimpleHashFromTwoHashes(left, right)
	}
}

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
	fmt.Println(length, k)
	return k
}
