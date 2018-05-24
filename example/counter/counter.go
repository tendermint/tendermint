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

func (app *CounterApplication) Info(req types.ParamsInfo) types.ResultInfo {
	return types.ResultInfo{Data: cmn.Fmt("{\"hashes\":%v,\"txs\":%v}", app.hashCount, app.txCount)}
}

func (app *CounterApplication) SetOption(req types.ParamsSetOption) types.ResultSetOption {
	key, value := req.Key, req.Value
	if key == "serial" && value == "on" {
		app.serial = true
	} else {
		/*
			TODO Panic and have the ABCI server pass an exception.
			The client can call SetOptionSync() and get an `error`.
			return types.ResultSetOption{
				Error: cmn.Fmt("Unknown key (%s) or value (%s)", key, value),
			}
		*/
		return types.ResultSetOption{}
	}

	return types.ResultSetOption{}
}

func (app *CounterApplication) DeliverTx(tx []byte) types.ResultDeliverTx {
	if app.serial {
		if len(tx) > 8 {
			return types.ResultDeliverTx{
				Code: code.CodeTypeEncodingError,
				Log:  fmt.Sprintf("Max tx size is 8 bytes, got %d", len(tx))}
		}
		tx8 := make([]byte, 8)
		copy(tx8[len(tx8)-len(tx):], tx)
		txValue := binary.BigEndian.Uint64(tx8)
		if txValue != uint64(app.txCount) {
			return types.ResultDeliverTx{
				Code: code.CodeTypeBadNonce,
				Log:  fmt.Sprintf("Invalid nonce. Expected %v, got %v", app.txCount, txValue)}
		}
	}
	app.txCount++
	return types.ResultDeliverTx{Code: code.CodeTypeOK}
}

func (app *CounterApplication) CheckTx(tx []byte) types.ResultCheckTx {
	if app.serial {
		if len(tx) > 8 {
			return types.ResultCheckTx{
				Code: code.CodeTypeEncodingError,
				Log:  fmt.Sprintf("Max tx size is 8 bytes, got %d", len(tx))}
		}
		tx8 := make([]byte, 8)
		copy(tx8[len(tx8)-len(tx):], tx)
		txValue := binary.BigEndian.Uint64(tx8)
		if txValue < uint64(app.txCount) {
			return types.ResultCheckTx{
				Code: code.CodeTypeBadNonce,
				Log:  fmt.Sprintf("Invalid nonce. Expected >= %v, got %v", app.txCount, txValue)}
		}
	}
	return types.ResultCheckTx{Code: code.CodeTypeOK}
}

func (app *CounterApplication) Commit() (resp types.ResultCommit) {
	app.hashCount++
	if app.txCount == 0 {
		return types.ResultCommit{}
	}
	hash := make([]byte, 8)
	binary.BigEndian.PutUint64(hash, uint64(app.txCount))
	return types.ResultCommit{Data: hash}
}

func (app *CounterApplication) Query(reqQuery types.ParamsQuery) types.ResultQuery {
	switch reqQuery.Path {
	case "hash":
		return types.ResultQuery{Value: []byte(cmn.Fmt("%v", app.hashCount))}
	case "tx":
		return types.ResultQuery{Value: []byte(cmn.Fmt("%v", app.txCount))}
	default:
		return types.ResultQuery{Log: cmn.Fmt("Invalid query path. Expected hash or tx, got %v", reqQuery.Path)}
	}
}
