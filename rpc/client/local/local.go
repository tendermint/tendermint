package local

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/pubsub"
	"github.com/tendermint/tendermint/internal/pubsub/query"
	rpccore "github.com/tendermint/tendermint/internal/rpc/core"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/coretypes"
	"github.com/tendermint/tendermint/types"
)

/*
Local is a Client implementation that directly executes the rpc
functions on a given node, without going through HTTP or GRPC.

This implementation is useful for:

* Running tests against a node in-process without the overhead
of going through an http server
* Communication between an ABCI app and Tendermint core when they
are compiled in process.

For real clients, you probably want to use client.HTTP.  For more
powerful control during testing, you probably want the "client/mock" package.

You can subscribe for any event published by Tendermint using Subscribe method.
Note delivery is best-effort. If you don't read events fast enough, Tendermint
might cancel the subscription. The client will attempt to resubscribe (you
don't need to do anything). It will keep trying indefinitely with exponential
backoff (10ms -> 20ms -> 40ms) until successful.
*/
type Local struct {
	*eventbus.EventBus
	Logger log.Logger
	env    *rpccore.Environment
}

// NodeService describes the portion of the node interface that the
// local RPC client constructor needs to build a local client.
type NodeService interface {
	RPCEnvironment() *rpccore.Environment
	EventBus() *eventbus.EventBus
}

// New configures a client that calls the Node directly.
func New(logger log.Logger, node NodeService) (*Local, error) {
	env := node.RPCEnvironment()
	if env == nil {
		return nil, errors.New("rpc is nil")
	}
	return &Local{
		EventBus: node.EventBus(),
		Logger:   logger,
		env:      env,
	}, nil
}

var _ rpcclient.Client = (*Local)(nil)

func (c *Local) Status(ctx context.Context) (*coretypes.ResultStatus, error) {
	return c.env.Status(ctx)
}

func (c *Local) ABCIInfo(ctx context.Context) (*coretypes.ResultABCIInfo, error) {
	return c.env.ABCIInfo(ctx)
}

func (c *Local) ABCIQuery(ctx context.Context, path string, data bytes.HexBytes) (*coretypes.ResultABCIQuery, error) {
	return c.ABCIQueryWithOptions(ctx, path, data, rpcclient.DefaultABCIQueryOptions)
}

func (c *Local) ABCIQueryWithOptions(ctx context.Context, path string, data bytes.HexBytes, opts rpcclient.ABCIQueryOptions) (*coretypes.ResultABCIQuery, error) {
	return c.env.ABCIQuery(ctx, &coretypes.RequestABCIQuery{
		Path: path, Data: data, Height: coretypes.Int64(opts.Height), Prove: opts.Prove,
	})
}

func (c *Local) BroadcastTxCommit(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTxCommit, error) {
	return c.env.BroadcastTxCommit(ctx, &coretypes.RequestBroadcastTx{Tx: tx})
}

func (c *Local) BroadcastTx(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	return c.env.BroadcastTx(ctx, &coretypes.RequestBroadcastTx{Tx: tx})
}

func (c *Local) BroadcastTxAsync(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	return c.env.BroadcastTxAsync(ctx, &coretypes.RequestBroadcastTx{Tx: tx})
}

func (c *Local) BroadcastTxSync(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	return c.env.BroadcastTxSync(ctx, &coretypes.RequestBroadcastTx{Tx: tx})
}

func (c *Local) UnconfirmedTxs(ctx context.Context, page, perPage *int) (*coretypes.ResultUnconfirmedTxs, error) {
	return c.env.UnconfirmedTxs(ctx, &coretypes.RequestUnconfirmedTxs{
		Page:    coretypes.Int64Ptr(page),
		PerPage: coretypes.Int64Ptr(perPage),
	})
}

func (c *Local) NumUnconfirmedTxs(ctx context.Context) (*coretypes.ResultUnconfirmedTxs, error) {
	return c.env.NumUnconfirmedTxs(ctx)
}

func (c *Local) CheckTx(ctx context.Context, tx types.Tx) (*coretypes.ResultCheckTx, error) {
	return c.env.CheckTx(ctx, &coretypes.RequestCheckTx{Tx: tx})
}

func (c *Local) RemoveTx(ctx context.Context, txKey types.TxKey) error {
	return c.env.Mempool.RemoveTxByKey(txKey)
}

func (c *Local) NetInfo(ctx context.Context) (*coretypes.ResultNetInfo, error) {
	return c.env.NetInfo(ctx)
}

func (c *Local) DumpConsensusState(ctx context.Context) (*coretypes.ResultDumpConsensusState, error) {
	return c.env.DumpConsensusState(ctx)
}

func (c *Local) ConsensusState(ctx context.Context) (*coretypes.ResultConsensusState, error) {
	return c.env.GetConsensusState(ctx)
}

func (c *Local) ConsensusParams(ctx context.Context, height *int64) (*coretypes.ResultConsensusParams, error) {
	return c.env.ConsensusParams(ctx, &coretypes.RequestConsensusParams{Height: (*coretypes.Int64)(height)})
}

func (c *Local) Events(ctx context.Context, req *coretypes.RequestEvents) (*coretypes.ResultEvents, error) {
	return c.env.Events(ctx, req)
}

func (c *Local) Health(ctx context.Context) (*coretypes.ResultHealth, error) {
	return c.env.Health(ctx)
}

func (c *Local) BlockchainInfo(ctx context.Context, minHeight, maxHeight int64) (*coretypes.ResultBlockchainInfo, error) {
	return c.env.BlockchainInfo(ctx, &coretypes.RequestBlockchainInfo{
		MinHeight: coretypes.Int64(minHeight),
		MaxHeight: coretypes.Int64(maxHeight),
	})
}

func (c *Local) Genesis(ctx context.Context) (*coretypes.ResultGenesis, error) {
	return c.env.Genesis(ctx)
}

func (c *Local) GenesisChunked(ctx context.Context, id uint) (*coretypes.ResultGenesisChunk, error) {
	return c.env.GenesisChunked(ctx, &coretypes.RequestGenesisChunked{Chunk: coretypes.Int64(id)})
}

func (c *Local) Block(ctx context.Context, height *int64) (*coretypes.ResultBlock, error) {
	return c.env.Block(ctx, &coretypes.RequestBlockInfo{Height: (*coretypes.Int64)(height)})
}

func (c *Local) BlockByHash(ctx context.Context, hash bytes.HexBytes) (*coretypes.ResultBlock, error) {
	return c.env.BlockByHash(ctx, &coretypes.RequestBlockByHash{Hash: hash})
}

func (c *Local) BlockResults(ctx context.Context, height *int64) (*coretypes.ResultBlockResults, error) {
	return c.env.BlockResults(ctx, &coretypes.RequestBlockInfo{Height: (*coretypes.Int64)(height)})
}

func (c *Local) Header(ctx context.Context, height *int64) (*coretypes.ResultHeader, error) {
	return c.env.Header(ctx, &coretypes.RequestBlockInfo{Height: (*coretypes.Int64)(height)})
}

func (c *Local) HeaderByHash(ctx context.Context, hash bytes.HexBytes) (*coretypes.ResultHeader, error) {
	return c.env.HeaderByHash(ctx, &coretypes.RequestBlockByHash{Hash: hash})
}

func (c *Local) Commit(ctx context.Context, height *int64) (*coretypes.ResultCommit, error) {
	return c.env.Commit(ctx, &coretypes.RequestBlockInfo{Height: (*coretypes.Int64)(height)})
}

func (c *Local) Validators(ctx context.Context, height *int64, page, perPage *int) (*coretypes.ResultValidators, error) {
	return c.env.Validators(ctx, &coretypes.RequestValidators{
		Height:  (*coretypes.Int64)(height),
		Page:    coretypes.Int64Ptr(page),
		PerPage: coretypes.Int64Ptr(perPage),
	})
}

func (c *Local) Tx(ctx context.Context, hash bytes.HexBytes, prove bool) (*coretypes.ResultTx, error) {
	return c.env.Tx(ctx, &coretypes.RequestTx{Hash: hash, Prove: prove})
}

func (c *Local) TxSearch(ctx context.Context, queryString string, prove bool, page, perPage *int, orderBy string) (*coretypes.ResultTxSearch, error) {
	return c.env.TxSearch(ctx, &coretypes.RequestTxSearch{
		Query:   queryString,
		Prove:   prove,
		Page:    coretypes.Int64Ptr(page),
		PerPage: coretypes.Int64Ptr(perPage),
		OrderBy: orderBy,
	})
}

func (c *Local) BlockSearch(ctx context.Context, queryString string, page, perPage *int, orderBy string) (*coretypes.ResultBlockSearch, error) {
	return c.env.BlockSearch(ctx, &coretypes.RequestBlockSearch{
		Query:   queryString,
		Page:    coretypes.Int64Ptr(page),
		PerPage: coretypes.Int64Ptr(perPage),
		OrderBy: orderBy,
	})
}

func (c *Local) BroadcastEvidence(ctx context.Context, ev types.Evidence) (*coretypes.ResultBroadcastEvidence, error) {
	return c.env.BroadcastEvidence(ctx, &coretypes.RequestBroadcastEvidence{Evidence: ev})
}

func (c *Local) Subscribe(ctx context.Context, subscriber, queryString string, capacity ...int) (<-chan coretypes.ResultEvent, error) {
	q, err := query.New(queryString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	limit, quota := 1, 0
	if len(capacity) > 0 {
		limit = capacity[0]
		if len(capacity) > 1 {
			quota = capacity[1]
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	go func() { c.Wait(); cancel() }()

	subArgs := pubsub.SubscribeArgs{
		ClientID: subscriber,
		Query:    q,
		Quota:    quota,
		Limit:    limit,
	}
	sub, err := c.EventBus.SubscribeWithArgs(ctx, subArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	outc := make(chan coretypes.ResultEvent, 1)
	go c.eventsRoutine(ctx, sub, subArgs, outc)

	return outc, nil
}

func (c *Local) eventsRoutine(ctx context.Context, sub eventbus.Subscription, subArgs pubsub.SubscribeArgs, outc chan<- coretypes.ResultEvent) {
	qstr := subArgs.Query.String()
	for {
		msg, err := sub.Next(ctx)
		if errors.Is(err, pubsub.ErrUnsubscribed) {
			return // client unsubscribed
		} else if err != nil {
			c.Logger.Error("subscription was canceled, resubscribing",
				"err", err, "query", subArgs.Query.String())
			sub = c.resubscribe(ctx, subArgs)
			if sub == nil {
				return // client terminated
			}
			continue
		}
		select {
		case outc <- coretypes.ResultEvent{
			SubscriptionID: msg.SubscriptionID(),
			Query:          qstr,
			Data:           msg.Data(),
			Events:         msg.Events(),
		}:
		case <-ctx.Done():
			return
		}
	}
}

// Try to resubscribe with exponential backoff.
func (c *Local) resubscribe(ctx context.Context, subArgs pubsub.SubscribeArgs) eventbus.Subscription {
	timer := time.NewTimer(0)
	defer timer.Stop()

	attempts := 0
	for {
		if !c.IsRunning() {
			return nil
		}

		sub, err := c.EventBus.SubscribeWithArgs(ctx, subArgs)
		if err == nil {
			return sub
		}

		attempts++
		timer.Reset((10 << uint(attempts)) * time.Millisecond) // 10ms -> 20ms -> 40ms
		select {
		case <-timer.C:
			continue
		case <-ctx.Done():
			return nil
		}
	}
}

func (c *Local) Unsubscribe(ctx context.Context, subscriber, queryString string) error {
	args := pubsub.UnsubscribeArgs{Subscriber: subscriber}
	var err error
	args.Query, err = query.New(queryString)
	if err != nil {
		// if this isn't a valid query it might be an ID, so
		// we'll try that. It'll turn into an error when we
		// try to unsubscribe. Eventually, perhaps, we'll want
		// to change the interface to only allow
		// unsubscription by ID, but that's a larger change.
		args.ID = queryString
	}
	return c.EventBus.Unsubscribe(ctx, args)
}

func (c *Local) UnsubscribeAll(ctx context.Context, subscriber string) error {
	return c.EventBus.UnsubscribeAll(ctx, subscriber)
}
