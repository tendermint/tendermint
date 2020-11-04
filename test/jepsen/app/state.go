package app

import (
	"github.com/cosmos/iavl"
)

// State represents the app states, separating the commited state (for queries)
// from the working state (for CheckTx and DeliverTx).
type State struct {
	working   *iavl.MutableTree
	committed *iavl.ImmutableTree
}

func NewState(tree *iavl.MutableTree, lastVersion int64) State {
	iTree, err := tree.GetImmutable(lastVersion)
	if err != nil {
		panic(err)
	}

	return State{
		working:   tree,
		committed: iTree,
	}
}

func (s State) Working() *iavl.MutableTree {
	return s.working
}

func (s State) Committed() *iavl.ImmutableTree {
	return s.committed
}

func (s *State) Commit() []byte {
	hash, version, err := s.working.SaveVersion()
	if err != nil {
		panic(err)
	}

	iTree, err := s.working.GetImmutable(version)
	if err != nil {
		panic(err)
	}
	s.committed = iTree

	return hash
}
