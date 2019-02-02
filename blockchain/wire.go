package blockchain

import (
	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/types"
)

var cdc = amino.NewCodec()

func init() {
	RegisterBlockchainMessages(cdc)
	RegisterBlockchainStateMessages(cdc)
	types.RegisterBlockAmino(cdc)
}
