package core

import (
	"fmt"
	"time"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
	tmsp "github.com/tendermint/tmsp/types"
)

//-----------------------------------------------------------------------------
// NOTE: tx should be signed, but this is only checked at the app level (not by Tendermint!)

// Returns right away, with no response
func BroadcastTxAsync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	err := mempool.CheckTx(tx, nil)
	if err != nil {
		return nil, fmt.Errorf("Error broadcasting transaction: %v", err)
	}
	return &ctypes.ResultBroadcastTx{}, nil
}

// Returns with the response from CheckTx
func BroadcastTxSync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	resCh := make(chan *tmsp.Response, 1)
	err := mempool.CheckTx(tx, func(res *tmsp.Response) {
		resCh <- res
	})
	if err != nil {
		return nil, fmt.Errorf("Error broadcasting transaction: %v", err)
	}
	res := <-resCh
	r := res.GetCheckTx()
	return &ctypes.ResultBroadcastTx{
		Code: r.Code,
		Data: r.Data,
		Log:  r.Log,
	}, nil
}

// CONTRACT: only returns error if mempool.BroadcastTx errs (ie. problem with the app)
// or if we timeout waiting for tx to commit.
// If CheckTx or AppendTx fail, no error will be returned, but the returned result
// will contain a non-OK TMSP code.
func BroadcastTxCommit(tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {

	// subscribe to tx being committed in block
	appendTxResCh := make(chan types.EventDataTx, 1)
	types.AddListenerForEvent(eventSwitch, "rpc", types.EventStringTx(tx), func(data types.TMEventData) {
		appendTxResCh <- data.(types.EventDataTx)
	})

	// broadcast the tx and register checktx callback
	checkTxResCh := make(chan *tmsp.Response, 1)
	err := mempool.CheckTx(tx, func(res *tmsp.Response) {
		checkTxResCh <- res
	})
	if err != nil {
		log.Error("err", "err", err)
		return nil, fmt.Errorf("Error broadcasting transaction: %v", err)
	}
	checkTxRes := <-checkTxResCh
	checkTxR := checkTxRes.GetCheckTx()
	if checkTxR.Code != tmsp.CodeType_OK {
		// CheckTx failed!
		return &ctypes.ResultBroadcastTxCommit{
			CheckTx:  checkTxR,
			AppendTx: nil,
		}, nil
	}

	// Wait for the tx to be included in a block,
	// timeout after something reasonable.
	// TODO: configureable?
	timer := time.NewTimer(60 * 2 * time.Second)
	select {
	case appendTxRes := <-appendTxResCh:
		// The tx was included in a block.
		appendTxR := &tmsp.ResponseAppendTx{
			Code: appendTxRes.Code,
			Data: appendTxRes.Data,
			Log:  appendTxRes.Log,
		}
		log.Error("appendtx passed ", "r", appendTxR)
		return &ctypes.ResultBroadcastTxCommit{
			CheckTx:  checkTxR,
			AppendTx: appendTxR,
		}, nil
	case <-timer.C:
		log.Error("failed to include tx")
		return &ctypes.ResultBroadcastTxCommit{
			CheckTx:  checkTxR,
			AppendTx: nil,
		}, fmt.Errorf("Timed out waiting for transaction to be included in a block")
	}

	panic("Should never happen!")
}

func UnconfirmedTxs() (*ctypes.ResultUnconfirmedTxs, error) {
	txs := mempool.Reap(-1)
	return &ctypes.ResultUnconfirmedTxs{len(txs), txs}, nil
}

func NumUnconfirmedTxs() (*ctypes.ResultUnconfirmedTxs, error) {
	return &ctypes.ResultUnconfirmedTxs{N: mempool.Size()}, nil
}
