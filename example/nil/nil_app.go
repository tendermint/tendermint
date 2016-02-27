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

func (app *NilApplication) AppendTx(tx []byte) (code types.CodeType, result []byte, log string) {
	return types.CodeType_OK, nil, ""
}

func (app *NilApplication) CheckTx(tx []byte) (code types.CodeType, result []byte, log string) {
	return types.CodeType_OK, nil, ""
}

func (app *NilApplication) Commit() (hash []byte, log string) {
	return []byte("nil"), ""
}

func (app *NilApplication) Query(query []byte) (code types.CodeType, result []byte, log string) {
	return types.CodeType_OK, nil, ""
}
