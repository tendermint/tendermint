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
	return types.RetCodeOK
}

func (app *CounterApplication) AppendTx(tx []byte) ([]types.Event, types.RetCode) {
	if app.serial {
		tx8 := make([]byte, 8)
		copy(tx8, tx)
		txValue := binary.LittleEndian.Uint64(tx8)
		if txValue != uint64(app.txCount) {
			return nil, types.RetCodeBadNonce
		}
	}
	app.txCount += 1
	return nil, types.RetCodeOK
}

func (app *CounterApplication) CheckTx(tx []byte) types.RetCode {
	if app.serial {
		tx8 := make([]byte, 8)
		copy(tx8, tx)
		txValue := binary.LittleEndian.Uint64(tx8)
		if txValue < uint64(app.txCount) {
			return types.RetCodeBadNonce
		}
	}
	return types.RetCodeOK
}

func (app *CounterApplication) GetHash() ([]byte, types.RetCode) {
	app.hashCount += 1

	if app.txCount == 0 {
		return nil, types.RetCodeOK
	} else {
		hash := make([]byte, 32)
		binary.LittleEndian.PutUint64(hash, uint64(app.txCount))
		return hash, types.RetCodeOK
	}
}

func (app *CounterApplication) AddListener(key string) types.RetCode {
	return types.RetCodeOK
}

func (app *CounterApplication) RemListener(key string) types.RetCode {
	return types.RetCodeOK
}

func (app *CounterApplication) Query(query []byte) (types.RetCode, []byte) {
	return types.RetCodeOK, nil
}
