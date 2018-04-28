package consensus

import (
	"github.com/tendermint/go-amino"
	"github.com/tendermint/go-crypto"
)

var cdc = amino.NewCodec()

func init() {
	RegisterConsensusMessages(cdc)
	RegisterWALMessages(cdc)
	crypto.RegisterAmino(cdc)
}
