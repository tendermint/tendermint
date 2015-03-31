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
	"bytes"
	"crypto/sha256"

	"github.com/tendermint/tendermint2/binary"
)

func HashFromTwoHashes(left []byte, right []byte) []byte {
	var n int64
	var err error
	var hasher = sha256.New()
	binary.WriteByteSlice(left, hasher, &n, &err)
	binary.WriteByteSlice(right, hasher, &n, &err)
	if err != nil {
		panic(err)
	}
	return hasher.Sum(nil)
}

func HashFromHashes(hashes [][]byte) []byte {
	// Recursive impl.
	switch len(hashes) {
	case 0:
		return nil
	case 1:
		return hashes[0]
	default:
		left := HashFromHashes(hashes[:(len(hashes)+1)/2])
		right := HashFromHashes(hashes[(len(hashes)+1)/2:])
		return HashFromTwoHashes(left, right)
	}
}

// Convenience for HashFromHashes.
func HashFromBinaries(items []interface{}) []byte {
	hashes := [][]byte{}
	for _, item := range items {
		hashes = append(hashes, HashFromBinary(item))
	}
	return HashFromHashes(hashes)
}

// General Convenience
func HashFromBinary(item interface{}) []byte {
	hasher, n, err := sha256.New(), new(int64), new(error)
	binary.WriteBinary(item, hasher, n, err)
	if *err != nil {
		panic(err)
	}
	return hasher.Sum(nil)
}

// Convenience for HashFromHashes.
func HashFromHashables(items []Hashable) []byte {
	hashes := [][]byte{}
	for _, item := range items {
		hash := item.Hash()
		hashes = append(hashes, hash)
	}
	return HashFromHashes(hashes)
}

type HashTrail struct {
	Hash   []byte
	Parent *HashTrail
	Left   *HashTrail
	Right  *HashTrail
}

func (ht *HashTrail) Flatten() [][]byte {
	// Nonrecursive impl.
	trail := [][]byte{}
	for ht != nil {
		if ht.Left != nil {
			trail = append(trail, ht.Left.Hash)
		} else if ht.Right != nil {
			trail = append(trail, ht.Right.Hash)
		} else {
			break
		}
		ht = ht.Parent
	}
	return trail
}

// returned trails[0].Hash is the leaf hash.
// trails[0].Parent.Hash is the hash above that, etc.
func HashTrailsFromHashables(items []Hashable) (trails []*HashTrail, root *HashTrail) {
	// Recursive impl.
	switch len(items) {
	case 0:
		return nil, nil
	case 1:
		trail := &HashTrail{items[0].Hash(), nil, nil, nil}
		return []*HashTrail{trail}, trail
	default:
		lefts, leftRoot := HashTrailsFromHashables(items[:(len(items)+1)/2])
		rights, rightRoot := HashTrailsFromHashables(items[(len(items)+1)/2:])
		rootHash := HashFromTwoHashes(leftRoot.Hash, rightRoot.Hash)
		root := &HashTrail{rootHash, nil, nil, nil}
		leftRoot.Parent = root
		leftRoot.Right = rightRoot
		rightRoot.Parent = root
		rightRoot.Left = leftRoot
		return append(lefts, rights...), root
	}
}

// Ensures that leafHash is part of rootHash.
func VerifyHashTrail(index uint, total uint, leafHash []byte, trail [][]byte, rootHash []byte) bool {
	computedRoot := ComputeRootFromTrail(index, total, leafHash, trail)
	if computedRoot == nil {
		return false
	}
	return bytes.Equal(computedRoot, rootHash)
}

// Use the leafHash and trail to get the root merkle hash.
// If the length of the trail slice isn't exactly correct, the result is nil.
func ComputeRootFromTrail(index uint, total uint, leafHash []byte, trail [][]byte) []byte {
	// Recursive impl.
	if index >= total {
		return nil
	}
	switch total {
	case 0:
		panic("Cannot call ComputeRootFromTrail() with 0 total")
	case 1:
		if len(trail) != 0 {
			return nil
		}
		return leafHash
	default:
		if len(trail) == 0 {
			return nil
		}
		numLeft := (total + 1) / 2
		if index < numLeft {
			leftRoot := ComputeRootFromTrail(index, numLeft, leafHash, trail[:len(trail)-1])
			if leftRoot == nil {
				return nil
			}
			return HashFromTwoHashes(leftRoot, trail[len(trail)-1])
		} else {
			rightRoot := ComputeRootFromTrail(index-numLeft, total-numLeft, leafHash, trail[:len(trail)-1])
			if rightRoot == nil {
				return nil
			}
			return HashFromTwoHashes(trail[len(trail)-1], rightRoot)
		}
	}
}
