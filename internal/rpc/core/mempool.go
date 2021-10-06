package core

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/internal/state/indexer"
	"github.com/tendermint/tendermint/rpc/coretypes"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------------------
// NOTE: tx should be signed, but this is only checked at the app level (not by Tendermint!)

// BroadcastTxAsync returns right away, with no response. Does not wait for
// CheckTx nor DeliverTx results.
// More: https://docs.tendermint.com/master/rpc/#/Tx/broadcast_tx_async
func (env *Environment) BroadcastTxAsync(ctx *rpctypes.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	err := env.Mempool.CheckTx(ctx.Context(), tx, nil, mempool.TxInfo{})
	if err != nil {
		return nil, err
	}

	return &coretypes.ResultBroadcastTx{Hash: tx.Hash()}, nil
}

// BroadcastTxSync returns with the response from CheckTx. Does not wait for
// DeliverTx result.
// More: https://docs.tendermint.com/master/rpc/#/Tx/broadcast_tx_sync
func (env *Environment) BroadcastTxSync(ctx *rpctypes.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	resCh := make(chan *abci.Response, 1)
	err := env.Mempool.CheckTx(
		ctx.Context(),
		tx,
		func(res *abci.Response) { resCh <- res },
		mempool.TxInfo{},
	)
	if err != nil {
		return nil, err
	}

	res := <-resCh
	r := res.GetCheckTx()

	return &coretypes.ResultBroadcastTx{
		Code:         r.Code,
		Data:         r.Data,
		Log:          r.Log,
		Codespace:    r.Codespace,
		MempoolError: r.MempoolError,
		Hash:         tx.Hash(),
	}, nil
}

// BroadcastTxCommit returns with the responses from CheckTx and DeliverTx.
// More: https://docs.tendermint.com/master/rpc/#/Tx/broadcast_tx_commit
func (env *Environment) BroadcastTxCommit(ctx *rpctypes.Context, tx types.Tx) (*coretypes.ResultBroadcastTxCommit, error) { //nolint:lll
	resCh := make(chan *abci.Response, 1)
	err := env.Mempool.CheckTx(
		ctx.Context(),
		tx,
		func(res *abci.Response) { resCh <- res },
		mempool.TxInfo{},
	)
	if err != nil {
		return nil, err
	}

	r := (<-resCh).GetCheckTx()

	if !indexer.KVSinkEnabled(env.EventSinks) {
		return &coretypes.ResultBroadcastTxCommit{
				CheckTx: *r,
				Hash:    tx.Hash(),
			},
			errors.New("cannot wait for commit because kvEventSync is not enabled")
	}

	startAt := time.Now()
	timer := time.NewTimer(0)
	defer timer.Stop()

	count := 0
	for {
		count++
		select {
		case <-ctx.Context().Done():
			env.Logger.Error("Error on broadcastTxCommit",
				"duration", time.Since(startAt),
				"err", err)
			return &coretypes.ResultBroadcastTxCommit{
					CheckTx: *r,
					Hash:    tx.Hash(),
				}, fmt.Errorf("timeout waiting for commit of tx %s (%s)",
					tx.Hash(), time.Since(startAt))
		case <-timer.C:
			txres, err := env.Tx(ctx, tx.Hash(), false)
			if err != nil {
				jitter := 100*time.Millisecond + time.Duration(rand.Int63n(int64(time.Second))) // nolint: gosec
				backoff := 100 * time.Duration(count) * time.Millisecond
				timer.Reset(jitter + backoff)
				continue
			}

			return &coretypes.ResultBroadcastTxCommit{
				CheckTx:   *r,
				DeliverTx: txres.TxResult,
				Hash:      tx.Hash(),
				Height:    txres.Height,
			}, nil
		}
	}
}

// UnconfirmedTxs gets unconfirmed transactions (maximum ?limit entries)
// including their number.
// More: https://docs.tendermint.com/master/rpc/#/Info/unconfirmed_txs
func (env *Environment) UnconfirmedTxs(ctx *rpctypes.Context, limitPtr *int) (*coretypes.ResultUnconfirmedTxs, error) {
	// reuse per_page validator
	limit := env.validatePerPage(limitPtr)

	txs := env.Mempool.ReapMaxTxs(limit)
	return &coretypes.ResultUnconfirmedTxs{
		Count:      len(txs),
		Total:      env.Mempool.Size(),
		TotalBytes: env.Mempool.SizeBytes(),
		Txs:        txs}, nil
}

// NumUnconfirmedTxs gets number of unconfirmed transactions.
// More: https://docs.tendermint.com/master/rpc/#/Info/num_unconfirmed_txs
func (env *Environment) NumUnconfirmedTxs(ctx *rpctypes.Context) (*coretypes.ResultUnconfirmedTxs, error) {
	return &coretypes.ResultUnconfirmedTxs{
		Count:      env.Mempool.Size(),
		Total:      env.Mempool.Size(),
		TotalBytes: env.Mempool.SizeBytes()}, nil
}

// CheckTx checks the transaction without executing it. The transaction won't
// be added to the mempool either.
// More: https://docs.tendermint.com/master/rpc/#/Tx/check_tx
func (env *Environment) CheckTx(ctx *rpctypes.Context, tx types.Tx) (*coretypes.ResultCheckTx, error) {
	res, err := env.ProxyAppMempool.CheckTxSync(ctx.Context(), abci.RequestCheckTx{Tx: tx})
	if err != nil {
		return nil, err
	}
	return &coretypes.ResultCheckTx{ResponseCheckTx: *res}, nil
}

func (env *Environment) RemoveTx(ctx *rpctypes.Context, txkey types.TxKey) error {
	return env.Mempool.RemoveTxByKey(txkey)
}
