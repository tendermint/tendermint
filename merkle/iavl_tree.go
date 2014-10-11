package merkle

import (
	"bytes"
	"container/list"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/db"
)

const defaultCacheCapacity = 1000 // TODO make configurable.

/*
Immutable AVL Tree (wraps the Node root)

This tree is not concurrency safe.
You must wrap your calls with your own mutex.
*/
type IAVLTree struct {
	keyCodec   Codec
	valueCodec Codec
	root       *IAVLNode

	// Cache
	cache     map[string]nodeElement
	cacheSize int
	queue     *list.List

	// Persistence
	db DB
}

func NewIAVLTree(keyCodec, valueCodec Codec, cacheSize int, db DB) *IAVLTree {
	return &IAVLTree{
		keyCodec:   keyCodec,
		valueCodec: valueCodec,
		root:       nil,
		cache:      make(map[string]nodeElement),
		cacheSize:  cacheSize,
		queue:      list.New(),
		db:         db,
	}
}

func LoadIAVLTreeFromHash(keyCodec, valueCodec Codec, cacheSize int, db DB, hash []byte) *IAVLTree {
	t := NewIAVLTree(keyCodec, valueCodec, cacheSize, db)
	t.root = t.getNode(hash)
	return t
}

func (t *IAVLTree) Size() uint64 {
	if t.root == nil {
		return 0
	}
	return t.root.size
}

func (t *IAVLTree) Height() uint8 {
	if t.root == nil {
		return 0
	}
	return t.root.height
}

func (t *IAVLTree) Has(key interface{}) bool {
	if t.root == nil {
		return false
	}
	return t.root.has(t, key)
}

func (t *IAVLTree) Set(key interface{}, value interface{}) (updated bool) {
	if t.root == nil {
		t.root = NewIAVLNode(key, value)
		return false
	}
	t.root, updated = t.root.set(t, key, value)
	return updated
}

func (t *IAVLTree) Hash() []byte {
	if t.root == nil {
		return nil
	}
	hash, _ := t.root.hashWithCount(t)
	return hash
}

func (t *IAVLTree) HashWithCount() ([]byte, uint64) {
	if t.root == nil {
		return nil, 0
	}
	return t.root.hashWithCount(t)
}

func (t *IAVLTree) Save() []byte {
	if t.root == nil {
		return nil
	}
	return t.root.save(t)
}

func (t *IAVLTree) Get(key interface{}) (index uint64, value interface{}) {
	if t.root == nil {
		return 0, nil
	}
	return t.root.get(t, key)
}

func (t *IAVLTree) GetByIndex(index uint64) (key interface{}, value interface{}) {
	if t.root == nil {
		return nil, nil
	}
	return t.root.getByIndex(t, index)
}

func (t *IAVLTree) Remove(key interface{}) (value interface{}, removed bool) {
	if t.root == nil {
		return nil, false
	}
	newRootHash, newRoot, _, value, removed := t.root.remove(t, key)
	if !removed {
		return nil, false
	}
	if newRoot == nil && newRootHash != nil {
		t.root = t.getNode(newRootHash)
	} else {
		t.root = newRoot
	}
	return value, true
}

func (t *IAVLTree) Checkpoint() interface{} {
	return t.root
}

func (t *IAVLTree) Restore(checkpoint interface{}) {
	t.root = checkpoint.(*IAVLNode)
}

type nodeElement struct {
	node *IAVLNode
	elem *list.Element
}

func (t *IAVLTree) getNode(hash []byte) *IAVLNode {
	// Check the cache.
	nodeElem, ok := t.cache[string(hash)]
	if ok {
		// Already exists. Move to back of queue.
		t.queue.MoveToBack(nodeElem.elem)
		return nodeElem.node
	} else {
		// Doesn't exist, load.
		buf := t.db.Get(hash)
		r := bytes.NewReader(buf)
		var n int64
		var err error
		node := ReadIAVLNode(t, r, &n, &err)
		if err != nil {
			panic(err)
		}
		node.persisted = true
		t.cacheNode(node)
		return node
	}
}

func (t *IAVLTree) cacheNode(node *IAVLNode) {
	// Create entry in cache and append to queue.
	elem := t.queue.PushBack(node.hash)
	t.cache[string(node.hash)] = nodeElement{node, elem}
	// Maybe expire an item.
	if t.queue.Len() > t.cacheSize {
		hash := t.queue.Remove(t.queue.Front()).([]byte)
		delete(t.cache, string(hash))
	}
}

func (t *IAVLTree) saveNode(node *IAVLNode) {
	if node.hash == nil {
		panic("Expected to find node.hash, but none found.")
	}
	if node.persisted {
		panic("Shouldn't be calling save on an already persisted node.")
	}
	if _, ok := t.cache[string(node.hash)]; ok {
		panic("Shouldn't be calling save on an already cached node.")
	}
	// Save node bytes to db
	buf := bytes.NewBuffer(nil)
	_, _, err := node.writeToCountHashes(t, buf)
	if err != nil {
		panic(err)
	}
	t.db.Set(node.hash, buf.Bytes())
	node.persisted = true
	t.cacheNode(node)
}
