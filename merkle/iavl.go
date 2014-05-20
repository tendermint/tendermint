package merkle

import (
    "hash"
    "crypto/sha256"
)

// Immutable AVL Tree (wraps the Node root)

type IAVLTree struct {
    root *IAVLNode
}

func NewIAVLTree() *IAVLTree {
    return &IAVLTree{}
}

func (self *IAVLTree) Root() Node {
    return self.root.Copy(true)
}

func (self *IAVLTree) Size() int {
    return self.root.Size()
}

func (self *IAVLTree) Has(key Sortable) bool {
    return self.root.Has(key)
}

func (self *IAVLTree) Put(key Sortable, value interface{}) (err error) {
    self.root, _ = self.root.Put(key, value)
    return nil
}

func (self *IAVLTree) Get(key Sortable) (value interface{}, err error) {
    return self.root.Get(key)
}

func (self *IAVLTree) Remove(key Sortable) (value interface{}, err error) {
    new_root, value, err := self.root.Remove(key)
    if err != nil {
        return nil, err
    }
    self.root = new_root
    return value, nil
}

// Node

type IAVLNode struct {
    key     Sortable
    value   interface{}
    height  int
    hash    []byte
    left    *IAVLNode
    right   *IAVLNode
}

func (self *IAVLNode) Copy(copyHash bool) *IAVLNode {
    if self == nil {
        return nil
    }
    var hash []byte
    if copyHash {
        hash = self.hash
    }
    return &IAVLNode{
        key:    self.key,
        value:  self.value,
        height: self.height,
        hash:   hash,
        left:   self.left,
        right:  self.right,
    }
}

func (self *IAVLNode) Has(key Sortable) (has bool) {
    if self == nil {
        return false
    }
    if self.key.Equals(key) {
        return true
    } else if key.Less(self.key) {
        return self.left.Has(key)
    } else {
        return self.right.Has(key)
    }
}

func (self *IAVLNode) Get(key Sortable) (value interface{}, err error) {
    if self == nil {
        return nil, NotFound(key)
    }
    if self.key.Equals(key) {
        return self.value, nil
    } else if key.Less(self.key) {
        return self.left.Get(key)
    } else {
        return self.right.Get(key)
    }
}

// Copies and pops node from the tree.
// Returns a new tree (unless node is the root) & new (popped) node.
func (self *IAVLNode) pop_node(node *IAVLNode) (new_self, new_node *IAVLNode) {
    if node == nil {
        panic("node can't be nil")
    } else if node.left != nil && node.right != nil {
        panic("node must not have both left and right")
    }

    if self == nil {
        return nil, node.Copy(true)
    } else if self == node {
        var n *IAVLNode
        if node.left != nil {
            n = node.left
        } else if node.right != nil {
            n = node.right
        } else {
            n = nil
        }
        node = node.Copy(false)
        node.left = nil
        node.right = nil
        return n, node
    }

    self = self.Copy(false)

    if node.key.Less(self.key) {
        self.left, node = self.left.pop_node(node)
    } else {
        self.right, node = self.right.pop_node(node)
    }

    self.height = max(self.left.Height(), self.right.Height()) + 1
    return self, node
}

// Pushes the node to the tree, returns a new tree
func (self *IAVLNode) push_node(node *IAVLNode) *IAVLNode {
    if node == nil {
        panic("node can't be nil")
    } else if node.left != nil || node.right != nil {
        panic("node must now be a leaf")
    }

    self = self.Copy(false)

    if self == nil {
        node.height = 1
        return node
    } else if node.key.Less(self.key) {
        self.left = self.left.push_node(node)
    } else {
        self.right = self.right.push_node(node)
    }
    self.height = max(self.left.Height(), self.right.Height()) + 1
    return self
}

func (self *IAVLNode) rotate_right() *IAVLNode {
    if self == nil {
        return self
    }
    if self.left == nil {
        return self
    }
    return self.rotate(self.left.rmd)
}

func (self *IAVLNode) rotate_left() *IAVLNode {
    if self == nil {
        return self
    }
    if self.right == nil {
        return self
    }
    return self.rotate(self.right.lmd)
}

func (self *IAVLNode) rotate(get_new_root func() *IAVLNode) *IAVLNode {
    self, new_root := self.pop_node(get_new_root())
    new_root.left = self.left
    new_root.right = self.right
    self.hash = nil
    self.left = nil
    self.right = nil
    return new_root.push_node(self)
}

func (self *IAVLNode) balance() *IAVLNode {
    if self == nil {
        return self
    }
    for abs(self.left.Height() - self.right.Height()) > 2 {
        if self.left.Height() > self.right.Height() {
            self = self.rotate_right()
        } else {
            self = self.rotate_left()
        }
    }
    return self
}

// TODO: don't clear the hash if the value hasn't changed.
func (self *IAVLNode) Put(key Sortable, value interface{}) (_ *IAVLNode, updated bool) {
    if self == nil {
        return &IAVLNode{key: key, value: value, height: 1, hash: nil}, false
    }

    self = self.Copy(false)

    if self.key.Equals(key) {
        self.value = value
        return self, true
    }

    if key.Less(self.key) {
        self.left, updated = self.left.Put(key, value)
    } else {
        self.right, updated = self.right.Put(key, value)
    }
    self.height = max(self.left.Height(), self.right.Height()) + 1

    if !updated {
        self.height += 1
        return self.balance(), updated
    }
    return self, updated
}

func (self *IAVLNode) Remove(key Sortable) (_ *IAVLNode, value interface{}, err error) {
    if self == nil {
        return nil, nil, NotFound(key)
    }

    if self.key.Equals(key) {
        if self.left != nil && self.right != nil {
            var new_root *IAVLNode
            if self.left.Size() < self.right.Size() {
                self, new_root = self.pop_node(self.right.lmd())
            } else {
                self, new_root = self.pop_node(self.left.rmd())
            }
            new_root.left = self.left
            new_root.right = self.right
            return new_root, self.value, nil
        } else if self.left == nil {
            return self.right, self.value, nil
        } else if self.right == nil {
            return self.left, self.value, nil
        } else {
            return nil, self.value, nil
        }
    }

    self = self.Copy(true)

    if key.Less(self.key) {
        self.left, value, err = self.left.Remove(key)
    } else {
        self.right, value, err = self.right.Remove(key)
    }
    if err == nil {
        self.hash = nil
        self.height = max(self.left.Height(), self.right.Height()) + 1
        return self.balance(), value, err
    } else {
        return self, value, err
    }
}

func (self *IAVLNode) Height() int {
    if self == nil {
        return 0
    }
    return self.height
}

func (self *IAVLNode) Size() int {
    if self == nil {
        return 0
    }
    return 1 + self.left.Size() + self.right.Size()
}


func (self *IAVLNode) Key() Sortable {
    return self.key
}

func (self *IAVLNode) Value() interface{} {
    return self.value
}

func (self *IAVLNode) Left() Node {
    if self.left == nil {
        return nil
    }
    return self.left
}

func (self *IAVLNode) Right() Node {
    if self.right == nil {
        return nil
    }
    return self.right
}

// ...

func (self *IAVLNode) _md(side func(*IAVLNode)*IAVLNode) (*IAVLNode) {
    if self == nil {
        return nil
    } else if side(self) != nil {
        return side(self)._md(side)
    } else {
        return self
    }
}

func (self *IAVLNode) lmd() (*IAVLNode) {
    return self._md(func(node *IAVLNode)*IAVLNode { return node.left })
}

func (self *IAVLNode) rmd() (*IAVLNode) {
    return self._md(func(node *IAVLNode)*IAVLNode { return node.right })
}

func abs(i int) int {
    if i < 0 {
        return -i
    }
    return i
}

func max(a, b int) int {
    if a > b {
        return a
    }
    return b
}

// Calculate the hash of hasher over buf.
func CalcHash(buf []byte, hasher hash.Hash) []byte {
    hasher.Write(buf)
    return hasher.Sum(nil)
}

// calculate hash256 which is sha256(sha256(data))
func CalcSha256(buf []byte) []byte {
    return CalcHash(buf, sha256.New())
}
