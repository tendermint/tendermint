package blockchain

import (
	"github.com/tendermint/tendermint/blockchain"
	"github.com/tendermint/tendermint/types"
)

var maxBCMessageSize = (types.DefaultConsensusParams()).BlockSizeParams.MaxBytes + 2

func Fuzz(data []byte) int {
	_, msg, err := blockchain.DecodeMessage(data, maxBCMessageSize)
	if msg != nil && err == nil {
		return 1
	}
	if msg != nil && err != nil {
		return 0
	}
	if len(data) != maxBCMessageSize {
		if err == nil || msg != nil {
			return 0
		}
	}
	return -1
}
