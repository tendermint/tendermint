/*
Computes a deterministic minimal height merkle tree hash.
If the number of items is not a power of two, some leaves
will be at different levels. Tries to keep both sides of
the tree the same size, but the left may be one greater.

Use this for short deterministic trees, such as the validator list.
For larger datasets, use IAVLTree.

                        *
                       / \
                     /     \
                   /         \
                 /             \
                *               *
               / \             / \
              /   \           /   \
             /     \         /     \
            *       *       *       h6
           / \     / \     / \
          h0  h1  h2  h3  h4  h5

*/

package merkle

import (
	"golang.org/x/crypto/ripemd160"

	"github.com/tendermint/go-wire"
)

func SimpleHashFromTwoHashes(left []byte, right []byte) []byte {
	var hasher = ripemd160.New()
	err := wire.EncodeByteSlice(hasher, left)
	err = wire.EncodeByteSlice(hasher, right)
	if err != nil {
		panic(err)
	}
	return hasher.Sum(nil)
}

func SimpleHashFromHashes(hashes [][]byte) []byte {
	// Recursive impl.
	switch len(hashes) {
	case 0:
		return nil
	case 1:
		return hashes[0]
	default:
		left := SimpleHashFromHashes(hashes[:(len(hashes)+1)/2])
		right := SimpleHashFromHashes(hashes[(len(hashes)+1)/2:])
		return SimpleHashFromTwoHashes(left, right)
	}
}

// Convenience for SimpleHashFromHashes.
func SimpleHashFromBinaries(items []interface{}) []byte {
	hashes := make([][]byte, len(items))
	for i, item := range items {
		hashes[i] = SimpleHashFromBinary(item)
	}
	return SimpleHashFromHashes(hashes)
}

// General Convenience
func SimpleHashFromBinary(item interface{}) []byte {
	hasher := ripemd160.New()
	bz, err := wire.MarshalBinary(item)
	if err != nil {
		panic(err)
	}
	hasher.Write(bz)
	return hasher.Sum(nil)
}

// Convenience for SimpleHashFromHashes.
func SimpleHashFromHashables(items []Hashable) []byte {
	hashes := make([][]byte, len(items))
	for i, item := range items {
		hash := item.Hash()
		hashes[i] = hash
	}
	return SimpleHashFromHashes(hashes)
}

// Convenience for SimpleHashFromHashes.
func SimpleHashFromMap(m map[string]interface{}) []byte {
	sm := NewSimpleMap()
	for k, v := range m {
		sm.Set(k, v)
	}
	return sm.Hash()
}
