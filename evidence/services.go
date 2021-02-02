package evidence

import (
	"github.com/tendermint/tendermint/types"
)

//go:generate mockery --case underscore --name BlockStore

type BlockStore interface {
	LoadBlockMeta(height uint64) *types.BlockMeta
	LoadBlockCommit(height uint64) *types.Commit
}
