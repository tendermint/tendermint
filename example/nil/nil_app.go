package nilapp

import (
	"github.com/tendermint/tmsp/types"
)

type NilApplication struct {
}

func NewNilApplication() *NilApplication {
	return &NilApplication{}
}

func (app *NilApplication) Info() string {
	return "nil"
}

func (app *NilApplication) SetOption(key string, value string) (log string) {
	return ""
}

func (app *NilApplication) AppendTx(tx []byte) types.Result {
	return types.NewResultOK(nil, "")
}

func (app *NilApplication) CheckTx(tx []byte) types.Result {
	return types.NewResultOK(nil, "")
}

func (app *NilApplication) Commit() types.Result {
	return types.NewResultOK([]byte("nil"), "")
}

func (app *NilApplication) Query(query []byte) types.Result {
	return types.NewResultOK(nil, "")
}
