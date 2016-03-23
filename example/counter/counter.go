package counter

import (
	"encoding/binary"
	"fmt"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tmsp/types"
)

type CounterApplication struct {
	hashCount int
	txCount   int
	serial    bool
}

func NewCounterApplication(serial bool) *CounterApplication {
	return &CounterApplication{serial: serial}
}

func (app *CounterApplication) Info() string {
	return Fmt("hashes:%v, txs:%v", app.hashCount, app.txCount)
}

func (app *CounterApplication) SetOption(key string, value string) (log string) {
	if key == "serial" && value == "on" {
		app.serial = true
	}
	return ""
}

func (app *CounterApplication) AppendTx(tx []byte) types.Result {
	if app.serial {
		tx8 := make([]byte, 8)
		copy(tx8[len(tx8)-len(tx):], tx)
		txValue := binary.BigEndian.Uint64(tx8)
		if txValue != uint64(app.txCount) {
			return types.Result{
				Code: types.CodeType_BadNonce,
				Data: nil,
				Log:  fmt.Sprintf("Invalid nonce. Expected %v, got %v", app.txCount, txValue),
			}
		}
	}
	app.txCount += 1
	return types.OK
}

func (app *CounterApplication) CheckTx(tx []byte) types.Result {
	if app.serial {
		tx8 := make([]byte, 8)
		copy(tx8[len(tx8)-len(tx):], tx)
		txValue := binary.BigEndian.Uint64(tx8)
		if txValue < uint64(app.txCount) {
			return types.Result{
				Code: types.CodeType_BadNonce,
				Data: nil,
				Log:  fmt.Sprintf("Invalid nonce. Expected >= %v, got %v", app.txCount, txValue),
			}
		}
	}
	return types.OK
}

func (app *CounterApplication) Commit() types.Result {
	app.hashCount += 1

	if app.txCount == 0 {
		return types.OK
	} else {
		hash := make([]byte, 8)
		binary.BigEndian.PutUint64(hash, uint64(app.txCount))
		return types.NewResultOK(hash, "")
	}
}

func (app *CounterApplication) Query(query []byte) types.Result {
	return types.NewResultOK(nil, fmt.Sprintf("Query is not supported"))
}
