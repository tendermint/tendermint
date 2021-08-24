package evidence

import (
	"github.com/tendermint/tendermint/pkg/block"
	"github.com/tendermint/tendermint/pkg/metadata"
)

//go:generate ../../scripts/mockery_generate.sh BlockStore

type BlockStore interface {
	LoadBlockMeta(height int64) *block.BlockMeta
	LoadBlockCommit(height int64) *metadata.Commit
	Height() int64
}
