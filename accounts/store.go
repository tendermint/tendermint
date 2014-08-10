package accounts

import (
	. "github.com/tendermint/tendermint/blocks"
)

type AccountStore struct {
}

func (as *AccountStore) StageBlock(block *Block) error {
	// XXX implement staging.
	return nil
}

func (as *AccountStore) CommitBlock(block *Block) error {
	// XXX implement staging.
	return nil
}
