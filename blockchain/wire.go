package blockchain

import (
	"github.com/tendermint/go-amino"
)

var cdc = amino.NewCodec()

func init() {
	RegisterBlockchainMessages(cdc)
}
