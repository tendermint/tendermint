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
	"fmt"
	"sort"

	"golang.org/x/crypto/ripemd160"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"
)

func SimpleHashFromTwoHashes(left []byte, right []byte) []byte {
	var n int
	var err error
	var hasher = ripemd160.New()
	wire.WriteByteSlice(left, hasher, &n, &err)
	wire.WriteByteSlice(right, hasher, &n, &err)
	if err != nil {
		PanicCrisis(err)
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
	hasher, n, err := ripemd160.New(), new(int), new(error)
	wire.WriteBinary(item, hasher, n, err)
	if *err != nil {
		PanicCrisis(err)
	}
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
	kpPairsH := MakeSortedKVPairs(m)
	return SimpleHashFromHashables(kpPairsH)
}

//--------------------------------------------------------------------------------

/* Convenience struct for key-value pairs.
A list of KVPairs is hashed via `SimpleHashFromHashables`.
NOTE: Each `Value` is encoded for hashing without extra type information,
so the user is presumed to be aware of the Value types.
*/
type KVPair struct {
	Key   string
	Value interface{}
}

func (kv KVPair) Hash() []byte {
	hasher, n, err := ripemd160.New(), new(int), new(error)
	wire.WriteString(kv.Key, hasher, n, err)
	if kvH, ok := kv.Value.(Hashable); ok {
		wire.WriteByteSlice(kvH.Hash(), hasher, n, err)
	} else {
		wire.WriteBinary(kv.Value, hasher, n, err)
	}
	if *err != nil {
		PanicSanity(*err)
	}
	return hasher.Sum(nil)
}

type KVPairs []KVPair

func (kvps KVPairs) Len() int           { return len(kvps) }
func (kvps KVPairs) Less(i, j int) bool { return kvps[i].Key < kvps[j].Key }
func (kvps KVPairs) Swap(i, j int)      { kvps[i], kvps[j] = kvps[j], kvps[i] }
func (kvps KVPairs) Sort()              { sort.Sort(kvps) }

func MakeSortedKVPairs(m map[string]interface{}) []Hashable {
	kvPairs := []KVPair{}
	for k, v := range m {
		kvPairs = append(kvPairs, KVPair{k, v})
	}
	KVPairs(kvPairs).Sort()
	kvPairsH := []Hashable{}
	for _, kvp := range kvPairs {
		kvPairsH = append(kvPairsH, kvp)
	}
	return kvPairsH
}

//--------------------------------------------------------------------------------

type SimpleProof struct {
	Aunts [][]byte `json:"aunts"` // Hashes from leaf's sibling to a root's child.
}

// proofs[0] is the proof for items[0].
func SimpleProofsFromHashables(items []Hashable) (rootHash []byte, proofs []*SimpleProof) {
	trails, rootSPN := trailsFromHashables(items)
	rootHash = rootSPN.Hash
	proofs = make([]*SimpleProof, len(items))
	for i, trail := range trails {
		proofs[i] = &SimpleProof{
			Aunts: trail.FlattenAunts(),
		}
	}
	return
}

// Verify that leafHash is a leaf hash of the simple-merkle-tree
// which hashes to rootHash.
func (sp *SimpleProof) Verify(index int, total int, leafHash []byte, rootHash []byte) bool {
	computedHash := computeHashFromAunts(index, total, leafHash, sp.Aunts)
	if computedHash == nil {
		return false
	}
	if !bytes.Equal(computedHash, rootHash) {
		return false
	}
	return true
}

func (sp *SimpleProof) String() string {
	return sp.StringIndented("")
}

func (sp *SimpleProof) StringIndented(indent string) string {
	return fmt.Sprintf(`SimpleProof{
%s  Aunts: %X
%s}`,
		indent, sp.Aunts,
		indent)
}

// Use the leafHash and innerHashes to get the root merkle hash.
// If the length of the innerHashes slice isn't exactly correct, the result is nil.
func computeHashFromAunts(index int, total int, leafHash []byte, innerHashes [][]byte) []byte {
	// Recursive impl.
	if index >= total {
		return nil
	}
	switch total {
	case 0:
		PanicSanity("Cannot call computeHashFromAunts() with 0 total")
		return nil
	case 1:
		if len(innerHashes) != 0 {
			return nil
		}
		return leafHash
	default:
		if len(innerHashes) == 0 {
			return nil
		}
		numLeft := (total + 1) / 2
		if index < numLeft {
			leftHash := computeHashFromAunts(index, numLeft, leafHash, innerHashes[:len(innerHashes)-1])
			if leftHash == nil {
				return nil
			}
			return SimpleHashFromTwoHashes(leftHash, innerHashes[len(innerHashes)-1])
		} else {
			rightHash := computeHashFromAunts(index-numLeft, total-numLeft, leafHash, innerHashes[:len(innerHashes)-1])
			if rightHash == nil {
				return nil
			}
			return SimpleHashFromTwoHashes(innerHashes[len(innerHashes)-1], rightHash)
		}
	}
}

// Helper structure to construct merkle proof.
// The node and the tree is thrown away afterwards.
// Exactly one of node.Left and node.Right is nil, unless node is the root, in which case both are nil.
// node.Parent.Hash = hash(node.Hash, node.Right.Hash) or
// 									  hash(node.Left.Hash, node.Hash), depending on whether node is a left/right child.
type SimpleProofNode struct {
	Hash   []byte
	Parent *SimpleProofNode
	Left   *SimpleProofNode // Left sibling  (only one of Left,Right is set)
	Right  *SimpleProofNode // Right sibling (only one of Left,Right is set)
}

// Starting from a leaf SimpleProofNode, FlattenAunts() will return
// the inner hashes for the item corresponding to the leaf.
func (spn *SimpleProofNode) FlattenAunts() [][]byte {
	// Nonrecursive impl.
	innerHashes := [][]byte{}
	for spn != nil {
		if spn.Left != nil {
			innerHashes = append(innerHashes, spn.Left.Hash)
		} else if spn.Right != nil {
			innerHashes = append(innerHashes, spn.Right.Hash)
		} else {
			break
		}
		spn = spn.Parent
	}
	return innerHashes
}

// trails[0].Hash is the leaf hash for items[0].
// trails[i].Parent.Parent....Parent == root for all i.
func trailsFromHashables(items []Hashable) (trails []*SimpleProofNode, root *SimpleProofNode) {
	// Recursive impl.
	switch len(items) {
	case 0:
		return nil, nil
	case 1:
		trail := &SimpleProofNode{items[0].Hash(), nil, nil, nil}
		return []*SimpleProofNode{trail}, trail
	default:
		lefts, leftRoot := trailsFromHashables(items[:(len(items)+1)/2])
		rights, rightRoot := trailsFromHashables(items[(len(items)+1)/2:])
		rootHash := SimpleHashFromTwoHashes(leftRoot.Hash, rightRoot.Hash)
		root := &SimpleProofNode{rootHash, nil, nil, nil}
		leftRoot.Parent = root
		leftRoot.Right = rightRoot
		rightRoot.Parent = root
		rightRoot.Left = leftRoot
		return append(lefts, rights...), root
	}
}
