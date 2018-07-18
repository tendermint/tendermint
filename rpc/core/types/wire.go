package core_types

import (
	"github.com/tendermint/go-amino"
	cryptoAmino "github.com/tendermint/tendermint/crypto/encoding/amino"
	"github.com/tendermint/tendermint/types"
)

func RegisterAmino(cdc *amino.Codec) {
	types.RegisterEventDatas(cdc)
	types.RegisterEvidences(cdc)
	cryptoAmino.RegisterAmino(cdc)
}
