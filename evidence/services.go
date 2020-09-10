package evidence

import (
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

//go:generate mockery --case underscore --name BlockStore

type BlockStore interface {
	LoadBlockMeta(height int64) *types.BlockMeta
	LoadBlockCommit(height int64) *types.Commit
}

//go:generate mockery --case underscore --name StateStore

type StateStore interface {
	LoadValidators(height int64) (*types.ValidatorSet, error)
	LoadState() state.State
}
