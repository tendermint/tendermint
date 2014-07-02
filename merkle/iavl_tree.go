package merkle

import (
	. "github.com/tendermint/tendermint/binary"
)

const HASH_BYTE_SIZE int = 4 + 32

/*
Immutable AVL Tree (wraps the Node root)

This tree is not concurrency safe.
You must wrap your calls with your own mutex.
*/
type IAVLTree struct {
	db   Db
	root *IAVLNode
}

func NewIAVLTree(db Db) *IAVLTree {
	return &IAVLTree{db: db, root: nil}
}

func NewIAVLTreeFromHash(db Db, hash ByteSlice) *IAVLTree {
	root := &IAVLNode{
		hash:  hash,
		flags: IAVLNODE_FLAG_PERSISTED | IAVLNODE_FLAG_PLACEHOLDER,
	}
	root.fill(db)
	return &IAVLTree{db: db, root: root}
}

func (t *IAVLTree) Root() Node {
	return t.root
}

func (t *IAVLTree) Size() uint64 {
	if t.root == nil {
		return 0
	}
	return t.root.Size()
}

func (t *IAVLTree) Height() uint8 {
	if t.root == nil {
		return 0
	}
	return t.root.Height()
}

func (t *IAVLTree) Has(key Key) bool {
	if t.root == nil {
		return false
	}
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
	if t.root == nil {
		return nil, 0
	}
	return t.root.Hash()
}

func (t *IAVLTree) Save() {
	if t.root == nil {
		return
	}
	if t.root.hash == nil {
		t.root.Hash()
	}
	t.root.Save(t.db)
}

func (t *IAVLTree) Get(key Key) (value Value) {
	if t.root == nil {
		return nil
	}
	return t.root.get(t.db, key)
}

func (t *IAVLTree) Remove(key Key) (value Value, err error) {
	if t.root == nil {
		return nil, NotFound(key)
	}
	newRoot, _, value, err := t.root.remove(t.db, key)
	if err != nil {
		return nil, err
	}
	t.root = newRoot
	return value, nil
}

func (t *IAVLTree) Copy() Tree {
	return &IAVLTree{db: t.db, root: t.root}
}

// Traverses all the nodes of the tree in prefix order.
// return true from cb to halt iteration.
// node.Height() == 0 if you just want a value node.
func (t *IAVLTree) Traverse(cb func(Node) bool) {
	if t.root == nil {
		return
	}
	t.root.traverse(t.db, cb)
}

func (t *IAVLTree) Values() <-chan Value {
	root := t.root
	ch := make(chan Value)
	if root == nil {
		close(ch)
		return ch
	}
	go func() {
		root.traverse(t.db, func(n Node) bool {
			if n.Height() == 0 {
				ch <- n.Value()
			}
			return true
		})
		close(ch)
	}()
	return ch
}
