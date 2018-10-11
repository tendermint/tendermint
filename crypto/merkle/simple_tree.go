package merkle

import (
	"github.com/tendermint/tendermint/crypto/tmhash"
)

// SimpleHashFromTwoHashes is the basic operation of the Merkle tree: Hash(left | right).
func SimpleHashFromTwoHashes(left, right []byte) []byte {
	var hasher = tmhash.New()
	err := encodeByteSlice(hasher, left)
	if err != nil {
		panic(err)
	}
	err = encodeByteSlice(hasher, right)
	if err != nil {
		panic(err)
	}
	return hasher.Sum(nil)
}

// SimpleHashFromByteSlices computes a Merkle tree where the leaves are the byte slice,
// in the provided order.
func SimpleHashFromByteSlices(items [][]byte) []byte {
	hashes := make([][]byte, len(items))
	for i, item := range items {
		hash := tmhash.Sum(item)
		hashes[i] = hash
	}
	return simpleHashFromHashes(hashes)
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

//----------------------------------------------------------------

// Expects hashes!
func simpleHashFromHashes(hashes [][]byte) []byte {
	return simpleHashFromHashesInPlace(hashes, 0, len(hashes))
}

func simpleHashFromHashesInPlace(hashes [][]byte, start, end int) []byte {
	// In place recursive impl.
	numHashes := end - start
	switch numHashes {
	case 0:
		return nil
	case 1:
		return hashes[start]
	default:
		left := simpleHashFromHashesInPlace(hashes, start, start+(numHashes+1)/2)
		right := simpleHashFromHashesInPlace(hashes, start+(numHashes+1)/2, end)
		return SimpleHashFromTwoHashes(left, right)
	}
}
