package merkle

import (
	"bytes"
	"fmt"

	"github.com/tendermint/tendermint/crypto/tmhash"
)

// SimpleProof represents a simple merkle proof.
type SimpleProof struct {
	Aunts [][]byte `json:"aunts"` // Hashes from leaf's sibling to a root's child.
}

// SimpleProofsFromByteSlices computes inclusion proof for given items.
// proofs[0] is the proof for items[0].
func SimpleProofsFromByteSlices(items [][]byte) (rootHash []byte, proofs []*SimpleProof) {
	trails, rootSPN := trailsFromByteSlices(items)
	rootHash = rootSPN.Hash
	proofs = make([]*SimpleProof, len(items))
	for i, trail := range trails {
		proofs[i] = &SimpleProof{
			Aunts: trail.FlattenAunts(),
		}
	}
	return
}

// SimpleProofsFromMap generates proofs from a map. The keys/values of the map will be used as the keys/values
// in the underlying key-value pairs.
// The keys are sorted before the proofs are computed.
func SimpleProofsFromMap(m map[string]Hasher) (rootHash []byte, proofs map[string]*SimpleProof, keys []string) {
	sm := newSimpleMap()
	for k, v := range m {
		sm.Set(k, v)
	}
	sm.Sort()
	kvs := sm.kvs
	kvsB := make([][]byte, 0, len(kvs))
	for _, kvp := range kvs {
		kvsB = append(kvsB, KVPair(kvp).Bytes())
	}

	rootHash, proofList := SimpleProofsFromByteSlices(kvsB)
	proofs = make(map[string]*SimpleProof)
	keys = make([]string, len(proofList))
	for i, kvp := range kvs {
		proofs[string(kvp.Key)] = proofList[i]
		keys[i] = string(kvp.Key)
	}
	return
}

// Verify that leaf is a leaf hash of the simple-merkle-tree
// which hashes to rootHash.
func (sp *SimpleProof) Verify(index int, total int, leaf []byte, rootHash []byte) bool {
	leafHash := tmhash.Sum(append([]byte{0}, leaf...))
	return sp.VerifyHash(index, total, leafHash, rootHash)
}

// VerifyHash that leafHash is a leaf hash of the simple-merkle-tree
// which hashes to rootHash.
func (sp *SimpleProof) VerifyHash(index int, total int, leafHash []byte, rootHash []byte) bool {
	computedHash := computeHashFromAunts(index, total, leafHash, sp.Aunts)
	return computedHash != nil && bytes.Equal(computedHash, rootHash)
}

// String implements the stringer interface for SimpleProof.
// It is a wrapper around StringIndented.
func (sp *SimpleProof) String() string {
	return sp.StringIndented("")
}

// StringIndented generates a canonical string representation of a SimpleProof.
func (sp *SimpleProof) StringIndented(indent string) string {
	return fmt.Sprintf(`SimpleProof{
%s  Aunts: %X
%s}`,
		indent, sp.Aunts,
		indent)
}

// Use the leafHash and innerHashes to get the root merkle hash.
// If the length of the innerHashes slice isn't exactly correct, the result is nil.
// Recursive impl.
func computeHashFromAunts(index int, total int, leafHash []byte, innerHashes [][]byte) []byte {
	if index >= total || index < 0 || total <= 0 {
		return nil
	}
	switch total {
	case 0:
		panic("Cannot call computeHashFromAunts() with 0 total")
	case 1:
		if len(innerHashes) != 0 {
			return nil
		}
		return leafHash
	default:
		if len(innerHashes) == 0 {
			return nil
		}
		numLeft := getSplitPoint(total)
		if index < numLeft {
			leftHash := computeHashFromAunts(index, numLeft, leafHash, innerHashes[:len(innerHashes)-1])
			if leftHash == nil {
				return nil
			}
			return SimpleHashFromTwoHashes(leftHash, innerHashes[len(innerHashes)-1])
		}
		rightHash := computeHashFromAunts(index-numLeft, total-numLeft, leafHash, innerHashes[:len(innerHashes)-1])
		if rightHash == nil {
			return nil
		}
		return SimpleHashFromTwoHashes(innerHashes[len(innerHashes)-1], rightHash)
	}
}

// SimpleProofNode is a helper structure to construct merkle proof.
// The node and the tree is thrown away afterwards.
// Exactly one of node.Left and node.Right is nil, unless node is the root, in which case both are nil.
// node.Parent.Hash = hash(node.Hash, node.Right.Hash) or
// hash(node.Left.Hash, node.Hash), depending on whether node is a left/right child.
type SimpleProofNode struct {
	Hash   []byte
	Parent *SimpleProofNode
	Left   *SimpleProofNode // Left sibling  (only one of Left,Right is set)
	Right  *SimpleProofNode // Right sibling (only one of Left,Right is set)
}

// FlattenAunts will return the inner hashes for the item corresponding to the leaf,
// starting from a leaf SimpleProofNode.
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
func trailsFromByteSlices(items [][]byte) (trails []*SimpleProofNode, root *SimpleProofNode) {
	// Recursive impl.
	switch len(items) {
	case 0:
		return nil, nil
	case 1:
		hash0 := tmhash.Sum(append(LeafHashPrefix, items[0]...))
		trail := &SimpleProofNode{hash0, nil, nil, nil}
		return []*SimpleProofNode{trail}, trail
	default:
		k := getSplitPoint(len(items))
		lefts, leftRoot := trailsFromByteSlices(items[:k])
		rights, rightRoot := trailsFromByteSlices(items[k:])
		rootHash := SimpleHashFromTwoHashes(leftRoot.Hash, rightRoot.Hash)
		root := &SimpleProofNode{rootHash, nil, nil, nil}
		leftRoot.Parent = root
		leftRoot.Right = rightRoot
		rightRoot.Parent = root
		rightRoot.Left = leftRoot
		return append(lefts, rights...), root
	}
}
