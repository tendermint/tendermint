package common

import ()

// This immutable balanced binary tree happens to be an
// immutable AVL+ tree, adapted from tendermint/merkle.
// Unlike that one, this is in-memory, non-merkleized,
// and nodes can be nil to signify an empty tree.
type IBBSTree struct {
	key    uint64
	value  interface{}
	size   uint64
	height uint8
	left   *IBBSTree
	right  *IBBSTree
}

// Creates an empty tree.
func NewIBBSTree() *IBBSTree {
	return nil
}

// Creates a single tree node from key and value.
func NewIBBSTreeNode(key uint64, value interface{}) *IBBSTree {
	return &IBBSTree{
		key:   key,
		value: value,
		size:  1,
	}
}

func (self *IBBSTree) Copy() *IBBSTree {
	if self == nil {
		return nil
	}
	return &IBBSTree{
		key:    self.key,
		value:  self.value,
		size:   self.size,
		height: self.height,
		left:   self.left,
		right:  self.right,
	}
}

func (self *IBBSTree) Size() uint64 {
	if self == nil {
		return 0
	}
	return self.size
}

func (self *IBBSTree) Height() uint8 {
	if self == nil {
		return 0
	}
	return self.height
}

func (self *IBBSTree) Has(key uint64) (has bool) {
	if self == nil {
		return false
	}
	if self.key == key {
		return true
	}
	if self.height == 0 {
		return false
	} else {
		if key < self.key {
			return self.left.Has(key)
		} else {
			return self.right.Has(key)
		}
	}
}

func (self *IBBSTree) Get(key uint64) (value interface{}) {
	if self == nil {
		return nil
	}
	if self.height == 0 {
		if self.key == key {
			return self.value
		} else {
			return nil
		}
	} else {
		if key < self.key {
			return self.left.Get(key)
		} else {
			return self.right.Get(key)
		}
	}
}

func (self *IBBSTree) Set(key uint64, value interface{}) (_ *IBBSTree, updated bool) {
	if self == nil {
		return NewIBBSTreeNode(key, value), false
	}
	if self.height == 0 {
		if key < self.key {
			return &IBBSTree{
				key:    self.key,
				height: 1,
				size:   2,
				left:   NewIBBSTreeNode(key, value),
				right:  self,
			}, false
		} else if self.key == key {
			return NewIBBSTreeNode(key, value), true
		} else {
			return &IBBSTree{
				key:    key,
				height: 1,
				size:   2,
				left:   self,
				right:  NewIBBSTreeNode(key, value),
			}, false
		}
	} else {
		self = self.Copy()
		if key < self.key {
			self.left, updated = self.left.Set(key, value)
		} else {
			self.right, updated = self.right.Set(key, value)
		}
		if updated {
			return self, updated
		} else {
			self.calcHeightAndSize()
			return self.balance(), updated
		}
	}
}

func (self *IBBSTree) Remove(key uint64) (newSelf *IBBSTree, value interface{}, removed bool) {
	newSelf, _, _, value, removed = self.remove(key)
	return
}

// newKey: new leftmost leaf key for tree after successfully removing 'key' if changed.
func (self *IBBSTree) remove(key uint64) (newSelf *IBBSTree, hasNewKey bool, newKey uint64, value interface{}, removed bool) {
	if self == nil {
		return nil, false, 0, nil, false
	}
	if self.height == 0 {
		if self.key == key {
			return nil, false, 0, self.value, true
		} else {
			return self, false, 0, nil, false
		}
	} else {
		if key < self.key {
			var newLeft *IBBSTree
			newLeft, hasNewKey, newKey, value, removed = self.left.remove(key)
			if !removed {
				return self, false, 0, value, false
			} else if newLeft == nil { // left node held value, was removed
				return self.right, true, self.key, value, true
			}
			self = self.Copy()
			self.left = newLeft
		} else {
			var newRight *IBBSTree
			newRight, hasNewKey, newKey, value, removed = self.right.remove(key)
			if !removed {
				return self, false, 0, value, false
			} else if newRight == nil { // right node held value, was removed
				return self.left, false, 0, value, true
			}
			self = self.Copy()
			self.right = newRight
			if hasNewKey {
				self.key = newKey
				hasNewKey = false
				newKey = 0
			}
		}
		self.calcHeightAndSize()
		return self.balance(), hasNewKey, newKey, value, true
	}
}

func (self *IBBSTree) rotateRight() *IBBSTree {
	self = self.Copy()
	sl := self.left.Copy()
	slr := sl.right

	sl.right = self
	self.left = slr

	self.calcHeightAndSize()
	sl.calcHeightAndSize()

	return sl
}

func (self *IBBSTree) rotateLeft() *IBBSTree {
	self = self.Copy()
	sr := self.right.Copy()
	srl := sr.left

	sr.left = self
	self.right = srl

	self.calcHeightAndSize()
	sr.calcHeightAndSize()

	return sr
}

func (self *IBBSTree) calcHeightAndSize() {
	self.height = MaxUint8(self.left.Height(), self.right.Height()) + 1
	self.size = self.left.Size() + self.right.Size()
}

func (self *IBBSTree) calcBalance() int {
	return int(self.left.Height()) - int(self.right.Height())
}

func (self *IBBSTree) balance() (newSelf *IBBSTree) {
	balance := self.calcBalance()
	if balance > 1 {
		if self.left.calcBalance() >= 0 {
			// Left Left Case
			return self.rotateRight()
		} else {
			// Left Right Case
			self = self.Copy()
			self.left = self.left.rotateLeft()
			//self.calcHeightAndSize()
			return self.rotateRight()
		}
	}
	if balance < -1 {
		if self.right.calcBalance() <= 0 {
			// Right Right Case
			return self.rotateLeft()
		} else {
			// Right Left Case
			self = self.Copy()
			self.right = self.right.rotateRight()
			//self.calcHeightAndSize()
			return self.rotateLeft()
		}
	}
	// Nothing changed
	return self
}

// Iteration stops when stop is returned.
func (self *IBBSTree) Iterate(cb func(uint64, interface{}) bool) bool {
	if self == nil {
		return false
	}
	if self.height == 0 {
		return cb(self.key, self.value)
	} else {
		stop := self.left.Iterate(cb)
		if stop {
			return stop
		}
		return self.right.Iterate(cb)
	}
}
