package merkle

import (
	"bytes"
	"fmt"

	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/crypto/tmhash"
)

const (
	maxAunts = 100
)

// SimpleProof represents a simple Merkle proof.
// NOTE: The convention for proofs is to include leaf hashes but to
// exclude the root hash.
// This convention is implemented across IAVL range proofs as well.
// Keep this consistent unless there's a very good reason to change
// everything.  This also affects the generalized proof system as
// well.
type SimpleProof struct {
	Total    int      `json:"total"`     // Total number of items.
	Index    int      `json:"index"`     // Index of item to prove.
	LeafHash []byte   `json:"leaf_hash"` // Hash of item value.
	Aunts    [][]byte `json:"aunts"`     // Hashes from leaf's sibling to a root's child.
}

// SimpleProofsFromByteSlices computes inclusion proof for given items.
// proofs[0] is the proof for items[0].
func SimpleProofsFromByteSlices(items [][]byte) (rootHash []byte, proofs []*SimpleProof) {
	trails, rootSPN := trailsFromByteSlices(items)
	rootHash = rootSPN.Hash
	proofs = make([]*SimpleProof, len(items))
	for i, trail := range trails {
		proofs[i] = &SimpleProof{
			Total:    len(items),
			Index:    i,
			LeafHash: trail.Hash,
			Aunts:    trail.FlattenAunts(),
		}
	}
	return
}

// SimpleProofsFromMap generates proofs from a map. The keys/values of the map will be used as the keys/values
// in the underlying key-value pairs.
// The keys are sorted before the proofs are computed.
func SimpleProofsFromMap(m map[string][]byte) (rootHash []byte, proofs map[string]*SimpleProof, keys []string) {
	sm := newSimpleMap()
	for k, v := range m {
		sm.Set(k, v)
	}
	sm.Sort()
	kvs := sm.kvs
	kvsBytes := make([][]byte, len(kvs))
	for i, kvp := range kvs {
		kvsBytes[i] = KVPair(kvp).Bytes()
	}

	rootHash, proofList := SimpleProofsFromByteSlices(kvsBytes)
	proofs = make(map[string]*SimpleProof)
	keys = make([]string, len(proofList))
	for i, kvp := range kvs {
		proofs[string(kvp.Key)] = proofList[i]
		keys[i] = string(kvp.Key)
	}
	return
}

// Verify that the SimpleProof proves the root hash.
// Check sp.Index/sp.Total manually if needed
func (sp *SimpleProof) Verify(rootHash []byte, leaf []byte) error {
	leafHash := leafHash(leaf)
	if sp.Total < 0 {
		return errors.New("Proof total must be positive")
	}
	if sp.Index < 0 {
		return errors.New("Proof index cannot be negative")
	}
	if !bytes.Equal(sp.LeafHash, leafHash) {
		return errors.Errorf("invalid leaf hash: wanted %X got %X", leafHash, sp.LeafHash)
	}
	computedHash := sp.ComputeRootHash()
	if !bytes.Equal(computedHash, rootHash) {
		return errors.Errorf("invalid root hash: wanted %X got %X", rootHash, computedHash)
	}
	return nil
}

// Compute the root hash given a leaf hash.  Does not verify the result.
func (sp *SimpleProof) ComputeRootHash() []byte {
	return computeHashFromAunts(
		sp.Index,
		sp.Total,
		sp.LeafHash,
		sp.Aunts,
	)
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

// ValidateBasic performs basic validation.
// NOTE: - it expects LeafHash and Aunts of tmhash.Size size
//			 - it expects no more than 100 aunts
func (sp *SimpleProof) ValidateBasic() error {
	if sp.Total < 0 {
		return errors.New("negative Total")
	}
	if sp.Index < 0 {
		return errors.New("negative Index")
	}
	if len(sp.LeafHash) != tmhash.Size {
		return errors.Errorf("expected LeafHash size to be %d, got %d", tmhash.Size, len(sp.LeafHash))
	}
	if len(sp.Aunts) > maxAunts {
		return errors.Errorf("expected no more than %d aunts, got %d", maxAunts, len(sp.Aunts))
	}
	for i, auntHash := range sp.Aunts {
		if len(auntHash) != tmhash.Size {
			return errors.Errorf("expected Aunts#%d size to be %d, got %d", i, tmhash.Size, len(auntHash))
		}
	}
	return nil
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
			return innerHash(leftHash, innerHashes[len(innerHashes)-1])
		}
		rightHash := computeHashFromAunts(index-numLeft, total-numLeft, leafHash, innerHashes[:len(innerHashes)-1])
		if rightHash == nil {
			return nil
		}
		return innerHash(innerHashes[len(innerHashes)-1], rightHash)
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
		switch {
		case spn.Left != nil:
			innerHashes = append(innerHashes, spn.Left.Hash)
		case spn.Right != nil:
			innerHashes = append(innerHashes, spn.Right.Hash)
		default:
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
		trail := &SimpleProofNode{leafHash(items[0]), nil, nil, nil}
		return []*SimpleProofNode{trail}, trail
	default:
		k := getSplitPoint(len(items))
		lefts, leftRoot := trailsFromByteSlices(items[:k])
		rights, rightRoot := trailsFromByteSlices(items[k:])
		rootHash := innerHash(leftRoot.Hash, rightRoot.Hash)
		root := &SimpleProofNode{rootHash, nil, nil, nil}
		leftRoot.Parent = root
		leftRoot.Right = rightRoot
		rightRoot.Parent = root
		rightRoot.Left = leftRoot
		return append(lefts, rights...), root
	}
}
