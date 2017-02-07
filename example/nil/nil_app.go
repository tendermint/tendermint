package nilapp

import (
	"github.com/tendermint/abci/types"
)

type NilApplication struct {
}

func NewNilApplication() *NilApplication {
	return &NilApplication{}
}

func (app *NilApplication) Info() (resInfo types.ResponseInfo) {
	return
}

func (app *NilApplication) SetOption(key string, value string) (log string) {
	return ""
}

func (app *NilApplication) DeliverTx(tx []byte) types.Result {
	return types.NewResultOK(nil, "")
}

func (app *NilApplication) CheckTx(tx []byte) types.Result {
	return types.NewResultOK(nil, "")
}

func (app *NilApplication) Commit() types.Result {
	return types.NewResultOK([]byte("nil"), "")
}

func (app *NilApplication) Query(reqQuery types.RequestQuery) (resQuery types.ResponseQuery) {
	return resQuery
}

func (app *NilApplication) InitChain(validators []*types.Validator) {
}

func (app *NilApplication) BeginBlock(hash []byte, header *types.Header) {
}

func (app *NilApplication) EndBlock(height uint64) types.ResponseEndBlock {
	return types.ResponseEndBlock{}
}
