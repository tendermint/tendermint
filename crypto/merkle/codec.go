package merkle

import (
	amino "github.com/tendermint/go-amino"
)

var cdc *amino.Codec

func init() {
	cdc = amino.NewCodec()
	cdc.Seal()
}
