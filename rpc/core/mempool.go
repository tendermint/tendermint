package core

import (
	"fmt"
	"time"

	"github.com/tendermint/go-events"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
	tmsp "github.com/tendermint/tmsp/types"
)

//-----------------------------------------------------------------------------
// NOTE: tx should be signed, but this is only checked at the app level (not by Tendermint!)

// Returns right away, with no response
func BroadcastTxAsync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	err := mempoolReactor.BroadcastTx(tx, nil)
	if err != nil {
		return nil, fmt.Errorf("Error broadcasting transaction: %v", err)
	}
	return &ctypes.ResultBroadcastTx{}, nil
}

// Returns with the response from CheckTx
func BroadcastTxSync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	resCh := make(chan *tmsp.Response, 1)
	err := mempoolReactor.BroadcastTx(tx, func(res *tmsp.Response) {
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
// or if we timeout waiting for tx to commit
func BroadcastTxCommit(tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {

	// subscribe to tx being committed in block
	appendTxResCh := make(chan *tmsp.Response, 1)
	eventSwitch.AddListenerForEvent("rpc", types.EventStringTx(tx), func(data events.EventData) {
		appendTxResCh <- data.(*tmsp.Response)
	})

	// broadcast the tx and register checktx callback
	checkTxResCh := make(chan *tmsp.Response, 1)
	err := mempoolReactor.BroadcastTx(tx, func(res *tmsp.Response) {
		checkTxResCh <- res
	})
	if err != nil {
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
	timer := time.NewTimer(60 * 5 * time.Second)
	select {
	case appendTxRes := <-appendTxResCh:
		// The tx was included in a block.
		appendTxR := appendTxRes.GetAppendTx()
		return &ctypes.ResultBroadcastTxCommit{
			CheckTx:  checkTxR,
			AppendTx: appendTxR,
		}, nil
	case <-timer.C:
		return &ctypes.ResultBroadcastTxCommit{
			CheckTx:  checkTxR,
			AppendTx: nil,
		}, fmt.Errorf("Timed out waiting for transaction to be included in a block")
	}

	panic("Should never happen!")
}

func UnconfirmedTxs() (*ctypes.ResultUnconfirmedTxs, error) {
	txs := mempoolReactor.Mempool.Reap(-1)
	return &ctypes.ResultUnconfirmedTxs{len(txs), txs}, nil
}

func NumUnconfirmedTxs() (*ctypes.ResultUnconfirmedTxs, error) {
	return &ctypes.ResultUnconfirmedTxs{N: mempoolReactor.Mempool.Size()}, nil
}
