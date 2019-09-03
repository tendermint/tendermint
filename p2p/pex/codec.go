package pex

import (
	amino "github.com/tendermint/go-amino"
)

var cdc *amino.Codec = amino.NewCodec()

func init() {
	RegisterPexMessage(cdc)
}
