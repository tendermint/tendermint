package iavl

import (
	"fmt"

	"github.com/pkg/errors"
	dbm "github.com/tendermint/tmlibs/db"
)

var ErrVersionDoesNotExist = fmt.Errorf("version does not exist")

// VersionedTree is a persistent tree which keeps track of versions.
type VersionedTree struct {
	*orphaningTree                  // The current, working tree.
	versions       map[uint64]*Tree // The previous, saved versions of the tree.
	latestVersion  uint64           // The latest saved version.
	ndb            *nodeDB
}

// NewVersionedTree returns a new tree with the specified cache size and datastore.
func NewVersionedTree(cacheSize int, db dbm.DB) *VersionedTree {
	ndb := newNodeDB(cacheSize, db)
	head := &Tree{ndb: ndb}

	return &VersionedTree{
		orphaningTree: newOrphaningTree(head),
		versions:      map[uint64]*Tree{},
		ndb:           ndb,
	}
}

// LatestVersion returns the latest saved version of the tree.
func (tree *VersionedTree) LatestVersion() uint64 {
	return tree.latestVersion
}

// IsEmpty returns whether or not the tree has any keys. Only trees that are
// not empty can be saved.
func (tree *VersionedTree) IsEmpty() bool {
	return tree.orphaningTree.Size() == 0
}

// VersionExists returns whether or not a version exists.
func (tree *VersionedTree) VersionExists(version uint64) bool {
	_, ok := tree.versions[version]
	return ok
}

// Tree returns the current working tree.
func (tree *VersionedTree) Tree() *Tree {
	return tree.orphaningTree.Tree
}

// Hash returns the hash of the latest saved version of the tree, as returned
// by SaveVersion. If no versions have been saved, Hash returns nil.
func (tree *VersionedTree) Hash() []byte {
	if tree.latestVersion > 0 {
		return tree.versions[tree.latestVersion].Hash()
	}
	return nil
}

// String returns a string representation of the tree.
func (tree *VersionedTree) String() string {
	return tree.ndb.String()
}

// Set sets a key in the working tree. Nil values are not supported.
func (tree *VersionedTree) Set(key, val []byte) bool {
	return tree.orphaningTree.Set(key, val)
}

// Remove removes a key from the working tree.
func (tree *VersionedTree) Remove(key []byte) ([]byte, bool) {
	return tree.orphaningTree.Remove(key)
}

// Load a versioned tree from disk. All tree versions are loaded automatically.
func (tree *VersionedTree) Load() error {
	roots, err := tree.ndb.getRoots()
	if err != nil {
		return err
	}
	if len(roots) == 0 {
		return nil
	}

	// Load all roots from the database.
	for version, root := range roots {
		t := &Tree{ndb: tree.ndb}
		t.load(root)

		tree.versions[version] = t

		if version > tree.latestVersion {
			tree.latestVersion = version
		}
	}

	// Set the working tree to a copy of the latest.
	tree.orphaningTree = newOrphaningTree(
		tree.versions[tree.latestVersion].clone(),
	)

	return nil
}

// GetVersioned gets the value at the specified key and version.
func (tree *VersionedTree) GetVersioned(key []byte, version uint64) (
	index int, value []byte,
) {
	if t, ok := tree.versions[version]; ok {
		return t.Get(key)
	}
	return -1, nil
}

// SaveVersion saves a new tree version to disk, based on the current state of
// the tree. Multiple calls to SaveVersion with the same version are not allowed.
func (tree *VersionedTree) SaveVersion(version uint64) ([]byte, error) {
	if _, ok := tree.versions[version]; ok {
		return nil, errors.Errorf("version %d was already saved", version)
	}
	if tree.root == nil {
		return nil, ErrNilRoot
	}
	if version == 0 {
		return nil, errors.New("version must be greater than zero")
	}
	if version <= tree.latestVersion {
		return nil, errors.Errorf("version must be greater than latest (%d <= %d)",
			version, tree.latestVersion)
	}

	tree.latestVersion = version
	tree.versions[version] = tree.orphaningTree.Tree

	tree.orphaningTree.SaveVersion(version)
	tree.orphaningTree = newOrphaningTree(
		tree.versions[version].clone(),
	)

	tree.ndb.SaveRoot(tree.root, version)
	tree.ndb.Commit()

	return tree.root.hash, nil
}

// DeleteVersion deletes a tree version from disk. The version can then no
// longer be accessed.
func (tree *VersionedTree) DeleteVersion(version uint64) error {
	if version == 0 {
		return errors.New("version must be greater than 0")
	}
	if version == tree.latestVersion {
		return errors.Errorf("cannot delete latest saved version (%d)", version)
	}
	if _, ok := tree.versions[version]; !ok {
		return errors.WithStack(ErrVersionDoesNotExist)
	}

	tree.ndb.DeleteVersion(version)
	tree.ndb.Commit()

	delete(tree.versions, version)

	return nil
}

// GetVersionedWithProof gets the value under the key at the specified version
// if it exists, or returns nil.  A proof of existence or absence is returned
// alongside the value.
func (tree *VersionedTree) GetVersionedWithProof(key []byte, version uint64) ([]byte, KeyProof, error) {
	if t, ok := tree.versions[version]; ok {
		return t.GetWithProof(key)
	}
	return nil, nil, errors.WithStack(ErrVersionDoesNotExist)
}

// GetVersionedRangeWithProof gets key/value pairs within the specified range
// and limit. To specify a descending range, swap the start and end keys.
//
// Returns a list of keys, a list of values and a proof.
func (tree *VersionedTree) GetVersionedRangeWithProof(startKey, endKey []byte, limit int, version uint64) ([][]byte, [][]byte, *KeyRangeProof, error) {
	if t, ok := tree.versions[version]; ok {
		return t.GetRangeWithProof(startKey, endKey, limit)
	}
	return nil, nil, nil, errors.WithStack(ErrVersionDoesNotExist)
}

// GetVersionedFirstInRangeWithProof gets the first key/value pair in the
// specified range, with a proof.
func (tree *VersionedTree) GetVersionedFirstInRangeWithProof(startKey, endKey []byte, version uint64) ([]byte, []byte, *KeyFirstInRangeProof, error) {
	if t, ok := tree.versions[version]; ok {
		return t.GetFirstInRangeWithProof(startKey, endKey)
	}
	return nil, nil, nil, errors.WithStack(ErrVersionDoesNotExist)
}

// GetVersionedLastInRangeWithProof gets the last key/value pair in the
// specified range, with a proof.
func (tree *VersionedTree) GetVersionedLastInRangeWithProof(startKey, endKey []byte, version uint64) ([]byte, []byte, *KeyLastInRangeProof, error) {
	if t, ok := tree.versions[version]; ok {
		return t.GetLastInRangeWithProof(startKey, endKey)
	}
	return nil, nil, nil, errors.WithStack(ErrVersionDoesNotExist)
}
