package core

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/internal/state/indexer"
	tmmath "github.com/tendermint/tendermint/libs/math"
	"github.com/tendermint/tendermint/rpc/coretypes"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------------------
// NOTE: tx should be signed, but this is only checked at the app level (not by Tendermint!)

// BroadcastTxAsync returns right away, with no response. Does not wait for
// CheckTx nor DeliverTx results.
// More: https://docs.tendermint.com/master/rpc/#/Tx/broadcast_tx_async
func (env *Environment) BroadcastTxAsync(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	err := env.Mempool.CheckTx(ctx, tx, nil, mempool.TxInfo{})
	if err != nil {
		return nil, err
	}

	return &coretypes.ResultBroadcastTx{Hash: tx.Hash()}, nil
}

// BroadcastTxSync returns with the response from CheckTx. Does not wait for
// DeliverTx result.
// More: https://docs.tendermint.com/master/rpc/#/Tx/broadcast_tx_sync
func (env *Environment) BroadcastTxSync(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	resCh := make(chan *abci.ResponseCheckTx, 1)
	err := env.Mempool.CheckTx(
		ctx,
		tx,
		func(res *abci.ResponseCheckTx) { resCh <- res },
		mempool.TxInfo{},
	)
	if err != nil {
		return nil, err
	}

	r := <-resCh

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
func (env *Environment) BroadcastTxCommit(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTxCommit, error) {
	resCh := make(chan *abci.ResponseCheckTx, 1)
	err := env.Mempool.CheckTx(
		ctx,
		tx,
		func(res *abci.ResponseCheckTx) { resCh <- res },
		mempool.TxInfo{},
	)
	if err != nil {
		return nil, err
	}

	r := <-resCh
	if r.Code != abci.CodeTypeOK {
		return &coretypes.ResultBroadcastTxCommit{
			CheckTx: *r,
			Hash:    tx.Hash(),
		}, fmt.Errorf("transaction encountered error (%s)", r.MempoolError)
	}

	if !indexer.KVSinkEnabled(env.EventSinks) {
		return &coretypes.ResultBroadcastTxCommit{
				CheckTx: *r,
				Hash:    tx.Hash(),
			},
			errors.New("cannot confirm transaction because kvEventSink is not enabled")
	}

	startAt := time.Now()
	timer := time.NewTimer(0)
	defer timer.Stop()

	count := 0
	for {
		count++
		select {
		case <-ctx.Done():
			env.Logger.Error("error on broadcastTxCommit",
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

// UnconfirmedTxs gets unconfirmed transactions from the mempool in order of priority
// More: https://docs.tendermint.com/master/rpc/#/Info/unconfirmed_txs
func (env *Environment) UnconfirmedTxs(ctx context.Context, pagePtr, perPagePtr *int) (*coretypes.ResultUnconfirmedTxs, error) {
	totalCount := env.Mempool.Size()
	perPage := env.validatePerPage(perPagePtr)
	page, err := validatePage(pagePtr, perPage, totalCount)
	if err != nil {
		return nil, err
	}

	skipCount := validateSkipCount(page, perPage)

	txs := env.Mempool.ReapMaxTxs(skipCount + tmmath.MinInt(perPage, totalCount-skipCount))
	result := txs[skipCount:]

	return &coretypes.ResultUnconfirmedTxs{
		Count:      len(result),
		Total:      totalCount,
		TotalBytes: env.Mempool.SizeBytes(),
		Txs:        result}, nil
}

// NumUnconfirmedTxs gets number of unconfirmed transactions.
// More: https://docs.tendermint.com/master/rpc/#/Info/num_unconfirmed_txs
func (env *Environment) NumUnconfirmedTxs(ctx context.Context) (*coretypes.ResultUnconfirmedTxs, error) {
	return &coretypes.ResultUnconfirmedTxs{
		Count:      env.Mempool.Size(),
		Total:      env.Mempool.Size(),
		TotalBytes: env.Mempool.SizeBytes()}, nil
}

// CheckTx checks the transaction without executing it. The transaction won't
// be added to the mempool either.
// More: https://docs.tendermint.com/master/rpc/#/Tx/check_tx
func (env *Environment) CheckTx(ctx context.Context, tx types.Tx) (*coretypes.ResultCheckTx, error) {
	res, err := env.ProxyAppMempool.CheckTx(ctx, abci.RequestCheckTx{Tx: tx})
	if err != nil {
		return nil, err
	}
	return &coretypes.ResultCheckTx{ResponseCheckTx: *res}, nil
}

func (env *Environment) RemoveTx(ctx context.Context, txkey types.TxKey) error {
	return env.Mempool.RemoveTxByKey(txkey)
}
