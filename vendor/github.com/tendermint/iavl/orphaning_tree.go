package iavl

import (
	cmn "github.com/tendermint/tmlibs/common"
)

// orphaningTree is a tree which keeps track of orphaned nodes.
type orphaningTree struct {
	*Tree

	// A map of orphan hash to orphan version.
	// The version stored here is the one at which the orphan's lifetime
	// begins.
	orphans map[string]uint64
}

// newOrphaningTree creates a new orphaning tree from the given *Tree.
func newOrphaningTree(t *Tree) *orphaningTree {
	return &orphaningTree{
		Tree:    t,
		orphans: map[string]uint64{},
	}
}

// Set a key on the underlying tree while storing the orphaned nodes.
func (tree *orphaningTree) Set(key, value []byte) bool {
	orphaned, updated := tree.Tree.set(key, value)
	tree.addOrphans(orphaned)
	return updated
}

// Remove a key from the underlying tree while storing the orphaned nodes.
func (tree *orphaningTree) Remove(key []byte) ([]byte, bool) {
	val, orphaned, removed := tree.Tree.remove(key)
	tree.addOrphans(orphaned)
	return val, removed
}

// Unorphan undoes the orphaning of a node, removing the orphan entry on disk
// if necessary.
func (tree *orphaningTree) unorphan(hash []byte) {
	tree.deleteOrphan(hash)
	tree.ndb.Unorphan(hash)
}

// Save the underlying Tree. Saves orphans too.
func (tree *orphaningTree) SaveVersion(version uint64) {
	// Save the current tree at the given version. For each saved node, we
	// delete any existing orphan entries in the previous trees.
	// This is necessary because sometimes tree re-balancing causes nodes to be
	// incorrectly marked as orphaned, since tree patterns after a re-balance
	// may mirror previous tree patterns, with matching hashes.
	tree.ndb.SaveBranch(tree.root, func(node *Node) {
		// The node version is set here since it isn't known until we save.
		// Note that we only want to set the version for inner nodes the first
		// time, as they represent the beginning of the lifetime of that node.
		// So unless it's a leaf node, we only update version when it's 0.
		if node.version == 0 || node.isLeaf() {
			node.version = version
		}
		tree.unorphan(node._hash())
	})
	tree.ndb.SaveOrphans(version, tree.orphans)
}

// Add orphans to the orphan list. Doesn't write to disk.
func (tree *orphaningTree) addOrphans(orphans []*Node) {
	for _, node := range orphans {
		if !node.persisted {
			// We don't need to orphan nodes that were never persisted.
			continue
		}
		if len(node.hash) == 0 {
			cmn.PanicSanity("Expected to find node hash, but was empty")
		}
		tree.orphans[string(node.hash)] = node.version
	}
}

// Delete an orphan from the orphan list. Doesn't write to disk.
func (tree *orphaningTree) deleteOrphan(hash []byte) (version uint64, deleted bool) {
	if version, ok := tree.orphans[string(hash)]; ok {
		delete(tree.orphans, string(hash))
		return version, true
	}
	return 0, false
}
