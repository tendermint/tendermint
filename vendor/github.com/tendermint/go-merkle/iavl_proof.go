package merkle

import (
	"bytes"

	"golang.org/x/crypto/ripemd160"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"
)

type IAVLProof struct {
	LeafNode   IAVLProofLeafNode
	InnerNodes []IAVLProofInnerNode
	RootHash   []byte
}

func (proof *IAVLProof) Verify(keyBytes, valueBytes, rootHash []byte) bool {
	if !bytes.Equal(keyBytes, proof.LeafNode.KeyBytes) {
		return false
	}
	if !bytes.Equal(valueBytes, proof.LeafNode.ValueBytes) {
		return false
	}
	if !bytes.Equal(rootHash, proof.RootHash) {
		return false
	}
	hash := proof.LeafNode.Hash()
	// fmt.Printf("leaf hash: %X\n", hash)
	for _, branch := range proof.InnerNodes {
		hash = branch.Hash(hash)
		// fmt.Printf("branch hash: %X\n", hash)
	}
	// fmt.Printf("root: %X, computed: %X\n", proof.RootHash, hash)
	return bytes.Equal(proof.RootHash, hash)
}

type IAVLProofInnerNode struct {
	Height int8
	Size   int
	Left   []byte
	Right  []byte
}

func (branch IAVLProofInnerNode) Hash(childHash []byte) []byte {
	hasher := ripemd160.New()
	buf := new(bytes.Buffer)
	n, err := int(0), error(nil)
	wire.WriteInt8(branch.Height, buf, &n, &err)
	wire.WriteVarint(branch.Size, buf, &n, &err)
	if len(branch.Left) == 0 {
		wire.WriteByteSlice(childHash, buf, &n, &err)
		wire.WriteByteSlice(branch.Right, buf, &n, &err)
	} else {
		wire.WriteByteSlice(branch.Left, buf, &n, &err)
		wire.WriteByteSlice(childHash, buf, &n, &err)
	}
	if err != nil {
		PanicCrisis(Fmt("Failed to hash IAVLProofInnerNode: %v", err))
	}
	// fmt.Printf("InnerNode hash bytes: %X\n", buf.Bytes())
	hasher.Write(buf.Bytes())
	return hasher.Sum(nil)
}

type IAVLProofLeafNode struct {
	KeyBytes   []byte
	ValueBytes []byte
}

func (leaf IAVLProofLeafNode) Hash() []byte {
	hasher := ripemd160.New()
	buf := new(bytes.Buffer)
	n, err := int(0), error(nil)
	wire.WriteInt8(0, buf, &n, &err)
	wire.WriteVarint(1, buf, &n, &err)
	wire.WriteByteSlice(leaf.KeyBytes, buf, &n, &err)
	wire.WriteByteSlice(leaf.ValueBytes, buf, &n, &err)
	if err != nil {
		PanicCrisis(Fmt("Failed to hash IAVLProofLeafNode: %v", err))
	}
	// fmt.Printf("LeafNode hash bytes:   %X\n", buf.Bytes())
	hasher.Write(buf.Bytes())
	return hasher.Sum(nil)
}

func (node *IAVLNode) constructProof(t *IAVLTree, key []byte, proof *IAVLProof) (exists bool) {
	if node.height == 0 {
		if bytes.Compare(node.key, key) == 0 {
			leaf := IAVLProofLeafNode{
				KeyBytes:   node.key,
				ValueBytes: node.value,
			}
			proof.LeafNode = leaf
			return true
		} else {
			return false
		}
	} else {
		if bytes.Compare(key, node.key) < 0 {
			exists := node.getLeftNode(t).constructProof(t, key, proof)
			if !exists {
				return false
			}
			branch := IAVLProofInnerNode{
				Height: node.height,
				Size:   node.size,
				Left:   nil,
				Right:  node.getRightNode(t).hash,
			}
			proof.InnerNodes = append(proof.InnerNodes, branch)
			return true
		} else {
			exists := node.getRightNode(t).constructProof(t, key, proof)
			if !exists {
				return false
			}
			branch := IAVLProofInnerNode{
				Height: node.height,
				Size:   node.size,
				Left:   node.getLeftNode(t).hash,
				Right:  nil,
			}
			proof.InnerNodes = append(proof.InnerNodes, branch)
			return true
		}
	}
}

// Returns nil if key is not in tree.
func (t *IAVLTree) ConstructProof(key []byte) *IAVLProof {
	if t.root == nil {
		return nil
	}
	t.root.hashWithCount(t) // Ensure that all hashes are calculated.
	proof := &IAVLProof{
		RootHash: t.root.hash,
	}
	t.root.constructProof(t, key, proof)
	return proof
}
