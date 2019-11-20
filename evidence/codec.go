package evidence

import (
	amino "github.com/tendermint/go-amino"
	cryptoamino "github.com/tendermint/tendermint/crypto/encoding/amino"
	"github.com/tendermint/tendermint/types"
)

var cdc = amino.NewCodec()

func init() {
	RegisterEvidenceMessages(cdc)
	cryptoamino.RegisterAmino(cdc)
	types.RegisterEvidences(cdc)
}

// For testing purposes only
func RegisterMockEvidences() {
	types.RegisterMockEvidences(cdc)
}
