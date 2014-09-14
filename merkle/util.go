package merkle

import (
	"bytes"
	"crypto/sha256"
	"fmt"

	. "github.com/tendermint/tendermint/binary"
)

func HashFromTwoHashes(left []byte, right []byte) []byte {
	var n int64
	var err error
	var hasher = sha256.New()
	WriteByteSlice(hasher, left, &n, &err)
	WriteByteSlice(hasher, right, &n, &err)
	if err != nil {
		panic(err)
	}
	return hasher.Sum(nil)
}

/*
Computes a deterministic minimal height merkle tree hash.
If the number of items is not a power of two, some leaves
will be at different levels.

				*
			   / \
			 /     \
		   /         \
		 /             \
		*               *
	   / \             / \
	  /   \           /   \
	 /     \         /     \
	*       h2      *       *
   / \             / \     / \
  h0  h1          h3  h4  h5  h6

*/
func HashFromHashes(hashes [][]byte) []byte {
	switch len(hashes) {
	case 0:
		panic("Cannot compute hash of empty slice")
	case 1:
		return hashes[0]
	default:
		left := HashFromHashes(hashes[:len(hashes)/2])
		right := HashFromHashes(hashes[len(hashes)/2:])
		return HashFromTwoHashes(left, right)
	}
}

// Convenience for HashFromHashes.
func HashFromBinaries(items []Binary) []byte {
	hashes := [][]byte{}
	for _, item := range items {
		hasher := sha256.New()
		_, err := item.WriteTo(hasher)
		if err != nil {
			panic(err)
		}
		hash := hasher.Sum(nil)
		hashes = append(hashes, hash)
	}
	return HashFromHashes(hashes)
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

/*
Calculates an array of hashes, useful for deriving hash trails.

				7
			   / \
			 /     \
		   /         \
		 /             \
		3                11
	   / \              / \
	  /   \            /   \
	 /     \          /     \
	1       5        9       13
   / \     / \      / \      / \
  0   2   4   6    8   10  12   14
  h0  h1  h2  h3   h4  h5  h6   h7

(diagram and idea borrowed from libswift)

The hashes provided get assigned to even indices.
The derived merkle hashes get assigned to odd indices.
If "hashes" is not of length power of 2, it is padded
with blank (zeroed) hashes.
*/
func HashTreeFromHashes(hashes [][]byte) [][]byte {

	// Make length of "hashes" a power of 2
	hashesLen := uint32(len(hashes))
	fullLen := uint32(1)
	for {
		if fullLen >= hashesLen {
			break
		} else {
			fullLen <<= 1
		}
	}
	blank := make([]byte, len(hashes[0]))
	for i := hashesLen; i < fullLen; i++ {
		hashes = append(hashes, blank)
	}

	// The result is twice the length minus one.
	res := make([][]byte, len(hashes)*2-1)
	for i, hash := range hashes {
		res[i*2] = hash
	}

	// Fill all the hashes recursively.
	fillTreeRoot(res, 0, len(res)-1)
	return res
}

// Fill in the blanks.
func fillTreeRoot(res [][]byte, start, end int) []byte {
	if start == end {
		return res[start]
	} else {
		mid := (start + end) / 2
		left := fillTreeRoot(res, start, mid-1)
		right := fillTreeRoot(res, mid+1, end)
		root := HashFromTwoHashes(left, right)
		res[mid] = root
		return root
	}
}

// Convenience for HashTreeFromHashes.
func HashTreeFromHashables(items []Hashable) [][]byte {
	hashes := [][]byte{}
	for _, item := range items {
		hash := item.Hash()
		hashes = append(hashes, hash)
	}
	return HashTreeFromHashes(hashes)
}

/*
Given the original index of an item,
(e.g. for h5 in the diagram above, the index is 5, not 10)
returns a trail of hashes, which along with the index can be
used to calculate the merkle root.

See VerifyHashTrailForIndex()
*/
func HashTrailForIndex(hashTree [][]byte, index int) [][]byte {
	trail := [][]byte{}
	index *= 2

	// We start from the leaf layer and work our way up.
	// Notice the indices in the diagram:
	// 0  2  4 ... offset 0, stride 2
	// 1  5  9 ... offset 1, stride 4
	// 3 11 19 ... offset 3, stride 8
	// 7 23 39 ... offset 7, stride 16 etc.

	offset := 0
	stride := 2

	for {
		// Calculate sibling of index.
		var next int
		if ((index-offset)/stride)%2 == 0 {
			next = index + stride
		} else {
			next = index - stride
		}
		if next >= len(hashTree) {
			break
		}
		// Insert sibling hash to trail.
		trail = append(trail, hashTree[next])

		index = (index + next) / 2
		offset += stride
		stride *= 2
	}

	return trail
}

// Ensures that leafHash is part of rootHash.
func VerifyHashTrailForIndex(index int, leafHash []byte, trail [][]byte, rootHash []byte) bool {
	index *= 2
	offset := 0
	stride := 2

	tempHash := make([]byte, len(leafHash))
	copy(tempHash, leafHash)

	for i := 0; i < len(trail); i++ {
		var next int
		if ((index-offset)/stride)%2 == 0 {
			next = index + stride
			tempHash = HashFromTwoHashes(tempHash, trail[i])
		} else {
			next = index - stride
			tempHash = HashFromTwoHashes(trail[i], tempHash)
		}
		index = (index + next) / 2
		offset += stride
		stride *= 2
	}

	return bytes.Equal(rootHash, tempHash)
}

//-----------------------------------------------------------------------------

func PrintIAVLNode(node *IAVLNode) {
	fmt.Println("==== NODE")
	if node != nil {
		printIAVLNode(node, 0)
	}
	fmt.Println("==== END")
}

func printIAVLNode(node *IAVLNode, indent int) {
	indentPrefix := ""
	for i := 0; i < indent; i++ {
		indentPrefix += "    "
	}

	if node.right != nil {
		printIAVLNode(node.rightFilled(nil), indent+1)
	}

	fmt.Printf("%s%v:%v\n", indentPrefix, node.key, node.height)

	if node.left != nil {
		printIAVLNode(node.leftFilled(nil), indent+1)
	}

}

func maxUint8(a, b uint8) uint8 {
	if a > b {
		return a
	}
	return b
}
