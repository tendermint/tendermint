package p2p

import (
	amino "github.com/tendermint/go-amino"
	cryptoAmino "github.com/pakula/prism/crypto/encoding/amino"
)

var cdc = amino.NewCodec()

func init() {
	cryptoAmino.RegisterAmino(cdc)
}
