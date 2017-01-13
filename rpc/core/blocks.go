package core

import (
	"fmt"
	. "github.com/tendermint/go-common"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------------------

// TODO: limit/permission on (max - min)
func BlockchainInfo(minHeight, maxHeight int) (*ctypes.ResultBlockchainInfo, error) {
	if maxHeight == 0 {
		maxHeight = blockStore.Height()
	} else {
		maxHeight = MinInt(blockStore.Height(), maxHeight)
	}
	if minHeight == 0 {
		minHeight = MaxInt(1, maxHeight-20)
	}
	log.Debug("BlockchainInfoHandler", "maxHeight", maxHeight, "minHeight", minHeight)

	blockMetas := []*types.BlockMeta{}
	for height := maxHeight; height >= minHeight; height-- {
		blockMeta := blockStore.LoadBlockMeta(height)
		blockMetas = append(blockMetas, blockMeta)
	}

	return &ctypes.ResultBlockchainInfo{blockStore.Height(), blockMetas}, nil
}

//-----------------------------------------------------------------------------

func Block(height int) (*ctypes.ResultBlock, error) {
	if height == 0 {
		return nil, fmt.Errorf("Height must be greater than 0")
	}
	if height > blockStore.Height() {
		return nil, fmt.Errorf("Height must be less than the current blockchain height")
	}

	blockMeta := blockStore.LoadBlockMeta(height)
	block := blockStore.LoadBlock(height)
	return &ctypes.ResultBlock{blockMeta, block}, nil
}
