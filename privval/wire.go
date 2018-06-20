package privval

import (
	"github.com/tendermint/go-amino"
	"github.com/tendermint/go-crypto"
)

var cdc = amino.NewCodec()

func init() {
	crypto.RegisterAmino(cdc)
	RegisterSocketPVMsg(cdc)
}
