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
	}
	if minHeight == 0 {
		minHeight = MaxUint(0, maxHeight-20)
	}

	blockMetas := []*BlockMeta{}
	for height := minHeight; height <= maxHeight; height++ {
		blockMetas = append(blockMetas, blockStore.LoadBlockMeta(height))
	}

	res := BlockchainInfoResponse{
		LastHeight: blockStore.Height(),
		BlockMetas: blockMetas,
	}

	WriteAPIResponse(w, API_OK, res)
	return
}
