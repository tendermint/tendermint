package core

import (
	"fmt"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------------------

func BlockchainInfoHandler(minHeight, maxHeight uint) (uint, []*types.BlockMeta) {
	if maxHeight == 0 {
		maxHeight = blockStore.Height()
	} else {
		maxHeight = MinUint(blockStore.Height(), maxHeight)
	}
	if minHeight == 0 {
		minHeight = uint(MaxInt(1, int(maxHeight)-20))
	}
	log.Debug("BlockchainInfoHandler", "maxHeight", maxHeight, "minHeight", minHeight)

	blockMetas := []*types.BlockMeta{}
	for height := maxHeight; height >= minHeight; height-- {
		blockMeta := blockStore.LoadBlockMeta(height)
		blockMetas = append(blockMetas, blockMeta)
	}

	return blockStore.Height(), blockMetas
}

//-----------------------------------------------------------------------------

func GetBlockHandler(height uint) (*types.BlockMeta, *types.Block, error) {
	if height == 0 {
		return nil, nil, fmt.Errorf("height must be greater than 1")
	}
	if height > blockStore.Height() {
		return nil, nil, fmt.Errorf("height must be less than the current blockchain height")
	}

	blockMeta := blockStore.LoadBlockMeta(height)
	block := blockStore.LoadBlock(height)
	return blockMeta, block, nil
}
