package counter

import (
	"context"
	"encoding/binary"
	"fmt"

	tmtypes "github.com/tendermint/tendermint/types"

	"github.com/tendermint/tendermint/abci/example/code"
	"github.com/tendermint/tendermint/abci/types"
)

type Application struct {
	types.BaseApplication

	hashCount                  int
	txCount                    int
	serial                     bool
	HasCoreChainLocks          bool
	CurrentCoreChainLockHeight uint32
	CoreChainLockStep          int32
}

func NewApplication(serial bool) *Application {
	return &Application{serial: serial, CoreChainLockStep: 1}
}

func (app *Application) Info(_ context.Context, _ *types.RequestInfo) (*types.ResponseInfo, error) {
	return &types.ResponseInfo{Data: fmt.Sprintf("{\"hashes\":%v,\"txs\":%v}", app.hashCount, app.txCount)}, nil
}

func (app *Application) CheckTx(_ context.Context, req *types.RequestCheckTx) (*types.ResponseCheckTx, error) {
	if app.serial {
		if len(req.Tx) > 8 {
			return &types.ResponseCheckTx{
				Code: code.CodeTypeEncodingError,
			}, nil
		}
		tx8 := make([]byte, 8)
		copy(tx8[len(tx8)-len(req.Tx):], req.Tx)
		txValue := binary.BigEndian.Uint64(tx8)
		if txValue < uint64(app.txCount) {
			return &types.ResponseCheckTx{
				Code: code.CodeTypeBadNonce,
			}, nil
		}
	}
	return &types.ResponseCheckTx{Code: code.CodeTypeOK}, nil
}

func (app *Application) Commit(_ context.Context) (*types.ResponseCommit, error) {
	app.hashCount++
	if app.txCount == 0 {
		return &types.ResponseCommit{}, nil
	}
	hash := make([]byte, 32)
	binary.BigEndian.PutUint64(hash[24:], uint64(app.txCount))
	return &types.ResponseCommit{}, nil
}

func (app *Application) Query(_ context.Context, reqQuery *types.RequestQuery) (*types.ResponseQuery, error) {
	switch reqQuery.Path {
	case "verify-chainlock":
		return &types.ResponseQuery{Code: 0}, nil
	case "hash":
		return &types.ResponseQuery{Value: []byte(fmt.Sprintf("%v", app.hashCount))}, nil
	case "tx":
		return &types.ResponseQuery{Value: []byte(fmt.Sprintf("%v", app.txCount))}, nil
	default:
		return &types.ResponseQuery{Log: fmt.Sprintf("Invalid query path. Expected hash or tx, got %v", reqQuery.Path)}, nil
	}
}

func (app *Application) FinalizeBlock(_ context.Context, req *types.RequestFinalizeBlock) (*types.ResponseFinalizeBlock, error) {
	var resp types.ResponseFinalizeBlock
	for _, tx := range req.Txs {
		if app.serial {
			if len(tx) > 8 {
				resp.TxResults = append(resp.TxResults, &types.ExecTxResult{
					Code: code.CodeTypeEncodingError,
					Log:  fmt.Sprintf("Max tx size is 8 bytes, got %d", len(tx)),
				})
			}
			tx8 := make([]byte, 8)
			copy(tx8[len(tx8)-len(tx):], tx)
			txValue := binary.BigEndian.Uint64(tx8)
			if txValue != uint64(app.txCount) {
				resp.TxResults = append(resp.TxResults, &types.ExecTxResult{
					Code: code.CodeTypeBadNonce,
					Log:  fmt.Sprintf("Invalid nonce. Expected %v, got %v", app.txCount, txValue),
				})
			}
		}
		app.txCount++
	}
	if app.HasCoreChainLocks {
		app.CurrentCoreChainLockHeight = app.CurrentCoreChainLockHeight + uint32(app.CoreChainLockStep)
		coreChainLock := tmtypes.NewMockChainLock(app.CurrentCoreChainLockHeight)
		resp.NextCoreChainLockUpdate = coreChainLock.ToProto()
	}
	return &resp, nil
}
