package mempool

import (
	amino "github.com/tendermint/go-amino"
)

var cdc = amino.NewCodec()

func init() {
	RegisterMempoolMessages(cdc)
}
