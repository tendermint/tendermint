package state

import (
	. "github.com/tendermint/tendermint/blocks"
)

// XXX ugh, bad name.
type StateStore struct {
}

func (ss *StateStore) StageBlock(block *Block) error {
	// XXX implement staging.
	return nil
}

func (ss *StateStore) CommitBlock(block *Block) error {
	// XXX implement staging.
	return nil
}
