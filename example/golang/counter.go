package example

import (
	"encoding/binary"

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

func (app *CounterApplication) Echo(message string) string {
	return message
}

func (app *CounterApplication) Info() []string {
	return []string{Fmt("hashes:%v, txs:%v", app.hashCount, app.txCount)}
}

func (app *CounterApplication) SetOption(key string, value string) types.RetCode {
	if key == "serial" && value == "on" {
		app.serial = true
	}
	return 0
}

func (app *CounterApplication) AppendTx(tx []byte) ([]types.Event, types.RetCode) {
	if app.serial {
		tx8 := make([]byte, 8)
		copy(tx8, tx)
		txValue := binary.LittleEndian.Uint64(tx8)
		if txValue != uint64(app.txCount) {
			return nil, types.RetCodeInternalError
		}
	}
	app.txCount += 1
	return nil, 0
}

func (app *CounterApplication) CheckTx(tx []byte) types.RetCode {
	if app.serial {
		tx8 := make([]byte, 8)
		copy(tx8, tx)
		txValue := binary.LittleEndian.Uint64(tx8)
		if txValue < uint64(app.txCount) {
			return types.RetCodeInternalError
		}
	}
	return 0
}

func (app *CounterApplication) GetHash() ([]byte, types.RetCode) {
	app.hashCount += 1

	if app.txCount == 0 {
		return nil, 0
	} else {
		hash := make([]byte, 32)
		binary.LittleEndian.PutUint64(hash, uint64(app.txCount))
		return hash, 0
	}
}

func (app *CounterApplication) AddListener(key string) types.RetCode {
	return 0
}

func (app *CounterApplication) RemListener(key string) types.RetCode {
	return 0
}
