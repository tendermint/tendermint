package merkle

import (
	"bytes"
	"crypto/sha256"
	. "github.com/tendermint/tendermint/binary"
	"io"
)

// Node

type IAVLNode struct {
	key       []byte
	value     []byte
	size      uint64
	height    uint8
	hash      []byte
	leftHash  []byte
	leftNode  *IAVLNode
	rightHash []byte
	rightNode *IAVLNode
	persisted bool
}

func NewIAVLNode(key []byte, value []byte) *IAVLNode {
	return &IAVLNode{
		key:   key,
		value: value,
		size:  1,
	}
}

func ReadIAVLNode(r io.Reader, n *int64, err *error) *IAVLNode {
	node := &IAVLNode{}

	// node header & key
	node.height = ReadUInt8(r, n, err)
	node.size = ReadUInt64(r, n, err)
	node.key = ReadByteSlice(r, n, err)
	if *err != nil {
		panic(*err)
	}

	// node value or children.
	if node.height == 0 {
		node.value = ReadByteSlice(r, n, err)
	} else {
		node.leftHash = ReadByteSlice(r, n, err)
		node.rightHash = ReadByteSlice(r, n, err)
	}
	if *err != nil {
		panic(*err)
	}
	return node
}

func (self *IAVLNode) Copy() *IAVLNode {
	if self.height == 0 {
		panic("Why are you copying a value node?")
	}
	return &IAVLNode{
		key:       self.key,
		size:      self.size,
		height:    self.height,
		hash:      nil, // Going to be mutated anyways.
		leftHash:  self.leftHash,
		leftNode:  self.leftNode,
		rightHash: self.rightHash,
		rightNode: self.rightNode,
		persisted: self.persisted,
	}
}

func (self *IAVLNode) Size() uint64 {
	return self.size
}

func (self *IAVLNode) Height() uint8 {
	return self.height
}

func (self *IAVLNode) has(ndb *IAVLNodeDB, key []byte) (has bool) {
	if bytes.Equal(self.key, key) {
		return true
	}
	if self.height == 0 {
		return false
	} else {
		if bytes.Compare(key, self.key) == -1 {
			return self.getLeftNode(ndb).has(ndb, key)
		} else {
			return self.getRightNode(ndb).has(ndb, key)
		}
	}
}

func (self *IAVLNode) get(ndb *IAVLNodeDB, key []byte) (value []byte) {
	if self.height == 0 {
		if bytes.Equal(self.key, key) {
			return self.value
		} else {
			return nil
		}
	} else {
		if bytes.Compare(key, self.key) == -1 {
			return self.getLeftNode(ndb).get(ndb, key)
		} else {
			return self.getRightNode(ndb).get(ndb, key)
		}
	}
}

func (self *IAVLNode) HashWithCount() ([]byte, uint64) {
	if self.hash != nil {
		return self.hash, 0
	}

	hasher := sha256.New()
	_, hashCount, err := self.writeToCountHashes(hasher)
	if err != nil {
		panic(err)
	}
	self.hash = hasher.Sum(nil)

	return self.hash, hashCount + 1
}

func (self *IAVLNode) Save(ndb *IAVLNodeDB) []byte {
	if self.hash == nil {
		self.hash, _ = self.HashWithCount()
	}
	if self.persisted {
		return self.hash
	}

	// save children
	if self.leftNode != nil {
		self.leftHash = self.leftNode.Save(ndb)
		self.leftNode = nil
	}
	if self.rightNode != nil {
		self.rightHash = self.rightNode.Save(ndb)
		self.rightNode = nil
	}

	// save self
	ndb.Save(self)
	return self.hash
}

func (self *IAVLNode) set(ndb *IAVLNodeDB, key []byte, value []byte) (_ *IAVLNode, updated bool) {
	if self.height == 0 {
		if bytes.Compare(key, self.key) == -1 {
			return &IAVLNode{
				key:       self.key,
				height:    1,
				size:      2,
				leftNode:  NewIAVLNode(key, value),
				rightNode: self,
			}, false
		} else if bytes.Equal(self.key, key) {
			return NewIAVLNode(key, value), true
		} else {
			return &IAVLNode{
				key:       key,
				height:    1,
				size:      2,
				leftNode:  self,
				rightNode: NewIAVLNode(key, value),
			}, false
		}
	} else {
		self = self.Copy()
		if bytes.Compare(key, self.key) == -1 {
			self.leftNode, updated = self.getLeftNode(ndb).set(ndb, key, value)
			self.leftHash = nil
		} else {
			self.rightNode, updated = self.getRightNode(ndb).set(ndb, key, value)
			self.rightHash = nil
		}
		if updated {
			return self, updated
		} else {
			self.calcHeightAndSize(ndb)
			return self.balance(ndb), updated
		}
	}
}

// newHash/newNode: The new hash or node to replace self after remove.
// newKey: new leftmost leaf key for tree after successfully removing 'key' if changed.
// value: removed value.
func (self *IAVLNode) remove(ndb *IAVLNodeDB, key []byte) (
	newHash []byte, newNode *IAVLNode, newKey []byte, value []byte, err error) {
	if self.height == 0 {
		if bytes.Equal(self.key, key) {
			return nil, nil, nil, self.value, nil
		} else {
			return nil, self, nil, nil, NotFound(key)
		}
	} else {
		if bytes.Compare(key, self.key) == -1 {
			var newLeftHash []byte
			var newLeftNode *IAVLNode
			newLeftHash, newLeftNode, newKey, value, err = self.getLeftNode(ndb).remove(ndb, key)
			if err != nil {
				return nil, self, nil, value, err
			} else if newLeftHash == nil && newLeftNode == nil { // left node held value, was removed
				return self.rightHash, self.rightNode, self.key, value, nil
			}
			self = self.Copy()
			self.leftHash, self.leftNode = newLeftHash, newLeftNode
		} else {
			var newRightHash []byte
			var newRightNode *IAVLNode
			newRightHash, newRightNode, newKey, value, err = self.getRightNode(ndb).remove(ndb, key)
			if err != nil {
				return nil, self, nil, value, err
			} else if newRightHash == nil && newRightNode == nil { // right node held value, was removed
				return self.leftHash, self.leftNode, nil, value, nil
			}
			self = self.Copy()
			self.rightHash, self.rightNode = newRightHash, newRightNode
			if newKey != nil {
				self.key = newKey
				newKey = nil
			}
		}
		self.calcHeightAndSize(ndb)
		return nil, self.balance(ndb), newKey, value, err
	}
}

func (self *IAVLNode) WriteTo(w io.Writer) (n int64, err error) {
	n, _, err = self.writeToCountHashes(w)
	return
}

func (self *IAVLNode) writeToCountHashes(w io.Writer) (n int64, hashCount uint64, err error) {
	// height & size & key
	WriteUInt8(w, self.height, &n, &err)
	WriteUInt64(w, self.size, &n, &err)
	WriteByteSlice(w, self.key, &n, &err)
	if err != nil {
		return
	}

	if self.height == 0 {
		// value
		WriteByteSlice(w, self.value, &n, &err)
	} else {
		// left
		if self.leftNode != nil {
			leftHash, leftCount := self.leftNode.HashWithCount()
			self.leftHash = leftHash
			hashCount += leftCount
		}
		if self.leftHash == nil {
			panic("self.leftHash was nil in save")
		}
		WriteByteSlice(w, self.leftHash, &n, &err)
		// right
		if self.rightNode != nil {
			rightHash, rightCount := self.rightNode.HashWithCount()
			self.rightHash = rightHash
			hashCount += rightCount
		}
		if self.rightHash == nil {
			panic("self.rightHash was nil in save")
		}
		WriteByteSlice(w, self.rightHash, &n, &err)
	}
	return
}

func (self *IAVLNode) getLeftNode(ndb *IAVLNodeDB) *IAVLNode {
	if self.leftNode != nil {
		return self.leftNode
	} else {
		return ndb.Get(self.leftHash)
	}
}

func (self *IAVLNode) getRightNode(ndb *IAVLNodeDB) *IAVLNode {
	if self.rightNode != nil {
		return self.rightNode
	} else {
		return ndb.Get(self.rightHash)
	}
}

func (self *IAVLNode) rotateRight(ndb *IAVLNodeDB) *IAVLNode {
	self = self.Copy()
	sl := self.getLeftNode(ndb).Copy()

	slrHash, slrCached := sl.rightHash, sl.rightNode
	sl.rightHash, sl.rightNode = nil, self
	self.leftHash, self.leftNode = slrHash, slrCached

	self.calcHeightAndSize(ndb)
	sl.calcHeightAndSize(ndb)

	return sl
}

func (self *IAVLNode) rotateLeft(ndb *IAVLNodeDB) *IAVLNode {
	self = self.Copy()
	sr := self.getRightNode(ndb).Copy()

	srlHash, srlCached := sr.leftHash, sr.leftNode
	sr.leftHash, sr.leftNode = nil, self
	self.rightHash, self.rightNode = srlHash, srlCached

	self.calcHeightAndSize(ndb)
	sr.calcHeightAndSize(ndb)

	return sr
}

func (self *IAVLNode) calcHeightAndSize(ndb *IAVLNodeDB) {
	self.height = maxUint8(self.getLeftNode(ndb).Height(), self.getRightNode(ndb).Height()) + 1
	self.size = self.getLeftNode(ndb).Size() + self.getRightNode(ndb).Size()
}

func (self *IAVLNode) calcBalance(ndb *IAVLNodeDB) int {
	return int(self.getLeftNode(ndb).Height()) - int(self.getRightNode(ndb).Height())
}

func (self *IAVLNode) balance(ndb *IAVLNodeDB) (newSelf *IAVLNode) {
	balance := self.calcBalance(ndb)
	if balance > 1 {
		if self.getLeftNode(ndb).calcBalance(ndb) >= 0 {
			// Left Left Case
			return self.rotateRight(ndb)
		} else {
			// Left Right Case
			self = self.Copy()
			self.leftHash, self.leftNode = nil, self.getLeftNode(ndb).rotateLeft(ndb)
			//self.calcHeightAndSize()
			return self.rotateRight(ndb)
		}
	}
	if balance < -1 {
		if self.getRightNode(ndb).calcBalance(ndb) <= 0 {
			// Right Right Case
			return self.rotateLeft(ndb)
		} else {
			// Right Left Case
			self = self.Copy()
			self.rightHash, self.rightNode = nil, self.getRightNode(ndb).rotateRight(ndb)
			//self.calcHeightAndSize()
			return self.rotateLeft(ndb)
		}
	}
	// Nothing changed
	return self
}

func (self *IAVLNode) traverse(ndb *IAVLNodeDB, cb func(*IAVLNode) bool) bool {
	stop := cb(self)
	if stop {
		return stop
	}
	if self.height > 0 {
		stop = self.getLeftNode(ndb).traverse(ndb, cb)
		if stop {
			return stop
		}
		stop = self.getRightNode(ndb).traverse(ndb, cb)
		if stop {
			return stop
		}
	}
	return false
}

// Only used in testing...
func (self *IAVLNode) lmd(ndb *IAVLNodeDB) *IAVLNode {
	if self.height == 0 {
		return self
	}
	return self.getLeftNode(ndb).lmd(ndb)
}

// Only used in testing...
func (self *IAVLNode) rmd(ndb *IAVLNodeDB) *IAVLNode {
	if self.height == 0 {
		return self
	}
	return self.getRightNode(ndb).rmd(ndb)
}
