package rpc

import (
	"net/http"

	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------------------

// Request: {}

type ResponseBlockchainInfo struct {
	LastHeight uint
	BlockMetas []*types.BlockMeta
}

func BlockchainInfoHandler(w http.ResponseWriter, r *http.Request) {
	minHeight, _ := GetParamUint(r, "min_height")
	maxHeight, _ := GetParamUint(r, "max_height")
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

	WriteAPIResponse(w, API_OK, ResponseBlockchainInfo{blockStore.Height(), blockMetas})
}

//-----------------------------------------------------------------------------

// Request: {"height": uint}

type ResponseGetBlock struct {
	BlockMeta *types.BlockMeta
	Block     *types.Block
}

func GetBlockHandler(w http.ResponseWriter, r *http.Request) {
	height, _ := GetParamUint(r, "height")
	if height == 0 {
		WriteAPIResponse(w, API_INVALID_PARAM, "height must be greater than 1")
		return
	}
	if height > blockStore.Height() {
		WriteAPIResponse(w, API_INVALID_PARAM, "height must be less than the current blockchain height")
		return
	}

	blockMeta := blockStore.LoadBlockMeta(height)
	block := blockStore.LoadBlock(height)

	WriteAPIResponse(w, API_OK, ResponseGetBlock{blockMeta, block})
}
