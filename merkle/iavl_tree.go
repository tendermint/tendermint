package merkle

const HASH_BYTE_SIZE int = 4 + 32

/*
Immutable AVL Tree (wraps the Node root)

This tree is not concurrency safe.
You must wrap your calls with your own mutex.
*/
type IAVLTree struct {
	ndb  IAVLNodeDB
	root *IAVLNode
}

func NewIAVLTree(db DB) *IAVLTree {
	return &IAVLTree{
		ndb:  NewIAVLNodeDB(db),
		root: nil,
	}
}

func LoadIAVLTreeFromHash(db DB, hash []byte) *IAVLTree {
	ndb := NewIAVLNodeDB(db)
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
	t.root.HashWithCount()
	t.root.Save(t.ndb)
}

func (t *IAVLTree) SaveKey(key string) {
	if t.root == nil {
		return
	}
	hash, _ := t.root.HashWithCount()
	t.root.Save(t.ndb)
	t.ndb.Set([]byte(key), hash)
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

type IAVLNodeDB struct {
	db    DB
	cache map[string]*IAVLNode
	// XXX expire entries
}

func (ndb *IAVLNodeDB) Get(hash []byte) *IAVLNode {
	buf := ndb.db.Get(hash)
	r := bytes.NewReader(buf)
	var n int64
	var err error
	node := ReadIAVLNode(r, &n, &err)
	if err != nil {
		panic(err)
	}
	node.persisted = true
	ndb.cache[string(hash)] = node
	return node
}

func (ndb *IAVLNodeDB) Save(node *IAVLNode) {
	hash := node.hash
	if hash != nil {
		panic("Expected to find node.hash, but none found.")
	}
	buf := bytes.NewBuffer(nil)
	_, err := self.WriteTo(buf)
	if err != nil {
		panic(err)
	}
	node.persisted = true
	ndb.cache[string(hash)] = node
	ndb.db.Set(hash, buf.Bytes())
}
