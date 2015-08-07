package merkle

import (
	"bytes"
	"github.com/tendermint/tendermint/Godeps/_workspace/src/code.google.com/p/go.crypto/ripemd160"
	"io"

	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/wire"
)

// Node

type IAVLNode struct {
	key       interface{}
	value     interface{}
	height    int8
	size      int
	hash      []byte
	leftHash  []byte
	leftNode  *IAVLNode
	rightHash []byte
	rightNode *IAVLNode
	persisted bool
}

func NewIAVLNode(key interface{}, value interface{}) *IAVLNode {
	return &IAVLNode{
		key:    key,
		value:  value,
		height: 0,
		size:   1,
	}
}

// NOTE: The hash is not saved or set.  The caller should set the hash afterwards.
// (Presumably the caller already has the hash)
func ReadIAVLNode(t *IAVLTree, r io.Reader, n *int64, err *error) *IAVLNode {
	node := &IAVLNode{}

	// node header
	node.height = wire.ReadInt8(r, n, err)
	node.size = wire.ReadVarint(r, n, err)
	node.key = decodeByteSlice(t.keyCodec, r, n, err)

	if node.height == 0 {
		// value
		node.value = decodeByteSlice(t.valueCodec, r, n, err)
	} else {
		// children
		node.leftHash = wire.ReadByteSlice(r, n, err)
		node.rightHash = wire.ReadByteSlice(r, n, err)
	}
	return node
}

func (node *IAVLNode) _copy() *IAVLNode {
	if node.height == 0 {
		PanicSanity("Why are you copying a value node?")
	}
	return &IAVLNode{
		key:       node.key,
		height:    node.height,
		size:      node.size,
		hash:      nil, // Going to be mutated anyways.
		leftHash:  node.leftHash,
		leftNode:  node.leftNode,
		rightHash: node.rightHash,
		rightNode: node.rightNode,
		persisted: false, // Going to be mutated, so it can't already be persisted.
	}
}

func (node *IAVLNode) has(t *IAVLTree, key interface{}) (has bool) {
	if t.keyCodec.Compare(node.key, key) == 0 {
		return true
	}
	if node.height == 0 {
		return false
	} else {
		if t.keyCodec.Compare(key, node.key) < 0 {
			return node.getLeftNode(t).has(t, key)
		} else {
			return node.getRightNode(t).has(t, key)
		}
	}
}

func (node *IAVLNode) get(t *IAVLTree, key interface{}) (index int, value interface{}) {
	if node.height == 0 {
		if t.keyCodec.Compare(node.key, key) == 0 {
			return 0, node.value
		} else {
			return 0, nil
		}
	} else {
		if t.keyCodec.Compare(key, node.key) < 0 {
			return node.getLeftNode(t).get(t, key)
		} else {
			rightNode := node.getRightNode(t)
			index, value = rightNode.get(t, key)
			index += node.size - rightNode.size
			return index, value
		}
	}
}

func (node *IAVLNode) getByIndex(t *IAVLTree, index int) (key interface{}, value interface{}) {
	if node.height == 0 {
		if index == 0 {
			return node.key, node.value
		} else {
			PanicSanity("getByIndex asked for invalid index")
			return nil, nil
		}
	} else {
		// TODO: could improve this by storing the
		// sizes as well as left/right hash.
		leftNode := node.getLeftNode(t)
		if index < leftNode.size {
			return leftNode.getByIndex(t, index)
		} else {
			return node.getRightNode(t).getByIndex(t, index-leftNode.size)
		}
	}
}

// NOTE: sets hashes recursively
func (node *IAVLNode) hashWithCount(t *IAVLTree) ([]byte, int) {
	if node.hash != nil {
		return node.hash, 0
	}

	hasher := ripemd160.New()
	buf := new(bytes.Buffer)
	_, hashCount, err := node.writeHashBytes(t, buf)
	if err != nil {
		PanicCrisis(err)
	}
	// fmt.Printf("Wrote IAVL hash bytes: %X\n", buf.Bytes())
	hasher.Write(buf.Bytes())
	node.hash = hasher.Sum(nil)
	// fmt.Printf("Write IAVL hash: %X\n", node.hash)

	return node.hash, hashCount + 1
}

// NOTE: sets hashes recursively
func (node *IAVLNode) writeHashBytes(t *IAVLTree, w io.Writer) (n int64, hashCount int, err error) {
	// height & size
	wire.WriteInt8(node.height, w, &n, &err)
	wire.WriteVarint(node.size, w, &n, &err)
	// key is not written for inner nodes, unlike writePersistBytes

	if node.height == 0 {
		// key & value
		encodeByteSlice(node.key, t.keyCodec, w, &n, &err)
		encodeByteSlice(node.value, t.valueCodec, w, &n, &err)
	} else {
		// left
		if node.leftNode != nil {
			leftHash, leftCount := node.leftNode.hashWithCount(t)
			node.leftHash = leftHash
			hashCount += leftCount
		}
		if node.leftHash == nil {
			PanicSanity("node.leftHash was nil in writeHashBytes")
		}
		wire.WriteByteSlice(node.leftHash, w, &n, &err)
		// right
		if node.rightNode != nil {
			rightHash, rightCount := node.rightNode.hashWithCount(t)
			node.rightHash = rightHash
			hashCount += rightCount
		}
		if node.rightHash == nil {
			PanicSanity("node.rightHash was nil in writeHashBytes")
		}
		wire.WriteByteSlice(node.rightHash, w, &n, &err)
	}
	return
}

// NOTE: sets hashes recursively
// NOTE: clears leftNode/rightNode recursively
func (node *IAVLNode) save(t *IAVLTree) []byte {
	if node.hash == nil {
		node.hash, _ = node.hashWithCount(t)
	}
	if node.persisted {
		return node.hash
	}

	// save children
	if node.leftNode != nil {
		node.leftHash = node.leftNode.save(t)
		node.leftNode = nil
	}
	if node.rightNode != nil {
		node.rightHash = node.rightNode.save(t)
		node.rightNode = nil
	}

	// save node
	t.ndb.SaveNode(t, node)
	return node.hash
}

// NOTE: sets hashes recursively
func (node *IAVLNode) writePersistBytes(t *IAVLTree, w io.Writer) (n int64, err error) {
	// node header
	wire.WriteInt8(node.height, w, &n, &err)
	wire.WriteVarint(node.size, w, &n, &err)
	// key (unlike writeHashBytes, key is written for inner nodes)
	encodeByteSlice(node.key, t.keyCodec, w, &n, &err)

	if node.height == 0 {
		// value
		encodeByteSlice(node.value, t.valueCodec, w, &n, &err)
	} else {
		// left
		if node.leftHash == nil {
			PanicSanity("node.leftHash was nil in writePersistBytes")
		}
		wire.WriteByteSlice(node.leftHash, w, &n, &err)
		// right
		if node.rightHash == nil {
			PanicSanity("node.rightHash was nil in writePersistBytes")
		}
		wire.WriteByteSlice(node.rightHash, w, &n, &err)
	}
	return
}

func (node *IAVLNode) set(t *IAVLTree, key interface{}, value interface{}) (newSelf *IAVLNode, updated bool) {
	if node.height == 0 {
		cmp := t.keyCodec.Compare(key, node.key)
		if cmp < 0 {
			return &IAVLNode{
				key:       node.key,
				height:    1,
				size:      2,
				leftNode:  NewIAVLNode(key, value),
				rightNode: node,
			}, false
		} else if cmp == 0 {
			return NewIAVLNode(key, value), true
		} else {
			return &IAVLNode{
				key:       key,
				height:    1,
				size:      2,
				leftNode:  node,
				rightNode: NewIAVLNode(key, value),
			}, false
		}
	} else {
		node = node._copy()
		if t.keyCodec.Compare(key, node.key) < 0 {
			node.leftNode, updated = node.getLeftNode(t).set(t, key, value)
			node.leftHash = nil
		} else {
			node.rightNode, updated = node.getRightNode(t).set(t, key, value)
			node.rightHash = nil
		}
		if updated {
			return node, updated
		} else {
			node.calcHeightAndSize(t)
			return node.balance(t), updated
		}
	}
}

// newHash/newNode: The new hash or node to replace node after remove.
// newKey: new leftmost leaf key for tree after successfully removing 'key' if changed.
// value: removed value.
func (node *IAVLNode) remove(t *IAVLTree, key interface{}) (
	newHash []byte, newNode *IAVLNode, newKey interface{}, value interface{}, removed bool) {
	if node.height == 0 {
		if t.keyCodec.Compare(key, node.key) == 0 {
			return nil, nil, nil, node.value, true
		} else {
			return nil, node, nil, nil, false
		}
	} else {
		if t.keyCodec.Compare(key, node.key) < 0 {
			var newLeftHash []byte
			var newLeftNode *IAVLNode
			newLeftHash, newLeftNode, newKey, value, removed = node.getLeftNode(t).remove(t, key)
			if !removed {
				return nil, node, nil, value, false
			} else if newLeftHash == nil && newLeftNode == nil { // left node held value, was removed
				return node.rightHash, node.rightNode, node.key, value, true
			}
			node = node._copy()
			node.leftHash, node.leftNode = newLeftHash, newLeftNode
			node.calcHeightAndSize(t)
			return nil, node.balance(t), newKey, value, true
		} else {
			var newRightHash []byte
			var newRightNode *IAVLNode
			newRightHash, newRightNode, newKey, value, removed = node.getRightNode(t).remove(t, key)
			if !removed {
				return nil, node, nil, value, false
			} else if newRightHash == nil && newRightNode == nil { // right node held value, was removed
				return node.leftHash, node.leftNode, nil, value, true
			}
			node = node._copy()
			node.rightHash, node.rightNode = newRightHash, newRightNode
			if newKey != nil {
				node.key = newKey
				newKey = nil
			}
			node.calcHeightAndSize(t)
			return nil, node.balance(t), newKey, value, true
		}
	}
}

func (node *IAVLNode) getLeftNode(t *IAVLTree) *IAVLNode {
	if node.leftNode != nil {
		return node.leftNode
	} else {
		return t.ndb.GetNode(t, node.leftHash)
	}
}

func (node *IAVLNode) getRightNode(t *IAVLTree) *IAVLNode {
	if node.rightNode != nil {
		return node.rightNode
	} else {
		return t.ndb.GetNode(t, node.rightHash)
	}
}

func (node *IAVLNode) rotateRight(t *IAVLTree) *IAVLNode {
	node = node._copy()
	sl := node.getLeftNode(t)._copy()

	slrHash, slrCached := sl.rightHash, sl.rightNode
	sl.rightHash, sl.rightNode = nil, node
	node.leftHash, node.leftNode = slrHash, slrCached

	node.calcHeightAndSize(t)
	sl.calcHeightAndSize(t)

	return sl
}

func (node *IAVLNode) rotateLeft(t *IAVLTree) *IAVLNode {
	node = node._copy()
	sr := node.getRightNode(t)._copy()

	srlHash, srlCached := sr.leftHash, sr.leftNode
	sr.leftHash, sr.leftNode = nil, node
	node.rightHash, node.rightNode = srlHash, srlCached

	node.calcHeightAndSize(t)
	sr.calcHeightAndSize(t)

	return sr
}

// NOTE: mutates height and size
func (node *IAVLNode) calcHeightAndSize(t *IAVLTree) {
	node.height = maxInt8(node.getLeftNode(t).height, node.getRightNode(t).height) + 1
	node.size = node.getLeftNode(t).size + node.getRightNode(t).size
}

func (node *IAVLNode) calcBalance(t *IAVLTree) int {
	return int(node.getLeftNode(t).height) - int(node.getRightNode(t).height)
}

func (node *IAVLNode) balance(t *IAVLTree) (newSelf *IAVLNode) {
	balance := node.calcBalance(t)
	if balance > 1 {
		if node.getLeftNode(t).calcBalance(t) >= 0 {
			// Left Left Case
			return node.rotateRight(t)
		} else {
			// Left Right Case
			node = node._copy()
			node.leftHash, node.leftNode = nil, node.getLeftNode(t).rotateLeft(t)
			//node.calcHeightAndSize()
			return node.rotateRight(t)
		}
	}
	if balance < -1 {
		if node.getRightNode(t).calcBalance(t) <= 0 {
			// Right Right Case
			return node.rotateLeft(t)
		} else {
			// Right Left Case
			node = node._copy()
			node.rightHash, node.rightNode = nil, node.getRightNode(t).rotateRight(t)
			//node.calcHeightAndSize()
			return node.rotateLeft(t)
		}
	}
	// Nothing changed
	return node
}

func (node *IAVLNode) traverse(t *IAVLTree, cb func(*IAVLNode) bool) bool {
	stop := cb(node)
	if stop {
		return stop
	}
	if node.height > 0 {
		stop = node.getLeftNode(t).traverse(t, cb)
		if stop {
			return stop
		}
		stop = node.getRightNode(t).traverse(t, cb)
		if stop {
			return stop
		}
	}
	return false
}

// Only used in testing...
func (node *IAVLNode) lmd(t *IAVLTree) *IAVLNode {
	if node.height == 0 {
		return node
	}
	return node.getLeftNode(t).lmd(t)
}

// Only used in testing...
func (node *IAVLNode) rmd(t *IAVLTree) *IAVLNode {
	if node.height == 0 {
		return node
	}
	return node.getRightNode(t).rmd(t)
}

//--------------------------------------------------------------------------------

// Read a (length prefixed) byteslice then decode the object using the codec
func decodeByteSlice(codec wire.Codec, r io.Reader, n *int64, err *error) interface{} {
	bytez := wire.ReadByteSlice(r, n, err)
	if *err != nil {
		return nil
	}
	n_ := new(int64)
	return codec.Decode(bytes.NewBuffer(bytez), n_, err)
}

// Encode object using codec, then write a (length prefixed) byteslice.
func encodeByteSlice(o interface{}, codec wire.Codec, w io.Writer, n *int64, err *error) {
	buf, n_ := new(bytes.Buffer), new(int64)
	codec.Encode(o, buf, n_, err)
	if *err != nil {
		return
	}
	wire.WriteByteSlice(buf.Bytes(), w, n, err)
}
