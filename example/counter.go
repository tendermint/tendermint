package example

import (
	"encoding/binary"
	"sync"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tmsp/types"
)

type CounterApplication struct {
	mtx         sync.Mutex
	hashCount   int
	txCount     int
	commitCount int
}

func NewCounterApplication() *CounterApplication {
	return &CounterApplication{}
}

func (app *CounterApplication) Open() types.AppContext {
	return &CounterAppContext{
		app:         app,
		hashCount:   app.hashCount,
		txCount:     app.txCount,
		commitCount: app.commitCount,
	}
}

//--------------------------------------------------------------------------------

type CounterAppContext struct {
	app         *CounterApplication
	hashCount   int
	txCount     int
	commitCount int
}

func (appC *CounterAppContext) Echo(message string) string {
	return message
}

func (appC *CounterAppContext) Info() []string {
	return []string{Fmt("hash, tx, commit counts:%d, %d, %d", appC.hashCount, appC.txCount, appC.commitCount)}
}

func (appC *CounterAppContext) SetOption(key string, value string) types.RetCode {
	return 0
}

func (appC *CounterAppContext) AppendTx(tx []byte) ([]types.Event, types.RetCode) {
	appC.txCount += 1
	return nil, 0
}

func (appC *CounterAppContext) GetHash() ([]byte, types.RetCode) {
	hash := make([]byte, 32)
	binary.PutVarint(hash, int64(appC.hashCount))
	appC.hashCount += 1
	return hash, 0
}

func (appC *CounterAppContext) Commit() types.RetCode {
	appC.commitCount += 1

	appC.app.mtx.Lock()
	appC.app.hashCount = appC.hashCount
	appC.app.txCount = appC.txCount
	appC.app.commitCount = appC.commitCount
	appC.app.mtx.Unlock()
	return 0
}

func (appC *CounterAppContext) Rollback() types.RetCode {
	appC.app.mtx.Lock()
	appC.hashCount = appC.app.hashCount
	appC.txCount = appC.app.txCount
	appC.commitCount = appC.app.commitCount
	appC.app.mtx.Unlock()
	return 0
}

func (appC *CounterAppContext) AddListener(key string) types.RetCode {
	return 0
}

func (appC *CounterAppContext) RemListener(key string) types.RetCode {
	return 0
}

func (appC *CounterAppContext) Close() error {
	return nil
}
