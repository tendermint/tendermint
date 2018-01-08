package iavl

import (
	"bytes"
	"fmt"
	"io"

	"golang.org/x/crypto/ripemd160"

	"github.com/tendermint/go-wire"
	cmn "github.com/tendermint/tmlibs/common"
)

// Node represents a node in a Tree.
type Node struct {
	key       []byte
	value     []byte
	version   uint64
	height    int8
	size      int
	hash      []byte
	leftHash  []byte
	leftNode  *Node
	rightHash []byte
	rightNode *Node
	persisted bool
}

// NewNode returns a new node from a key and value.
func NewNode(key []byte, value []byte) *Node {
	return &Node{
		key:     key,
		value:   value,
		height:  0,
		size:    1,
		version: 0,
	}
}

// MakeNode constructs an *Node from an encoded byte slice.
//
// The new node doesn't have its hash saved or set.  The caller must set it
// afterwards.
func MakeNode(buf []byte) (node *Node, err error) {
	node = &Node{}

	// Read node header.

	node.height = int8(buf[0])

	n := 1 // Keeps track of bytes read.
	buf = buf[n:]

	node.size, n, err = wire.GetVarint(buf)
	if err != nil {
		return nil, err
	}
	buf = buf[n:]

	node.key, n, err = wire.GetByteSlice(buf)
	if err != nil {
		return nil, err
	}
	buf = buf[n:]

	node.version = wire.GetUint64(buf)
	buf = buf[8:]

	// Read node body.

	if node.isLeaf() {
		node.value, _, err = wire.GetByteSlice(buf)
		if err != nil {
			return nil, err
		}
	} else { // Read children.
		leftHash, n, err := wire.GetByteSlice(buf)
		if err != nil {
			return nil, err
		}
		buf = buf[n:]

		rightHash, _, err := wire.GetByteSlice(buf)
		if err != nil {
			return nil, err
		}
		node.leftHash = leftHash
		node.rightHash = rightHash
	}
	return node, nil
}

// String returns a string representation of the node.
func (node *Node) String() string {
	if len(node.hash) == 0 {
		return "<no hash>"
	} else {
		return fmt.Sprintf("%x", node.hash)
	}
}

// clone creates a shallow copy of a node with its hash set to nil.
func (node *Node) clone() *Node {
	if node.isLeaf() {
		cmn.PanicSanity("Attempt to copy a leaf node")
	}
	return &Node{
		key:       node.key,
		height:    node.height,
		version:   node.version,
		size:      node.size,
		hash:      nil,
		leftHash:  node.leftHash,
		leftNode:  node.leftNode,
		rightHash: node.rightHash,
		rightNode: node.rightNode,
		persisted: false,
	}
}

func (node *Node) isLeaf() bool {
	return node.height == 0
}

// Check if the node has a descendant with the given key.
func (node *Node) has(t *Tree, key []byte) (has bool) {
	if bytes.Equal(node.key, key) {
		return true
	}
	if node.isLeaf() {
		return false
	}
	if bytes.Compare(key, node.key) < 0 {
		return node.getLeftNode(t).has(t, key)
	} else {
		return node.getRightNode(t).has(t, key)
	}
}

// Get a key under the node.
func (node *Node) get(t *Tree, key []byte) (index int, value []byte) {
	if node.isLeaf() {
		switch bytes.Compare(node.key, key) {
		case -1:
			return 1, nil
		case 1:
			return 0, nil
		default:
			return 0, node.value
		}
	}

	if bytes.Compare(key, node.key) < 0 {
		return node.getLeftNode(t).get(t, key)
	} else {
		rightNode := node.getRightNode(t)
		index, value = rightNode.get(t, key)
		index += node.size - rightNode.size
		return index, value
	}
}

func (node *Node) getByIndex(t *Tree, index int) (key []byte, value []byte) {
	if node.isLeaf() {
		if index == 0 {
			return node.key, node.value
		} else {
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

// Computes the hash of the node without computing its descendants. Must be
// called on nodes which have descendant node hashes already computed.
func (node *Node) _hash() []byte {
	if node.hash != nil {
		return node.hash
	}

	hasher := ripemd160.New()
	buf := new(bytes.Buffer)
	if _, err := node.writeHashBytes(buf); err != nil {
		cmn.PanicCrisis(err)
	}
	hasher.Write(buf.Bytes())
	node.hash = hasher.Sum(nil)

	return node.hash
}

// Hash the node and its descendants recursively. This usually mutates all
// descendant nodes. Returns the node hash and number of nodes hashed.
func (node *Node) hashWithCount() ([]byte, int) {
	if node.hash != nil {
		return node.hash, 0
	}

	hasher := ripemd160.New()
	buf := new(bytes.Buffer)
	_, hashCount, err := node.writeHashBytesRecursively(buf)
	if err != nil {
		cmn.PanicCrisis(err)
	}
	hasher.Write(buf.Bytes())
	node.hash = hasher.Sum(nil)

	return node.hash, hashCount + 1
}

// Writes the node's hash to the given io.Writer. This function expects
// child hashes to be already set.
func (node *Node) writeHashBytes(w io.Writer) (n int, err error) {
	wire.WriteInt8(node.height, w, &n, &err)
	wire.WriteVarint(node.size, w, &n, &err)

	// Key is not written for inner nodes, unlike writeBytes.

	if node.isLeaf() {
		wire.WriteByteSlice(node.key, w, &n, &err)
		wire.WriteByteSlice(node.value, w, &n, &err)
		wire.WriteUint64(node.version, w, &n, &err)
	} else {
		if node.leftHash == nil || node.rightHash == nil {
			cmn.PanicSanity("Found an empty child hash")
		}
		wire.WriteByteSlice(node.leftHash, w, &n, &err)
		wire.WriteByteSlice(node.rightHash, w, &n, &err)
	}
	return
}

// Writes the node's hash to the given io.Writer.
// This function has the side-effect of calling hashWithCount.
func (node *Node) writeHashBytesRecursively(w io.Writer) (n int, hashCount int, err error) {
	if node.leftNode != nil {
		leftHash, leftCount := node.leftNode.hashWithCount()
		node.leftHash = leftHash
		hashCount += leftCount
	}
	if node.rightNode != nil {
		rightHash, rightCount := node.rightNode.hashWithCount()
		node.rightHash = rightHash
		hashCount += rightCount
	}
	n, err = node.writeHashBytes(w)

	return
}

// Writes the node as a serialized byte slice to the supplied io.Writer.
func (node *Node) writeBytes(w io.Writer) (n int, err error) {
	wire.WriteInt8(node.height, w, &n, &err)
	wire.WriteVarint(node.size, w, &n, &err)

	// Unlike writeHashBytes, key is written for inner nodes.
	wire.WriteByteSlice(node.key, w, &n, &err)
	wire.WriteUint64(node.version, w, &n, &err)

	if node.isLeaf() {
		wire.WriteByteSlice(node.value, w, &n, &err)
	} else {
		if node.leftHash == nil {
			cmn.PanicSanity("node.leftHash was nil in writeBytes")
		}
		wire.WriteByteSlice(node.leftHash, w, &n, &err)

		if node.rightHash == nil {
			cmn.PanicSanity("node.rightHash was nil in writeBytes")
		}
		wire.WriteByteSlice(node.rightHash, w, &n, &err)
	}
	return
}

func (node *Node) set(t *Tree, key []byte, value []byte) (
	newSelf *Node, updated bool, orphaned []*Node,
) {
	if node.isLeaf() {
		switch bytes.Compare(key, node.key) {
		case -1:
			return &Node{
				key:       node.key,
				height:    1,
				size:      2,
				leftNode:  NewNode(key, value),
				rightNode: node,
			}, false, []*Node{}
		case 1:
			return &Node{
				key:       key,
				height:    1,
				size:      2,
				leftNode:  node,
				rightNode: NewNode(key, value),
			}, false, []*Node{}
		default:
			return NewNode(key, value), true, []*Node{node}
		}
	} else {
		orphaned = append(orphaned, node)
		node = node.clone()

		if bytes.Compare(key, node.key) < 0 {
			var leftOrphaned []*Node
			node.leftNode, updated, leftOrphaned = node.getLeftNode(t).set(t, key, value)
			node.leftHash = nil // leftHash is yet unknown
			orphaned = append(orphaned, leftOrphaned...)
		} else {
			var rightOrphaned []*Node
			node.rightNode, updated, rightOrphaned = node.getRightNode(t).set(t, key, value)
			node.rightHash = nil // rightHash is yet unknown
			orphaned = append(orphaned, rightOrphaned...)
		}

		if updated {
			return node, updated, orphaned
		} else {
			node.calcHeightAndSize(t)
			newNode, balanceOrphaned := node.balance(t)
			return newNode, updated, append(orphaned, balanceOrphaned...)
		}
	}
}

// newHash/newNode: The new hash or node to replace node after remove.
// newKey: new leftmost leaf key for tree after successfully removing 'key' if changed.
// value: removed value.
func (node *Node) remove(t *Tree, key []byte) (
	newHash []byte, newNode *Node, newKey []byte, value []byte, orphaned []*Node,
) {
	if node.isLeaf() {
		if bytes.Equal(key, node.key) {
			return nil, nil, nil, node.value, []*Node{node}
		}
		return node.hash, node, nil, nil, orphaned
	}

	if bytes.Compare(key, node.key) < 0 {
		var newLeftHash []byte
		var newLeftNode *Node

		newLeftHash, newLeftNode, newKey, value, orphaned =
			node.getLeftNode(t).remove(t, key)

		if len(orphaned) == 0 {
			return node.hash, node, nil, value, orphaned
		} else if newLeftHash == nil && newLeftNode == nil { // left node held value, was removed
			return node.rightHash, node.rightNode, node.key, value, orphaned
		}
		orphaned = append(orphaned, node)

		newNode := node.clone()
		newNode.leftHash, newNode.leftNode = newLeftHash, newLeftNode
		newNode.calcHeightAndSize(t)
		newNode, balanceOrphaned := newNode.balance(t)

		return newNode.hash, newNode, newKey, value, append(orphaned, balanceOrphaned...)
	} else {
		var newRightHash []byte
		var newRightNode *Node

		newRightHash, newRightNode, newKey, value, orphaned =
			node.getRightNode(t).remove(t, key)

		if len(orphaned) == 0 {
			return node.hash, node, nil, value, orphaned
		} else if newRightHash == nil && newRightNode == nil { // right node held value, was removed
			return node.leftHash, node.leftNode, nil, value, orphaned
		}
		orphaned = append(orphaned, node)

		newNode := node.clone()
		newNode.rightHash, newNode.rightNode = newRightHash, newRightNode
		if newKey != nil {
			newNode.key = newKey
		}
		newNode.calcHeightAndSize(t)
		newNode, balanceOrphaned := newNode.balance(t)

		return newNode.hash, newNode, nil, value, append(orphaned, balanceOrphaned...)
	}
}

func (node *Node) getLeftNode(t *Tree) *Node {
	if node.leftNode != nil {
		return node.leftNode
	}
	return t.ndb.GetNode(node.leftHash)
}

func (node *Node) getRightNode(t *Tree) *Node {
	if node.rightNode != nil {
		return node.rightNode
	}
	return t.ndb.GetNode(node.rightHash)
}

// Rotate right and return the new node and orphan.
func (node *Node) rotateRight(t *Tree) (newNode *Node, orphan *Node) {
	// TODO: optimize balance & rotate.
	node = node.clone()
	l := node.getLeftNode(t)
	_l := l.clone()

	_lrHash, _lrCached := _l.rightHash, _l.rightNode
	_l.rightHash, _l.rightNode = node.hash, node
	node.leftHash, node.leftNode = _lrHash, _lrCached

	node.calcHeightAndSize(t)
	_l.calcHeightAndSize(t)

	return _l, l
}

// Rotate left and return the new node and orphan.
func (node *Node) rotateLeft(t *Tree) (newNode *Node, orphan *Node) {
	// TODO: optimize balance & rotate.
	node = node.clone()
	r := node.getRightNode(t)
	_r := r.clone()

	_rlHash, _rlCached := _r.leftHash, _r.leftNode
	_r.leftHash, _r.leftNode = node.hash, node
	node.rightHash, node.rightNode = _rlHash, _rlCached

	node.calcHeightAndSize(t)
	_r.calcHeightAndSize(t)

	return _r, r
}

// NOTE: mutates height and size
func (node *Node) calcHeightAndSize(t *Tree) {
	node.height = maxInt8(node.getLeftNode(t).height, node.getRightNode(t).height) + 1
	node.size = node.getLeftNode(t).size + node.getRightNode(t).size
}

func (node *Node) calcBalance(t *Tree) int {
	return int(node.getLeftNode(t).height) - int(node.getRightNode(t).height)
}

// NOTE: assumes that node can be modified
// TODO: optimize balance & rotate
func (node *Node) balance(t *Tree) (newSelf *Node, orphaned []*Node) {
	if node.persisted {
		panic("Unexpected balance() call on persisted node")
	}
	balance := node.calcBalance(t)

	if balance > 1 {
		if node.getLeftNode(t).calcBalance(t) >= 0 {
			// Left Left Case
			newNode, orphaned := node.rotateRight(t)
			return newNode, []*Node{orphaned}
		} else {
			// Left Right Case
			var leftOrphaned *Node

			left := node.getLeftNode(t)
			node.leftHash = nil
			node.leftNode, leftOrphaned = left.rotateLeft(t)
			newNode, rightOrphaned := node.rotateRight(t)

			return newNode, []*Node{left, leftOrphaned, rightOrphaned}
		}
	}
	if balance < -1 {
		if node.getRightNode(t).calcBalance(t) <= 0 {
			// Right Right Case
			newNode, orphaned := node.rotateLeft(t)
			return newNode, []*Node{orphaned}
		} else {
			// Right Left Case
			var rightOrphaned *Node

			right := node.getRightNode(t)
			node.rightHash = nil
			node.rightNode, rightOrphaned = right.rotateRight(t)
			newNode, leftOrphaned := node.rotateLeft(t)

			return newNode, []*Node{right, leftOrphaned, rightOrphaned}
		}
	}
	// Nothing changed
	return node, []*Node{}
}

// traverse is a wrapper over traverseInRange when we want the whole tree
func (node *Node) traverse(t *Tree, ascending bool, cb func(*Node) bool) bool {
	return node.traverseInRange(t, nil, nil, ascending, false, cb)
}

func (node *Node) traverseInRange(t *Tree, start, end []byte, ascending bool, inclusive bool, cb func(*Node) bool) bool {
	afterStart := start == nil || bytes.Compare(start, node.key) <= 0
	beforeEnd := end == nil || bytes.Compare(node.key, end) < 0
	if inclusive {
		beforeEnd = end == nil || bytes.Compare(node.key, end) <= 0
	}

	stop := false
	if afterStart && beforeEnd {
		// IterateRange ignores this if not leaf
		stop = cb(node)
	}
	if stop {
		return stop
	}
	if node.isLeaf() {
		return stop
	}

	if ascending {
		// check lower nodes, then higher
		if afterStart {
			stop = node.getLeftNode(t).traverseInRange(t, start, end, ascending, inclusive, cb)
		}
		if stop {
			return stop
		}
		if beforeEnd {
			stop = node.getRightNode(t).traverseInRange(t, start, end, ascending, inclusive, cb)
		}
	} else {
		// check the higher nodes first
		if beforeEnd {
			stop = node.getRightNode(t).traverseInRange(t, start, end, ascending, inclusive, cb)
		}
		if stop {
			return stop
		}
		if afterStart {
			stop = node.getLeftNode(t).traverseInRange(t, start, end, ascending, inclusive, cb)
		}
	}

	return stop
}

// Only used in testing...
func (node *Node) lmd(t *Tree) *Node {
	if node.isLeaf() {
		return node
	}
	return node.getLeftNode(t).lmd(t)
}
