package coretypes

import (
	amino "github.com/tendermint/go-amino"

	"github.com/tendermint/tendermint/types"
)

func RegisterAmino(cdc *amino.Codec) {
	types.RegisterEventDatas(cdc)
	types.RegisterBlockAmino(cdc)
}
