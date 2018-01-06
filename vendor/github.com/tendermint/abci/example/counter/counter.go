package counter

import (
	"encoding/binary"
	"fmt"

	"github.com/tendermint/abci/example/code"
	"github.com/tendermint/abci/types"
	cmn "github.com/tendermint/tmlibs/common"
)

type CounterApplication struct {
	types.BaseApplication

	hashCount int
	txCount   int
	serial    bool
}

func NewCounterApplication(serial bool) *CounterApplication {
	return &CounterApplication{serial: serial}
}

func (app *CounterApplication) Info(req types.RequestInfo) types.ResponseInfo {
	return types.ResponseInfo{Data: cmn.Fmt("{\"hashes\":%v,\"txs\":%v}", app.hashCount, app.txCount)}
}

func (app *CounterApplication) SetOption(req types.RequestSetOption) types.ResponseSetOption {
	key, value := req.Key, req.Value
	if key == "serial" && value == "on" {
		app.serial = true
	} else {
		return types.ResponseSetOption{
			Code: code.CodeTypeBadOption,
			Log:  cmn.Fmt("Unknown key (%s) or value (%s)", key, value),
		}
	}

	return types.ResponseSetOption{
		Code: code.CodeTypeOK,
	}
}

func (app *CounterApplication) DeliverTx(tx []byte) types.ResponseDeliverTx {
	if app.serial {
		if len(tx) > 8 {
			return types.ResponseDeliverTx{
				Code: code.CodeTypeEncodingError,
				Log:  fmt.Sprintf("Max tx size is 8 bytes, got %d", len(tx))}
		}
		tx8 := make([]byte, 8)
		copy(tx8[len(tx8)-len(tx):], tx)
		txValue := binary.BigEndian.Uint64(tx8)
		if txValue != uint64(app.txCount) {
			return types.ResponseDeliverTx{
				Code: code.CodeTypeBadNonce,
				Log:  fmt.Sprintf("Invalid nonce. Expected %v, got %v", app.txCount, txValue)}
		}
	}
	app.txCount++
	return types.ResponseDeliverTx{Code: code.CodeTypeOK}
}

func (app *CounterApplication) CheckTx(tx []byte) types.ResponseCheckTx {
	if app.serial {
		if len(tx) > 8 {
			return types.ResponseCheckTx{
				Code: code.CodeTypeEncodingError,
				Log:  fmt.Sprintf("Max tx size is 8 bytes, got %d", len(tx))}
		}
		tx8 := make([]byte, 8)
		copy(tx8[len(tx8)-len(tx):], tx)
		txValue := binary.BigEndian.Uint64(tx8)
		if txValue < uint64(app.txCount) {
			return types.ResponseCheckTx{
				Code: code.CodeTypeBadNonce,
				Log:  fmt.Sprintf("Invalid nonce. Expected >= %v, got %v", app.txCount, txValue)}
		}
	}
	return types.ResponseCheckTx{Code: code.CodeTypeOK}
}

func (app *CounterApplication) Commit() (resp types.ResponseCommit) {
	app.hashCount++
	if app.txCount == 0 {
		return types.ResponseCommit{Code: code.CodeTypeOK}
	}
	hash := make([]byte, 8)
	binary.BigEndian.PutUint64(hash, uint64(app.txCount))
	return types.ResponseCommit{Code: code.CodeTypeOK, Data: hash}
}

func (app *CounterApplication) Query(reqQuery types.RequestQuery) types.ResponseQuery {
	switch reqQuery.Path {
	case "hash":
		return types.ResponseQuery{Value: []byte(cmn.Fmt("%v", app.hashCount))}
	case "tx":
		return types.ResponseQuery{Value: []byte(cmn.Fmt("%v", app.txCount))}
	default:
		return types.ResponseQuery{Log: cmn.Fmt("Invalid query path. Expected hash or tx, got %v", reqQuery.Path)}
	}
}
