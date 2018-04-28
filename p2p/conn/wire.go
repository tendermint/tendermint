package conn

import (
	"github.com/tendermint/go-amino"
	"github.com/tendermint/go-crypto"
)

var cdc *amino.Codec = amino.NewCodec()

func init() {
	crypto.RegisterAmino(cdc)
	RegisterPacket(cdc)
}
