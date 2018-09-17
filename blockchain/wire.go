package blockchain

import (
	"github.com/tendermint/go-amino"

	"github.com/tendermint/tendermint/evidence"
	"github.com/tendermint/tendermint/types"
)

var cdc = amino.NewCodec()

func init() {
	RegisterBlockchainMessages(cdc)
	types.RegisterBlockAmino(cdc)
	evidence.RegisterEvidenceMessages(cdc)
}
