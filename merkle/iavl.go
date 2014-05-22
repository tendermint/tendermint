package merkle

import (
    "bytes"
    "math"
    "io"
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
    return self.root
}

func (self *IAVLTree) Size() uint64 {
    return self.root.Size()
}

func (self *IAVLTree) Height() uint8 {
    return self.root.Height()
}

func (self *IAVLTree) Has(key Key) bool {
    return self.root.Has(nil, key)
}

func (self *IAVLTree) Put(key Key, value Value) (err error) {
    self.root, _ = self.root.Put(nil, key, value)
    return nil
}

func (self *IAVLTree) Hash() ([]byte, uint64) {
    return self.root.Hash()
}

func (self *IAVLTree) Get(key Key) (value Value, err error) {
    return self.root.Get(nil, key)
}

func (self *IAVLTree) Remove(key Key) (value Value, err error) {
    new_root, value, err := self.root.Remove(nil, key)
    if err != nil {
        return nil, err
    }
    self.root = new_root
    return value, nil
}

// Node

type IAVLNode struct {
    key     Key
    value   Value
    size    uint64
    height  uint8
    hash    []byte
    left    *IAVLNode
    right   *IAVLNode

    // volatile
    flags   byte
}

func (self *IAVLNode) Copy() *IAVLNode {
    if self == nil {
        return nil
    }
    return &IAVLNode{
        key:    self.key,
        value:  self.value,
        size:   self.size,
        height: self.height,
        left:   self.left,
        right:  self.right,
        hash:   nil,
        flags:  byte(0),
    }
}

func (self *IAVLNode) Key() Key {
    return self.key
}

func (self *IAVLNode) Value() Value {
    return self.value
}

func (self *IAVLNode) Left(db Db) Node {
    if self.left == nil { return nil }
    return self.left_filled(db)
}

func (self *IAVLNode) Right(db Db) Node {
    if self.right == nil { return nil }
    return self.right_filled(db)
}

func (self *IAVLNode) left_filled(db Db) *IAVLNode {
    // XXX
    return self.left
}

func (self *IAVLNode) right_filled(db Db) *IAVLNode {
    // XXX
    return self.right
}

func (self *IAVLNode) Size() uint64 {
    if self == nil {
        return 0
    }
    return self.size
}

func (self *IAVLNode) Has(db Db, key Key) (has bool) {
    if self == nil {
        return false
    }
    if self.key.Equals(key) {
        return true
    } else if key.Less(self.key) {
        return self.left_filled(db).Has(db, key)
    } else {
        return self.right_filled(db).Has(db, key)
    }
}

func (self *IAVLNode) Get(db Db, key Key) (value Value, err error) {
    if self == nil {
        return nil, NotFound(key)
    }
    if self.key.Equals(key) {
        return self.value, nil
    } else if key.Less(self.key) {
        return self.left_filled(db).Get(db, key)
    } else {
        return self.right_filled(db).Get(db, key)
    }
}

func (self *IAVLNode) Bytes() []byte {
    b := new(bytes.Buffer)
    self.WriteTo(b)
    return b.Bytes()
}

func (self *IAVLNode) Hash() ([]byte, uint64) {
    if self == nil {
        return nil, 0
    }
    if self.hash != nil {
        return self.hash, 0
    }

    hasher := sha256.New()
    _, hashCount, err := self.WriteTo(hasher)
    if err != nil { panic(err) }
    self.hash = hasher.Sum(nil)

    return self.hash, hashCount
}

func (self *IAVLNode) WriteTo(writer io.Writer) (written int64, hashCount uint64, err error) {

    write := func(bytes []byte) {
        if err == nil {
            var n int
            n, err = writer.Write(bytes)
            written += int64(n)
        }
    }

    // node descriptor
    nodeDesc := byte(0)
    if self.value != nil { nodeDesc |= 0x01 }
    if self.left != nil  { nodeDesc |= 0x02 }
    if self.right != nil { nodeDesc |= 0x04 }
    write([]byte{nodeDesc})

    // node height & size
    write(UInt8(self.height).Bytes())
    write(UInt64(self.size).Bytes())

    // node key
    keyBytes := self.key.Bytes()
    if len(keyBytes) > 255 { panic("key is too long") }
    write([]byte{byte(len(keyBytes))})
    write(keyBytes)

    // node value
    if self.value != nil {
        valueBytes := self.value.Bytes()
        if len(valueBytes) > math.MaxUint32 { panic("value is too long") }
        write([]byte{byte(len(valueBytes))})
        write(valueBytes)
    }

    // left child
    if self.left != nil {
        leftHash, leftCount := self.left.Hash()
        hashCount += leftCount
        write(leftHash)
    }

    // right child
    if self.right != nil {
        rightHash, rightCount := self.right.Hash()
        hashCount += rightCount
        write(rightHash)
    }

    return written, hashCount+1, err
}

// Returns a new tree (unless node is the root) & a copy of the popped node.
// Can only pop nodes that have one or no children.
func (self *IAVLNode) pop_node(db Db, node *IAVLNode) (new_self, new_node *IAVLNode) {
    if self == nil {
        panic("self can't be nil")
    } else if node == nil {
        panic("node can't be nil")
    } else if node.left != nil && node.right != nil {
        panic("node must not have both left and right")
    }

    if self == node {

        var n *IAVLNode
        if node.left != nil {
            n = node.left_filled(db)
        } else if node.right != nil {
            n = node.right_filled(db)
        } else {
            n = nil
        }
        node = node.Copy()
        node.left = nil
        node.right = nil
        node.calc_height_and_size(db)
        return n, node

    } else {

        self = self.Copy()
        if node.key.Less(self.key) {
            self.left, node = self.left_filled(db).pop_node(db, node)
        } else {
            self.right, node = self.right_filled(db).pop_node(db, node)
        }
        self.calc_height_and_size(db)
        return self, node

    }
}

func (self *IAVLNode) rotate_right(db Db) *IAVLNode {
    self = self.Copy()
    sl :=  self.left_filled(db).Copy()
    slr := sl.right

    sl.right = self
    self.left = slr

    self.calc_height_and_size(db)
    sl.calc_height_and_size(db)

    return sl
}

func (self *IAVLNode) rotate_left(db Db) *IAVLNode {
    self = self.Copy()
    sr :=  self.right_filled(db).Copy()
    srl := sr.left

    sr.left = self
    self.right = srl

    self.calc_height_and_size(db)
    sr.calc_height_and_size(db)

    return sr
}

func (self *IAVLNode) calc_height_and_size(db Db) {
    self.height = maxUint8(self.left_filled(db).Height(), self.right_filled(db).Height()) + 1
    self.size = self.left_filled(db).Size() + self.right_filled(db).Size() + 1
}

func (self *IAVLNode) calc_balance(db Db) int {
    if self == nil {
        return 0
    }
    return int(self.left_filled(db).Height()) - int(self.right_filled(db).Height())
}

func (self *IAVLNode) balance(db Db) (new_self *IAVLNode) {
    balance := self.calc_balance(db)
    if (balance > 1) {
        if (self.left_filled(db).calc_balance(db) >= 0) {
            // Left Left Case
            return self.rotate_right(db)
        } else {
            // Left Right Case
            self = self.Copy()
            self.left = self.left_filled(db).rotate_left(db)
            //self.calc_height_and_size()
            return self.rotate_right(db)
        }
    }
    if (balance < -1) {
        if (self.right_filled(db).calc_balance(db) <= 0) {
            // Right Right Case
            return self.rotate_left(db)
        } else {
            // Right Left Case
            self = self.Copy()
            self.right = self.right_filled(db).rotate_right(db)
            //self.calc_height_and_size()
            return self.rotate_left(db)
        }
    }
    // Nothing changed
    return self
}

// TODO: don't clear the hash if the value hasn't changed.
func (self *IAVLNode) Put(db Db, key Key, value Value) (_ *IAVLNode, updated bool) {
    if self == nil {
        return &IAVLNode{key: key, value: value, height: 1, size: 1, hash: nil}, false
    }

    self = self.Copy()

    if self.key.Equals(key) {
        self.value = value
        return self, true
    }

    if key.Less(self.key) {
        self.left, updated = self.left_filled(db).Put(db, key, value)
    } else {
        self.right, updated = self.right_filled(db).Put(db, key, value)
    }
    if updated {
        return self, updated
    } else {
        self.calc_height_and_size(db)
        return self.balance(db), updated
    }
}

func (self *IAVLNode) Remove(db Db, key Key) (new_self *IAVLNode, value Value, err error) {
    if self == nil {
        return nil, nil, NotFound(key)
    }

    if self.key.Equals(key) {
        if self.left != nil && self.right != nil {
            if self.left_filled(db).Size() < self.right_filled(db).Size() {
                self, new_self = self.pop_node(db, self.right_filled(db).lmd(db))
            } else {
                self, new_self = self.pop_node(db, self.left_filled(db).rmd(db))
            }
            new_self.left = self.left
            new_self.right = self.right
            new_self.calc_height_and_size(db)
            return new_self, self.value, nil
        } else if self.left == nil {
            return self.right_filled(db), self.value, nil
        } else if self.right == nil {
            return self.left_filled(db), self.value, nil
        } else {
            return nil, self.value, nil
        }
    }

    if key.Less(self.key) {
        if self.left == nil {
            return self, nil, NotFound(key)
        }
        var new_left *IAVLNode
        new_left, value, err = self.left_filled(db).Remove(db, key)
        if new_left == self.left_filled(db) { // not found
            return self, nil, err
        } else if err != nil { // some other error
            return self, value, err
        }
        self = self.Copy()
        self.left = new_left
    } else {
        if self.right == nil {
            return self, nil, NotFound(key)
        }
        var new_right *IAVLNode
        new_right, value, err = self.right_filled(db).Remove(db, key)
        if new_right == self.right_filled(db) { // not found
            return self, nil, err
        } else if err != nil { // some other error
            return self, value, err
        }
        self = self.Copy()
        self.right = new_right
    }
    self.calc_height_and_size(db)
    return self.balance(db), value, err
}

func (self *IAVLNode) Height() uint8 {
    if self == nil {
        return 0
    }
    return self.height
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

func (self *IAVLNode) lmd(db Db) (*IAVLNode) {
    return self._md(func(node *IAVLNode)*IAVLNode { return node.left_filled(db) })
}

func (self *IAVLNode) rmd(db Db) (*IAVLNode) {
    return self._md(func(node *IAVLNode)*IAVLNode { return node.right_filled(db) })
}

func maxUint8(a, b uint8) uint8 {
    if a > b {
        return a
    }
    return b
}
