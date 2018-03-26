package pex

import (
	"github.com/tendermint/go-amino"
)

var cdc *amino.Codec = amino.NewCodec()

func init() {
	RegisterPexMessage(cdc)
}
