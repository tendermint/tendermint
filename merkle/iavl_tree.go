package merkle

import (
	"bytes"
	"container/list"
	"sync"

	"github.com/tendermint/tendermint/wire"
	. "github.com/tendermint/tendermint/common"
	dbm "github.com/tendermint/tendermint/db"
)

/*
Immutable AVL Tree (wraps the Node root)
This tree is not goroutine safe.
*/
type IAVLTree struct {
	keyCodec   wire.Codec
	valueCodec wire.Codec
	root       *IAVLNode
	ndb        *nodeDB
}

func NewIAVLTree(keyCodec, valueCodec wire.Codec, cacheSize int, db dbm.DB) *IAVLTree {
	if db == nil {
		// In-memory IAVLTree
		return &IAVLTree{
			keyCodec:   keyCodec,
			valueCodec: valueCodec,
		}
	} else {
		// Persistent IAVLTree
		return &IAVLTree{
			keyCodec:   keyCodec,
			valueCodec: valueCodec,
			ndb:        newNodeDB(cacheSize, db),
		}
	}
}

// The returned tree and the original tree are goroutine independent.
// That is, they can each run in their own goroutine.
func (t *IAVLTree) Copy() Tree {
	if t.root == nil {
		return &IAVLTree{
			keyCodec:   t.keyCodec,
			valueCodec: t.valueCodec,
			root:       nil,
			ndb:        t.ndb,
		}
	}
	if t.ndb != nil && !t.root.persisted {
		// Saving a tree finalizes all the nodes.
		// It sets all the hashes recursively,
		// clears all the leftNode/rightNode values recursively,
		// and all the .persisted flags get set.
		PanicSanity("It is unsafe to Copy() an unpersisted tree.")
	} else if t.ndb == nil && t.root.hash == nil {
		// An in-memory IAVLTree is finalized when the hashes are
		// calculated.
		t.root.hashWithCount(t)
	}
	return &IAVLTree{
		keyCodec:   t.keyCodec,
		valueCodec: t.valueCodec,
		root:       t.root,
		ndb:        t.ndb,
	}
}

func (t *IAVLTree) Size() int {
	if t.root == nil {
		return 0
	}
	return t.root.size
}

func (t *IAVLTree) Height() int8 {
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

func (t *IAVLTree) HashWithCount() ([]byte, int) {
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

// Sets the root node by reading from db.
// If the hash is empty, then sets root to nil.
func (t *IAVLTree) Load(hash []byte) {
	if len(hash) == 0 {
		t.root = nil
	} else {
		t.root = t.ndb.GetNode(t, hash)
	}
}

func (t *IAVLTree) Get(key interface{}) (index int, value interface{}) {
	if t.root == nil {
		return 0, nil
	}
	return t.root.get(t, key)
}

func (t *IAVLTree) GetByIndex(index int) (key interface{}, value interface{}) {
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
		t.root = t.ndb.GetNode(t, newRootHash)
	} else {
		t.root = newRoot
	}
	return value, true
}

func (t *IAVLTree) Iterate(fn func(key interface{}, value interface{}) bool) (stopped bool) {
	if t.root == nil {
		return false
	}
	return t.root.traverse(t, func(node *IAVLNode) bool {
		if node.height == 0 {
			return fn(node.key, node.value)
		} else {
			return false
		}
	})
}

//-----------------------------------------------------------------------------

type nodeElement struct {
	node *IAVLNode
	elem *list.Element
}

type nodeDB struct {
	mtx        sync.Mutex
	cache      map[string]nodeElement
	cacheSize  int
	cacheQueue *list.List
	db         dbm.DB
}

func newNodeDB(cacheSize int, db dbm.DB) *nodeDB {
	return &nodeDB{
		cache:      make(map[string]nodeElement),
		cacheSize:  cacheSize,
		cacheQueue: list.New(),
		db:         db,
	}
}

func (ndb *nodeDB) GetNode(t *IAVLTree, hash []byte) *IAVLNode {
	ndb.mtx.Lock()
	defer ndb.mtx.Unlock()
	// Check the cache.
	nodeElem, ok := ndb.cache[string(hash)]
	if ok {
		// Already exists. Move to back of cacheQueue.
		ndb.cacheQueue.MoveToBack(nodeElem.elem)
		return nodeElem.node
	} else {
		// Doesn't exist, load.
		buf := ndb.db.Get(hash)
		if len(buf) == 0 {
			ndb.db.(*dbm.LevelDB).Print()
			PanicSanity(Fmt("Value missing for key %X", hash))
		}
		r := bytes.NewReader(buf)
		var n int64
		var err error
		node := ReadIAVLNode(t, r, &n, &err)
		if err != nil {
			PanicCrisis(Fmt("Error reading IAVLNode. bytes: %X  error: %v", buf, err))
		}
		node.hash = hash
		node.persisted = true
		ndb.cacheNode(node)
		return node
	}
}

func (ndb *nodeDB) SaveNode(t *IAVLTree, node *IAVLNode) {
	ndb.mtx.Lock()
	defer ndb.mtx.Unlock()
	if node.hash == nil {
		PanicSanity("Expected to find node.hash, but none found.")
	}
	if node.persisted {
		PanicSanity("Shouldn't be calling save on an already persisted node.")
	}
	/*if _, ok := ndb.cache[string(node.hash)]; ok {
		panic("Shouldn't be calling save on an already cached node.")
	}*/
	// Save node bytes to db
	buf := bytes.NewBuffer(nil)
	_, err := node.writePersistBytes(t, buf)
	if err != nil {
		PanicCrisis(err)
	}
	ndb.db.Set(node.hash, buf.Bytes())
	node.persisted = true
	ndb.cacheNode(node)
}

func (ndb *nodeDB) cacheNode(node *IAVLNode) {
	// Create entry in cache and append to cacheQueue.
	elem := ndb.cacheQueue.PushBack(node.hash)
	ndb.cache[string(node.hash)] = nodeElement{node, elem}
	// Maybe expire an item.
	if ndb.cacheQueue.Len() > ndb.cacheSize {
		hash := ndb.cacheQueue.Remove(ndb.cacheQueue.Front()).([]byte)
		delete(ndb.cache, string(hash))
	}
}
