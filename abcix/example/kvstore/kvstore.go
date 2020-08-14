package kvstore

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strconv"

	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abcix/example/code"
	"github.com/tendermint/tendermint/abcix/types"
	"github.com/tendermint/tendermint/libs/log"
	tdtypes "github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

var (
	kvPairPrefixKey = []byte("kvPairKey:")

	ProtocolVersion uint64 = 0x1
	maxBytes               = tdtypes.DefaultConsensusParams().Block.MaxBytes
	maxGas          int64  = 10
)

type State struct {
	db            dbm.DB
	Size          int64            `json:"size"`
	Height        int64            `json:"height"`
	ResourceUsage map[string]int64 `json:"resource_usage"`
	AppHash       []byte           `json:"app_hash"`
}

func loadState(db dbm.DB) State {
	return State{
		db:            db,
		ResourceUsage: make(map[string]int64),
	}
}

func prefixKey(key []byte) []byte {
	return append(kvPairPrefixKey, key...)
}

type Tx struct {
	kvs []struct {
		key []byte
		val []byte
	}
	from     []byte
	gasprice int64
}

//---------------------------------------------------

var _ types.Application = (*Application)(nil)

type Application struct {
	types.BaseApplication
	state        State
	logger       log.Logger
	RetainBlocks int64 // blocks to retain after commit (via ResponseCommit.RetainHeight)
}

func NewApplication() *Application {
	state := loadState(dbm.NewMemDB())
	return &Application{state: state, logger: log.NewTMLogger(log.NewSyncWriter(os.Stdout))}
}

func (app *Application) Info(req types.RequestInfo) (resInfo types.ResponseInfo) {
	return types.ResponseInfo{
		Data:             fmt.Sprintf("{\"size\":%v}", app.state.Size),
		Version:          version.ABCIVersion,
		AppVersion:       ProtocolVersion,
		LastBlockHeight:  app.state.Height,
		LastBlockAppHash: app.state.AppHash,
	}
}

func (app *Application) CheckTx(req types.RequestCheckTx) types.ResponseCheckTx {
	// Tx looks like "[key1]=[value1],[key2]=[value2],[from],[gasprice]"
	// e.g. "a=41,c=42,alice,100"
	tx, err := parseTx(req.Tx)
	if err != nil {
		app.logger.Error("failed to parse tx", "err", err)
		return types.ResponseCheckTx{Code: code.CodeTypeEncodingError}
	}
	return types.ResponseCheckTx{Code: code.CodeTypeOK, GasWanted: int64(len(tx.kvs)), Priority: uint64(tx.gasprice)}
}

// Iterate txs from mempool ordered by priority and select the ones to be included in the next block
func (app *Application) CreateBlock(
	req types.RequestCreateBlock,
	mempool *types.MempoolIter,
) types.ResponseCreateBlock {
	var txs, invalidTxs [][]byte
	var size int64

	remainBytes, remainGas := maxBytes, maxGas
	for mempool.HasNext() {
		tx, err := mempool.GetNextTransaction(remainBytes, remainGas)
		if err != nil {
			panic("failed to get next tx from mempool")
		}
		// Break if mempool runs out of tx
		if len(tx) == 0 {
			break
		}

		newState, gasUsed, err := executeTx(app.state, tx, true)
		if err != nil {
			invalidTxs = append(invalidTxs, tx)
			continue
		}
		txs = append(txs, tx)
		size = newState.Size
		remainBytes -= int64(len(tx))
		remainGas -= gasUsed
	}

	appHash := make([]byte, 8)
	binary.PutVarint(appHash, size)

	events := []types.Event{
		{
			Type: "create_block",
			Attributes: []types.EventAttribute{
				{Key: []byte("height"), Value: []byte{byte(req.Height)}},
				{Key: []byte("valid tx"), Value: []byte{byte(len(txs))}},
				{Key: []byte("invalid tx"), Value: []byte{byte(len(invalidTxs))}},
			},
		},
	}
	return types.ResponseCreateBlock{Txs: txs, InvalidTxs: invalidTxs, Hash: appHash, Events: events}
}

// Combination of ABCI.BeginBlock, []ABCI.DeliverTx, and ABCI.EndBlock
func (app *Application) DeliverBlock(req types.RequestDeliverBlock) types.ResponseDeliverBlock {
	ret := types.ResponseDeliverBlock{}
	// Tx looks like "[key1]=[value1],[key2]=[value2],[from],[gasprice]"
	// e.g. "a=41,c=42,alice,100"
	for _, tx := range req.Txs {
		newState, gasUsed, err := executeTx(app.state, tx, false)
		if err != nil {
			panic("consensus failure: invalid tx found in DeliverBlock: " + err.Error())
		}
		app.state = newState
		txResp := types.ResponseDeliverTx{GasUsed: gasUsed}
		ret.DeliverTxs = append(ret.DeliverTxs, &txResp)
	}
	return ret
}

func (app *Application) Commit() types.ResponseCommit {
	// Using a memdb - just return the big endian size of the db
	appHash := make([]byte, 8)
	binary.PutVarint(appHash, app.state.Size)
	app.state.AppHash = appHash
	app.state.Height++

	resp := types.ResponseCommit{Data: appHash}
	if app.RetainBlocks > 0 && app.state.Height >= app.RetainBlocks {
		resp.RetainHeight = app.state.Height - app.RetainBlocks + 1
	}
	return resp
}

// Returns an associated value or nil if missing.
func (app *Application) Query(reqQuery types.RequestQuery) (resQuery types.ResponseQuery) {
	// Can query key-value pair by path "kv" or resource usage by "usage"
	resQuery.Key = reqQuery.Data
	resQuery.Height = app.state.Height
	if reqQuery.Path == "kv" {
		value, err := app.state.db.Get(prefixKey(reqQuery.Data))
		if err != nil {
			panic(err)
		}
		if value == nil {
			resQuery.Log = "does not exist"
		} else {
			resQuery.Log = "exists"
		}
		resQuery.Value = value
	} else if reqQuery.Path == "usage" {
		usage, ok := app.state.ResourceUsage[string(reqQuery.Data)]
		if !ok {
			resQuery.Log = "does not exist"
		} else {
			resQuery.Log = "exists"
		}
		resQuery.Value = []byte(fmt.Sprintf("%d", usage))
	}
	return resQuery
}

func parseTx(tx []byte) (*Tx, error) {
	parts := bytes.Split(tx, []byte(","))
	size := len(parts)
	if size < 3 {
		return nil, errors.New("invalid tx")
	}
	gasprice, err := strconv.ParseInt(string(parts[size-1]), 10, 64)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse gasprice")
	}
	ret := Tx{
		from:     parts[size-2],
		gasprice: gasprice,
	}
	for _, kv := range parts[:size-2] {
		kvPair := bytes.Split(kv, []byte("="))
		if len(kvPair) != 2 {
			return nil, errors.New("invalid kv pair in tx")
		}
		ret.kvs = append(ret.kvs, struct {
			key []byte
			val []byte
		}{kvPair[0], kvPair[1]})
	}
	return &ret, nil
}

func executeTx(state State, txBytes []byte, preExecute bool) (newState State, gasUsed int64, err error) {
	tx, err := parseTx(txBytes)
	if err != nil {
		return
	}

	newState = state
	// Need to copy the map
	newState.ResourceUsage = make(map[string]int64)
	if err := copier.Copy(&newState.ResourceUsage, &state.ResourceUsage); err != nil {
		panic(err)
	}
	for _, kv := range tx.kvs {
		if !preExecute {
			if err := newState.db.Set(prefixKey(kv.key), kv.val); err != nil {
				panic(err)
			}
		}
		newState.Size++
		gasUsed++
		newState.ResourceUsage[string(tx.from)] += tx.gasprice
	}
	return
}
