package mempool

import (
	"github.com/tendermint/go-amino"
)

var cdc = amino.NewCodec()

func init() {
	RegisterMempoolMessages(cdc)
}
