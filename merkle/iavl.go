/*
This tree is not concurrency safe.
If you want to use it from multiple goroutines, you need to wrap all calls to *IAVLTree
with a mutex.
*/

package merkle

import (
    . "github.com/tendermint/tendermint/binary"
    "bytes"
    "io"
    "crypto/sha256"
)

const HASH_BYTE_SIZE int = 4+32

// Immutable AVL Tree (wraps the Node root)

type IAVLTree struct {
    db      Db
    root    *IAVLNode
}

func NewIAVLTree(db Db) *IAVLTree {
    return &IAVLTree{db:db, root:nil}
}

func NewIAVLTreeFromHash(db Db, hash ByteSlice) *IAVLTree {
    root := &IAVLNode{
        hash:   hash,
        flags:  IAVLNODE_FLAG_PERSISTED | IAVLNODE_FLAG_PLACEHOLDER,
    }
    root.fill(db)
    return &IAVLTree{db:db, root:root}
}

func (t *IAVLTree) Root() Node {
    return t.root
}

func (t *IAVLTree) Size() uint64 {
    if t.root == nil { return 0 }
    return t.root.Size()
}

func (t *IAVLTree) Height() uint8 {
    if t.root == nil { return 0 }
    return t.root.Height()
}

func (t *IAVLTree) Has(key Key) bool {
    if t.root == nil { return false }
    return t.root.has(t.db, key)
}

func (t *IAVLTree) Put(key Key, value Value) (updated bool) {
    if t.root == nil {
        t.root = NewIAVLNode(key, value)
        return false
    }
    t.root, updated = t.root.put(t.db, key, value)
    return updated
}

func (t *IAVLTree) Hash() (ByteSlice, uint64) {
    if t.root == nil { return nil, 0 }
    return t.root.Hash()
}

func (t *IAVLTree) Save() {
    if t.root == nil { return }
    if t.root.hash == nil {
        t.root.Hash()
    }
    t.root.Save(t.db)
}

func (t *IAVLTree) Get(key Key) (value Value) {
    if t.root == nil { return nil }
    return t.root.get(t.db, key)
}

func (t *IAVLTree) Remove(key Key) (value Value, err error) {
    if t.root == nil { return nil, NotFound(key) }
    newRoot, _, value, err := t.root.remove(t.db, key)
    if err != nil {
        return nil, err
    }
    t.root = newRoot
    return value, nil
}

func (t *IAVLTree) Copy() Tree {
    return &IAVLTree{db:t.db, root:t.root}
}

// Traverses all the nodes of the tree in prefix order.
// return true from cb to halt iteration.
// node.Height() == 0 if you just want a value node.
func (t *IAVLTree) Traverse(cb func(Node) bool) {
    if t.root == nil { return }
    t.root.traverse(t.db, cb)
}

func (t *IAVLTree) Values() <-chan Value {
    root := t.root
    ch := make(chan Value)
    go func() {
        root.traverse(t.db, func(n Node) bool {
            if n.Height() == 0 { ch <- n.Value() }
            return true
        })
        close(ch)
    }()
    return ch
}


// Node

type IAVLNode struct {
    key     Key
    value   Value
    size    uint64
    height  uint8
    hash    ByteSlice
    left    *IAVLNode
    right   *IAVLNode

    // volatile
    flags   byte
}

const (
    IAVLNODE_FLAG_PERSISTED =   byte(0x01)
    IAVLNODE_FLAG_PLACEHOLDER = byte(0x02)
)

func NewIAVLNode(key Key, value Value) *IAVLNode {
    return &IAVLNode{
        key:    key,
        value:  value,
        size:   1,
    }
}

func (self *IAVLNode) Copy() *IAVLNode {
    if self.height == 0 {
        panic("Why are you copying a value node?")
    }
    return &IAVLNode{
        key:    self.key,
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

func (self *IAVLNode) Size() uint64 {
    return self.size
}

func (self *IAVLNode) Height() uint8 {
    return self.height
}

func (self *IAVLNode) has(db Db, key Key) (has bool) {
    if self.key.Equals(key) {
        return true
    }
    if self.height == 0 {
        return false
    } else {
        if key.Less(self.key) {
            return self.leftFilled(db).has(db, key)
        } else {
            return self.rightFilled(db).has(db, key)
        }
    }
}

func (self *IAVLNode) get(db Db, key Key) (value Value) {
    if self.height == 0 {
        if self.key.Equals(key) {
            return self.value
        } else {
            return nil
        }
    } else {
        if key.Less(self.key) {
            return self.leftFilled(db).get(db, key)
        } else {
            return self.rightFilled(db).get(db, key)
        }
    }
}

func (self *IAVLNode) Hash() (ByteSlice, uint64) {
    if self.hash != nil {
        return self.hash, 0
    }

    hasher := sha256.New()
    _, hashCount, err := self.saveToCountHashes(hasher, false)
    if err != nil { panic(err) }
    self.hash = hasher.Sum(nil)

    return self.hash, hashCount+1
}

func (self *IAVLNode) Save(db Db) {
    if self.hash == nil {
        panic("savee.hash can't be nil")
    }
    if self.flags & IAVLNODE_FLAG_PERSISTED > 0 ||
       self.flags & IAVLNODE_FLAG_PLACEHOLDER > 0 {
        return
    }

    // children
    if self.height > 0 {
        self.left.Save(db)
        self.right.Save(db)
    }

    // save self
    buf := bytes.NewBuffer(nil)
    _, err := self.WriteTo(buf)
    if err != nil { panic(err) }
    db.Put([]byte(self.hash), buf.Bytes())

    self.flags |= IAVLNODE_FLAG_PERSISTED
}

func (self *IAVLNode) put(db Db, key Key, value Value) (_ *IAVLNode, updated bool) {
    if self.height == 0 {
        if key.Less(self.key) {
            return &IAVLNode{
                key:    self.key,
                height: 1,
                size:   2,
                left:   NewIAVLNode(key, value),
                right:  self,
            }, false
        } else if self.key.Equals(key) {
            return NewIAVLNode(key, value), true
        } else {
            return &IAVLNode{
                key:    key,
                height: 1,
                size:   2,
                left:   self,
                right:  NewIAVLNode(key, value),
            }, false
        }
    } else {
        self = self.Copy()
        if key.Less(self.key) {
            self.left, updated = self.leftFilled(db).put(db, key, value)
        } else {
            self.right, updated = self.rightFilled(db).put(db, key, value)
        }
        if updated {
            return self, updated
        } else {
            self.calcHeightAndSize(db)
            return self.balance(db), updated
        }
    }
}

// newKey: new leftmost leaf key for tree after successfully removing 'key' if changed.
func (self *IAVLNode) remove(db Db, key Key) (newSelf *IAVLNode, newKey Key, value Value, err error) {
    if self.height == 0 {
        if self.key.Equals(key) {
            return nil, nil, self.value, nil
        } else {
            return self, nil, nil, NotFound(key)
        }
    } else {
        if key.Less(self.key) {
            var newLeft *IAVLNode
            newLeft, newKey, value, err = self.leftFilled(db).remove(db, key)
            if err != nil {
                return self, nil, value, err
            } else if newLeft == nil { // left node held value, was removed
                return self.right, self.key, value, nil
            }
            self = self.Copy()
            self.left = newLeft
        } else {
            var newRight *IAVLNode
            newRight, newKey, value, err = self.rightFilled(db).remove(db, key)
            if err != nil {
                return self, nil, value, err
            } else if newRight == nil { // right node held value, was removed
                return self.left, nil, value, nil
            }
            self = self.Copy()
            self.right = newRight
            if newKey != nil {
                self.key = newKey
                newKey = nil
            }
        }
        self.calcHeightAndSize(db)
        return self.balance(db), newKey, value, err
    }
}

func (self *IAVLNode) WriteTo(w io.Writer) (n int64, err error) {
    n, _, err = self.saveToCountHashes(w, true)
    return
}

func (self *IAVLNode) saveToCountHashes(w io.Writer, meta bool) (n int64, hashCount uint64, err error) {
    var _n int64

    if meta {
        // height & size
        _n, err = UInt8(self.height).WriteTo(w)
        if err != nil { return } else { n += _n }
        _n, err = UInt64(self.size).WriteTo(w)
        if err != nil { return } else { n += _n }

        // key
        _n, err = Byte(GetBinaryType(self.key)).WriteTo(w)
        if err != nil { return } else { n += _n }
        _n, err = self.key.WriteTo(w)
        if err != nil { return } else { n += _n }
    }

    if self.height == 0 {
        // value
        _n, err = Byte(GetBinaryType(self.value)).WriteTo(w)
        if err != nil { return } else { n += _n }
        if self.value != nil {
            _n, err = self.value.WriteTo(w)
            if err != nil { return } else { n += _n }
        }
    } else {
        // left
        leftHash, leftCount := self.left.Hash()
        hashCount += leftCount
        _n, err = leftHash.WriteTo(w)
        if err != nil { return } else { n += _n }
        // right
        rightHash, rightCount := self.right.Hash()
        hashCount += rightCount
        _n, err = rightHash.WriteTo(w)
        if err != nil { return } else { n += _n }
    }

    return
}

// Given a placeholder node which has only the hash set,
// load the rest of the data from db.
// Not threadsafe.
func (self *IAVLNode) fill(db Db) {
    if self.hash == nil {
        panic("placeholder.hash can't be nil")
    }
    buf := db.Get(self.hash)
    r := bytes.NewReader(buf)
    // node header
    self.height = uint8(ReadUInt8(r))
    self.size = uint64(ReadUInt64(r))
    // key
    key := ReadBinary(r)
    self.key = key.(Key)

    if self.height == 0 {
        // value
        self.value = ReadBinary(r)
    } else {
        // left
        var leftHash ByteSlice
        leftHash = ReadByteSlice(r)
        self.left = &IAVLNode{
            hash:   leftHash,
            flags:  IAVLNODE_FLAG_PERSISTED | IAVLNODE_FLAG_PLACEHOLDER,
        }
        // right
        var rightHash ByteSlice
        rightHash = ReadByteSlice(r)
        self.right = &IAVLNode{
            hash:   rightHash,
            flags:  IAVLNODE_FLAG_PERSISTED | IAVLNODE_FLAG_PLACEHOLDER,
        }
        if r.Len() != 0 {
            panic("buf not all consumed")
        }
    }
    self.flags &= ^IAVLNODE_FLAG_PLACEHOLDER
}

func (self *IAVLNode) leftFilled(db Db) *IAVLNode {
    if self.left.flags & IAVLNODE_FLAG_PLACEHOLDER > 0 {
        self.left.fill(db)
    }
    return self.left
}

func (self *IAVLNode) rightFilled(db Db) *IAVLNode {
    if self.right.flags & IAVLNODE_FLAG_PLACEHOLDER > 0 {
        self.right.fill(db)
    }
    return self.right
}

func (self *IAVLNode) rotateRight(db Db) *IAVLNode {
    self = self.Copy()
    sl :=  self.leftFilled(db).Copy()
    slr := sl.right

    sl.right = self
    self.left = slr

    self.calcHeightAndSize(db)
    sl.calcHeightAndSize(db)

    return sl
}

func (self *IAVLNode) rotateLeft(db Db) *IAVLNode {
    self = self.Copy()
    sr :=  self.rightFilled(db).Copy()
    srl := sr.left

    sr.left = self
    self.right = srl

    self.calcHeightAndSize(db)
    sr.calcHeightAndSize(db)

    return sr
}

func (self *IAVLNode) calcHeightAndSize(db Db) {
    self.height = maxUint8(self.leftFilled(db).Height(), self.rightFilled(db).Height()) + 1
    self.size = self.leftFilled(db).Size() + self.rightFilled(db).Size()
}

func (self *IAVLNode) calcBalance(db Db) int {
    return int(self.leftFilled(db).Height()) - int(self.rightFilled(db).Height())
}

func (self *IAVLNode) balance(db Db) (newSelf *IAVLNode) {
    balance := self.calcBalance(db)
    if (balance > 1) {
        if (self.leftFilled(db).calcBalance(db) >= 0) {
            // Left Left Case
            return self.rotateRight(db)
        } else {
            // Left Right Case
            self = self.Copy()
            self.left = self.leftFilled(db).rotateLeft(db)
            //self.calcHeightAndSize()
            return self.rotateRight(db)
        }
    }
    if (balance < -1) {
        if (self.rightFilled(db).calcBalance(db) <= 0) {
            // Right Right Case
            return self.rotateLeft(db)
        } else {
            // Right Left Case
            self = self.Copy()
            self.right = self.rightFilled(db).rotateRight(db)
            //self.calcHeightAndSize()
            return self.rotateLeft(db)
        }
    }
    // Nothing changed
    return self
}

func (self *IAVLNode) lmd(db Db) (*IAVLNode) {
    if self.height == 0 {
        return self
    }
    return self.leftFilled(db).lmd(db)
}

func (self *IAVLNode) rmd(db Db) (*IAVLNode) {
    if self.height == 0 {
        return self
    }
    return self.rightFilled(db).rmd(db)
}

func (self *IAVLNode) traverse(db Db, cb func(Node)bool) bool {
    stop := cb(self)
    if stop { return stop }
    if self.height > 0 {
        stop = self.leftFilled(db).traverse(db, cb)
        if stop { return stop }
        stop = self.rightFilled(db).traverse(db, cb)
        if stop { return stop }
    }
    return false
}
