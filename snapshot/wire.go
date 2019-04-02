package snapshot

import (
	"github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/types"
)

var cdc = amino.NewCodec()

func init() {
	RegisterSnapshotMessages(cdc)
	types.RegisterBlockAmino(cdc)
}
