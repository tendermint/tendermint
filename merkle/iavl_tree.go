package merkle

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
	return &IAVLTree{
		db:   db,
		root: nil,
	}
}

// TODO rename to Load.
func NewIAVLTreeFromHash(db Db, hash []byte) *IAVLTree {
	root := &IAVLNode{
		hash:  hash,
		flags: IAVLNODE_FLAG_PERSISTED | IAVLNODE_FLAG_PLACEHOLDER,
	}
	root.fill(db)
	return &IAVLTree{db: db, root: root}
}

func NewIAVLTreeFromKey(db Db, key string) *IAVLTree {
	hash := db.Get([]byte(key))
	if hash == nil {
		return nil
	}
	root := &IAVLNode{
		hash:  hash,
		flags: IAVLNODE_FLAG_PERSISTED | IAVLNODE_FLAG_PLACEHOLDER,
	}
	root.fill(db)
	return &IAVLTree{db: db, root: root}
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
	return t.root.has(t.db, key)
}

func (t *IAVLTree) Set(key []byte, value []byte) (updated bool) {
	if t.root == nil {
		t.root = NewIAVLNode(key, value)
		return false
	}
	t.root, updated = t.root.set(t.db, key, value)
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
	t.root.Save(t.db)
}

func (t *IAVLTree) SaveKey(key string) {
	if t.root == nil {
		return
	}
	hash, _ := t.root.HashWithCount()
	t.root.Save(t.db)
	t.db.Set([]byte(key), hash)
}

func (t *IAVLTree) Get(key []byte) (value []byte) {
	if t.root == nil {
		return nil
	}
	return t.root.get(t.db, key)
}

func (t *IAVLTree) Remove(key []byte) (value []byte, err error) {
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
