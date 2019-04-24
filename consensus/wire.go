package consensus

import (
	amino "github.com/tendermint/go-amino"
	"github.com/pakula/prism/types"
)

var cdc = amino.NewCodec()

func init() {
	RegisterConsensusMessages(cdc)
	RegisterWALMessages(cdc)
	types.RegisterBlockAmino(cdc)
}
