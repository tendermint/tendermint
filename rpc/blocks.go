package rpc

import (
	"net/http"

	. "github.com/tendermint/tendermint/block"
	. "github.com/tendermint/tendermint/common"
)

type BlockchainInfoResponse struct {
	LastHeight uint
	BlockMetas []*BlockMeta
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

	blockMetas := []*BlockMeta{}
	for height := maxHeight; height >= minHeight; height-- {
		blockMeta := blockStore.LoadBlockMeta(height)
		blockMetas = append(blockMetas, blockMeta)
	}

	res := BlockchainInfoResponse{
		LastHeight: blockStore.Height(),
		BlockMetas: blockMetas,
	}

	WriteAPIResponse(w, API_OK, res)
	return
}

//-----------------------------------------------------------------------------

type BlockResponse struct {
	BlockMeta *BlockMeta
	Block     *Block
}

func BlockHandler(w http.ResponseWriter, r *http.Request) {
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
	res := BlockResponse{
		BlockMeta: blockMeta,
		Block: block,
	}
	WriteAPIResponse(w, API_OK, res)
	return
}
