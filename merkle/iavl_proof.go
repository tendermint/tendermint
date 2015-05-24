package merkle

import (
	"bytes"
	"crypto/sha256"

	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

type IAVLProof struct {
	Root     []byte
	Branches []IAVLProofBranch
	Leaf     IAVLProofLeaf
}

func (proof *IAVLProof) Verify() bool {
	hash := proof.Leaf.Hash()
	// fmt.Printf("leaf hash: %X\n", hash)
	for i := len(proof.Branches) - 1; 0 <= i; i-- {
		hash = proof.Branches[i].Hash(hash)
		// fmt.Printf("branch hash: %X\n", hash)
	}
	// fmt.Printf("root: %X, computed: %X\n", proof.Root, hash)
	return bytes.Equal(proof.Root, hash)
}

type IAVLProofBranch struct {
	Height uint8
	Size   uint
	Left   []byte
	Right  []byte
}

func (branch IAVLProofBranch) Hash(childHash []byte) []byte {
	hasher := sha256.New()
	buf := new(bytes.Buffer)
	n, err := int64(0), error(nil)
	binary.WriteUint8(branch.Height, buf, &n, &err)
	binary.WriteUvarint(branch.Size, buf, &n, &err)
	if branch.Left == nil {
		binary.WriteByteSlice(childHash, buf, &n, &err)
		binary.WriteByteSlice(branch.Right, buf, &n, &err)
	} else {
		binary.WriteByteSlice(branch.Left, buf, &n, &err)
		binary.WriteByteSlice(childHash, buf, &n, &err)
	}
	if err != nil {
		panic(Fmt("Failed to hash IAVLProofBranch: %v", err))
	}
	// fmt.Printf("Branch hash bytes: %X\n", buf.Bytes())
	hasher.Write(buf.Bytes())
	return hasher.Sum(nil)
}

type IAVLProofLeaf struct {
	KeyBytes   []byte
	ValueBytes []byte
}

func (leaf IAVLProofLeaf) Hash() []byte {
	hasher := sha256.New()
	buf := new(bytes.Buffer)
	n, err := int64(0), error(nil)
	binary.WriteUint8(0, buf, &n, &err)
	binary.WriteUvarint(1, buf, &n, &err)
	binary.WriteByteSlice(leaf.KeyBytes, buf, &n, &err)
	binary.WriteByteSlice(leaf.ValueBytes, buf, &n, &err)
	if err != nil {
		panic(Fmt("Failed to hash IAVLProofLeaf: %v", err))
	}
	// fmt.Printf("Leaf hash bytes:   %X\n", buf.Bytes())
	hasher.Write(buf.Bytes())
	return hasher.Sum(nil)
}

func (node *IAVLNode) constructProof(t *IAVLTree, key interface{}, proof *IAVLProof) (exists bool) {
	if node.height == 0 {
		if t.keyCodec.Compare(node.key, key) == 0 {
			keyBuf, valueBuf := new(bytes.Buffer), new(bytes.Buffer)
			n, err := int64(0), error(nil)
			t.keyCodec.Encode(node.key, keyBuf, &n, &err)
			if err != nil {
				panic(Fmt("Failed to encode node.key: %v", err))
			}
			t.valueCodec.Encode(node.value, valueBuf, &n, &err)
			if err != nil {
				panic(Fmt("Failed to encode node.value: %v", err))
			}
			leaf := IAVLProofLeaf{
				KeyBytes:   keyBuf.Bytes(),
				ValueBytes: valueBuf.Bytes(),
			}
			proof.Leaf = leaf
			return true
		} else {
			return false
		}
	} else {
		if t.keyCodec.Compare(key, node.key) < 0 {
			branch := IAVLProofBranch{
				Height: node.height,
				Size:   node.size,
				Left:   nil,
				Right:  node.getRightNode(t).hash,
			}
			if node.getRightNode(t).hash == nil {
				// fmt.Println(node.getRightNode(t))
				panic("WTF")
			}
			proof.Branches = append(proof.Branches, branch)
			return node.getLeftNode(t).constructProof(t, key, proof)
		} else {
			branch := IAVLProofBranch{
				Height: node.height,
				Size:   node.size,
				Left:   node.getLeftNode(t).hash,
				Right:  nil,
			}
			proof.Branches = append(proof.Branches, branch)
			return node.getRightNode(t).constructProof(t, key, proof)
		}
	}
}

// Returns nil if key is not in tree.
func (t *IAVLTree) ConstructProof(key interface{}) *IAVLProof {
	if t.root == nil {
		return nil
	}
	t.root.hashWithCount(t) // Ensure that all hashes are calculated.
	proof := &IAVLProof{
		Root: t.root.hash,
	}
	t.root.constructProof(t, key, proof)
	return proof
}
