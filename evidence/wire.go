package evidence

import (
	"github.com/tendermint/go-amino"
	cryptoAmino "github.com/tendermint/tendermint/crypto/encoding/amino"
	"github.com/tendermint/tendermint/types"
)

var cdc = amino.NewCodec()

func init() {
	RegisterEvidenceMessages(cdc)
	cryptoAmino.RegisterAmino(cdc)
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
