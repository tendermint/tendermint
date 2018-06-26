package node

import (
	amino "github.com/tendermint/go-amino"
	crypto "github.com/tendermint/tendermint/crypto"
)

var cdc = amino.NewCodec()

func init() {
	crypto.RegisterAmino(cdc)
}
