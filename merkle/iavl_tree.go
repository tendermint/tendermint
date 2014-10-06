package merkle

import (
	"bytes"
	"container/list"
)

const defaultCacheCapacity = 1000 // TODO make configurable.

// XXX Make Codec tree.

/*
Immutable AVL Tree (wraps the Node root)

This tree is not concurrency safe.
You must wrap your calls with your own mutex.
*/
type IAVLTree struct {
	ndb  *IAVLNodeDB
	root *IAVLNode
}

func NewIAVLTree(db DB) *IAVLTree {
	return &IAVLTree{
		ndb:  NewIAVLNodeDB(defaultCacheCapacity, db),
		root: nil,
	}
}

func LoadIAVLTreeFromHash(db DB, hash []byte) *IAVLTree {
	ndb := NewIAVLNodeDB(defaultCacheCapacity, db)
	root := ndb.Get(hash)
	if root == nil {
		return nil
	}
	return &IAVLTree{ndb: ndb, root: root}
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

func (t *IAVLTree) Has(key []byte) bool {
	if t.root == nil {
		return false
	}
	return t.root.has(t.ndb, key)
}

func (t *IAVLTree) Set(key []byte, value []byte) (updated bool) {
	if t.root == nil {
		t.root = NewIAVLNode(key, value)
		return false
	}
	t.root, updated = t.root.set(t.ndb, key, value)
	return updated
}

func (t *IAVLTree) Hash() []byte {
	if t.root == nil {
		return nil
	}
	hash, _ := t.root.HashWithCount()
	return hash
}

func (t *IAVLTree) HashWithCount() ([]byte, uint64) {
	if t.root == nil {
		return nil, 0
	}
	return t.root.HashWithCount()
}

func (t *IAVLTree) Save() {
	if t.root == nil {
		return
	}
	t.root.Save(t.ndb)
}

func (t *IAVLTree) Get(key []byte) (value []byte) {
	if t.root == nil {
		return nil
	}
	return t.root.get(t.ndb, key)
}

func (t *IAVLTree) Remove(key []byte) (value []byte, err error) {
	if t.root == nil {
		return nil, NotFound(key)
	}
	newRootHash, newRoot, _, value, err := t.root.remove(t.ndb, key)
	if err != nil {
		return nil, err
	}
	if newRoot == nil && newRootHash != nil {
		t.root = t.ndb.Get(newRootHash)
	} else {
		t.root = newRoot
	}
	return value, nil
}

func (t *IAVLTree) Copy() Tree {
	return &IAVLTree{ndb: t.ndb, root: t.root}
}

//-----------------------------------------------------------------------------

type nodeElement struct {
	node *IAVLNode
	elem *list.Element
}

type IAVLNodeDB struct {
	capacity int
	db       DB
	cache    map[string]nodeElement
	queue    *list.List
}

func NewIAVLNodeDB(capacity int, db DB) *IAVLNodeDB {
	return &IAVLNodeDB{
		capacity: capacity,
		db:       db,
		cache:    make(map[string]nodeElement),
		queue:    list.New(),
	}
}

func (ndb *IAVLNodeDB) Get(hash []byte) *IAVLNode {
	// Check the cache.
	nodeElem, ok := ndb.cache[string(hash)]
	if ok {
		// Already exists. Move to back of queue.
		ndb.queue.MoveToBack(nodeElem.elem)
		return nodeElem.node
	} else {
		// Doesn't exist, load.
		buf := ndb.db.Get(hash)
		r := bytes.NewReader(buf)
		var n int64
		var err error
		node := ReadIAVLNode(r, &n, &err)
		if err != nil {
			panic(err)
		}
		node.persisted = true
		ndb.cacheNode(node)
		return node
	}
}

func (ndb *IAVLNodeDB) Save(node *IAVLNode) {
	if node.hash == nil {
		panic("Expected to find node.hash, but none found.")
	}
	if node.persisted {
		panic("Shouldn't be calling save on an already persisted node.")
	}
	if _, ok := ndb.cache[string(node.hash)]; ok {
		panic("Shouldn't be calling save on an already cached node.")
	}
	// Save node bytes to db
	buf := bytes.NewBuffer(nil)
	_, err := node.WriteTo(buf)
	if err != nil {
		panic(err)
	}
	ndb.db.Set(node.hash, buf.Bytes())
	node.persisted = true
	ndb.cacheNode(node)
}

func (ndb *IAVLNodeDB) cacheNode(node *IAVLNode) {
	// Create entry in cache and append to queue.
	elem := ndb.queue.PushBack(node.hash)
	ndb.cache[string(node.hash)] = nodeElement{node, elem}
	// Maybe expire an item.
	if ndb.queue.Len() > ndb.capacity {
		hash := ndb.queue.Remove(ndb.queue.Front()).([]byte)
		delete(ndb.cache, string(hash))
	}
}
