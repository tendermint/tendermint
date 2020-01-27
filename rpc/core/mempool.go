package core

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"

	abci "github.com/tendermint/tendermint/abci/types"
	mempl "github.com/tendermint/tendermint/mempool"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------------------
// NOTE: tx should be signed, but this is only checked at the app level (not by Tendermint!)

// BroadcastTxAsync returns right away, with no response. Does not wait for
// CheckTx nor DeliverTx results.
// More: https://docs.tendermint.com/master/rpc/#/Tx/broadcast_tx_async
func BroadcastTxAsync(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	err := mempool.CheckTx(tx, nil, mempl.TxInfo{})

	if err != nil {
		return nil, err
	}
	return &ctypes.ResultBroadcastTx{Hash: tx.Hash()}, nil
}

// BroadcastTxSync returns with the response from CheckTx. Does not wait for
// DeliverTx result.
// More: https://docs.tendermint.com/master/rpc/#/Tx/broadcast_tx_sync
func BroadcastTxSync(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	resCh := make(chan *abci.Response, 1)
	err := mempool.CheckTx(tx, func(res *abci.Response) {
		resCh <- res
	}, mempl.TxInfo{})
	if err != nil {
		return nil, err
	}
	res := <-resCh
	r := res.GetCheckTx()
	return &ctypes.ResultBroadcastTx{
		Code: r.Code,
		Data: r.Data,
		Log:  r.Log,
		Hash: tx.Hash(),
	}, nil
}

// BroadcastTxCommit returns with the responses from CheckTx and DeliverTx.
// More: https://docs.tendermint.com/master/rpc/#/Tx/broadcast_tx_commit
func BroadcastTxCommit(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	subscriber := ctx.RemoteAddr()

	if eventBus.NumClients() >= config.MaxSubscriptionClients {
		return nil, fmt.Errorf("max_subscription_clients %d reached", config.MaxSubscriptionClients)
	} else if eventBus.NumClientSubscriptions(subscriber) >= config.MaxSubscriptionsPerClient {
		return nil, fmt.Errorf("max_subscriptions_per_client %d reached", config.MaxSubscriptionsPerClient)
	}

	// Subscribe to tx being committed in block.
	subCtx, cancel := context.WithTimeout(ctx.Context(), SubscribeTimeout)
	defer cancel()
	q := types.EventQueryTxFor(tx)
	deliverTxSub, err := eventBus.Subscribe(subCtx, subscriber, q)
	if err != nil {
		err = errors.Wrap(err, "failed to subscribe to tx")
		logger.Error("Error on broadcast_tx_commit", "err", err)
		return nil, err
	}
	defer eventBus.Unsubscribe(context.Background(), subscriber, q)

	// Broadcast tx and wait for CheckTx result
	checkTxResCh := make(chan *abci.Response, 1)
	err = mempool.CheckTx(tx, func(res *abci.Response) {
		checkTxResCh <- res
	}, mempl.TxInfo{})
	if err != nil {
		logger.Error("Error on broadcastTxCommit", "err", err)
		return nil, fmt.Errorf("error on broadcastTxCommit: %v", err)
	}
	checkTxResMsg := <-checkTxResCh
	checkTxRes := checkTxResMsg.GetCheckTx()
	if checkTxRes.Code != abci.CodeTypeOK {
		return &ctypes.ResultBroadcastTxCommit{
			CheckTx:   *checkTxRes,
			DeliverTx: abci.ResponseDeliverTx{},
			Hash:      tx.Hash(),
		}, nil
	}

	// Wait for the tx to be included in a block or timeout.
	select {
	case msg := <-deliverTxSub.Out(): // The tx was included in a block.
		deliverTxRes := msg.Data().(types.EventDataTx)
		return &ctypes.ResultBroadcastTxCommit{
			CheckTx:   *checkTxRes,
			DeliverTx: deliverTxRes.Result,
			Hash:      tx.Hash(),
			Height:    deliverTxRes.Height,
		}, nil
	case <-deliverTxSub.Cancelled():
		var reason string
		if deliverTxSub.Err() == nil {
			reason = "Tendermint exited"
		} else {
			reason = deliverTxSub.Err().Error()
		}
		err = fmt.Errorf("deliverTxSub was cancelled (reason: %s)", reason)
		logger.Error("Error on broadcastTxCommit", "err", err)
		return &ctypes.ResultBroadcastTxCommit{
			CheckTx:   *checkTxRes,
			DeliverTx: abci.ResponseDeliverTx{},
			Hash:      tx.Hash(),
		}, err
	case <-time.After(config.TimeoutBroadcastTxCommit):
		err = errors.New("timed out waiting for tx to be included in a block")
		logger.Error("Error on broadcastTxCommit", "err", err)
		return &ctypes.ResultBroadcastTxCommit{
			CheckTx:   *checkTxRes,
			DeliverTx: abci.ResponseDeliverTx{},
			Hash:      tx.Hash(),
		}, err
	}
}

// UnconfirmedTxs gets unconfirmed transactions (maximum ?limit entries)
// including their number.
// More: https://docs.tendermint.com/master/rpc/#/Info/unconfirmed_txs
func UnconfirmedTxs(ctx *rpctypes.Context, limit int) (*ctypes.ResultUnconfirmedTxs, error) {
	// reuse per_page validator
	limit = validatePerPage(limit)

	txs := mempool.ReapMaxTxs(limit)
	return &ctypes.ResultUnconfirmedTxs{
		Count:      len(txs),
		Total:      mempool.Size(),
		TotalBytes: mempool.TxsBytes(),
		Txs:        txs}, nil
}

// NumUnconfirmedTxs gets number of unconfirmed transactions.
// More: https://docs.tendermint.com/master/rpc/#/Info/num_unconfirmed_txs
func NumUnconfirmedTxs(ctx *rpctypes.Context) (*ctypes.ResultUnconfirmedTxs, error) {
	return &ctypes.ResultUnconfirmedTxs{
		Count:      mempool.Size(),
		Total:      mempool.Size(),
		TotalBytes: mempool.TxsBytes()}, nil
}
