package app

import (
	"github.com/cosmos/iavl"
)

// State represents the app states, separating the commited state (for queries)
// from the working state (for CheckTx and AppendTx)
type State struct {
	deliverTx  *iavl.MutableTree
	checkTx    *iavl.ImmutableTree
	persistent bool
}

func NewState(tree *iavl.MutableTree, version int64, persistent bool) State {
	iTree, err := tree.GetImmutable(0)
	if err != nil {
		panic(err)
	}
	return State{
		deliverTx:  tree,
		checkTx:    iTree,
		persistent: persistent,
	}
}

func (s State) Deliver() *iavl.MutableTree {
	return s.deliverTx
}

func (s State) Check() *iavl.ImmutableTree {
	return s.checkTx
}

// Commit stores the current Deliver() state as committed
// starts new Append/Check state, and
// returns the hash for the commit
func (s *State) Commit() []byte {
	hash, version, err := s.deliverTx.SaveVersion()
	if err != nil {
		panic(err)
	}

	iTree, err := s.deliverTx.GetImmutable(version)
	if err != nil {
		panic(err)
	}
	s.checkTx = iTree

	return hash
}
