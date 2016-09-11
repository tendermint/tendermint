package proxy

import (
	"github.com/tendermint/tendermint/types"
)

type State interface {
	ReplayBlocks([]byte, *types.Header, types.PartSetHeader, AppConnConsensus, BlockStore) error
}

type BlockStore interface {
	Height() int
	LoadBlockMeta(height int) *types.BlockMeta
	LoadBlock(height int) *types.Block
}
