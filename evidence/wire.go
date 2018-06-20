package evidence

import (
	"github.com/tendermint/go-amino"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/types"
)

var cdc = amino.NewCodec()

func init() {
	RegisterEvidenceMessages(cdc)
	crypto.RegisterAmino(cdc)
	types.RegisterEvidences(cdc)
	RegisterMockEvidences(cdc) // For testing
}

//-------------------------------------------

func RegisterMockEvidences(cdc *amino.Codec) {
	cdc.RegisterConcrete(types.MockGoodEvidence{},
		"tendermint/MockGoodEvidence", nil)
	cdc.RegisterConcrete(types.MockBadEvidence{},
		"tendermint/MockBadEvidence", nil)
}
