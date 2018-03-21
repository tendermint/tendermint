package conn

import (
	"github.com/tendermint/go-amino"
	"github.com/tendermint/go-crypto"
)

var cdc *amino.Codec

func init() {
	cdc = amino.NewCodec()
	crypto.RegisterAmino(cdc)
	RegisterPacket(cdc)
}
