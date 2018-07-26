package types

import (
	"github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto/encoding/amino"
)

var cdc = amino.NewCodec()

func init() {
	cryptoAmino.RegisterAmino(cdc)
	RegisterEvidences(cdc)
	RegisterMockEvidences(cdc)
}

//-------------------------------------------

func RegisterMockEvidences(cdc *amino.Codec) {
	cdc.RegisterConcrete(MockGoodEvidence{},
		"tendermint/MockGoodEvidence", nil)
	cdc.RegisterConcrete(MockBadEvidence{},
		"tendermint/MockBadEvidence", nil)
}
