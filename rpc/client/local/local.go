package local

import (
	"context"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	tmquery "github.com/tendermint/tendermint/libs/pubsub/query"
	nm "github.com/tendermint/tendermint/node"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/core"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
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
	*types.EventBus
	Logger log.Logger
	ctx    *rpctypes.Context
	env    *core.Environment
}

// NewLocal configures a client that calls the Node directly.
func New(node *nm.Node) *Local {
	env, err := node.ConfigureRPC()
	if err != nil {
		node.Logger.Error("Error configuring RPC", "err", err)
	}
	return &Local{
		EventBus: node.EventBus(),
		Logger:   log.NewNopLogger(),
		ctx:      &rpctypes.Context{},
		env:      env,
	}
}

var _ rpcclient.Client = (*Local)(nil)

// SetLogger allows to set a logger on the client.
func (c *Local) SetLogger(l log.Logger) {
	c.Logger = l
}

func (c *Local) Status(ctx context.Context) (*ctypes.ResultStatus, error) {
	return c.env.Status(c.ctx)
}

func (c *Local) ABCIInfo(ctx context.Context) (*ctypes.ResultABCIInfo, error) {
	return c.env.ABCIInfo(c.ctx)
}

func (c *Local) ABCIQuery(ctx context.Context, path string, data bytes.HexBytes) (*ctypes.ResultABCIQuery, error) {
	return c.ABCIQueryWithOptions(ctx, path, data, rpcclient.DefaultABCIQueryOptions)
}

func (c *Local) ABCIQueryWithOptions(
	ctx context.Context,
	path string,
	data bytes.HexBytes,
	opts rpcclient.ABCIQueryOptions) (*ctypes.ResultABCIQuery, error) {
	return c.env.ABCIQuery(c.ctx, path, data, opts.Height, opts.Prove)
}

func (c *Local) BroadcastTxCommit(ctx context.Context, tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	return c.env.BroadcastTxCommit(c.ctx, tx)
}

func (c *Local) BroadcastTxAsync(ctx context.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return c.env.BroadcastTxAsync(c.ctx, tx)
}

func (c *Local) BroadcastTxSync(ctx context.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return c.env.BroadcastTxSync(c.ctx, tx)
}

func (c *Local) UnconfirmedTxs(ctx context.Context, limit *int) (*ctypes.ResultUnconfirmedTxs, error) {
	return c.env.UnconfirmedTxs(c.ctx, limit)
}

func (c *Local) NumUnconfirmedTxs(ctx context.Context) (*ctypes.ResultUnconfirmedTxs, error) {
	return c.env.NumUnconfirmedTxs(c.ctx)
}

func (c *Local) CheckTx(ctx context.Context, tx types.Tx) (*ctypes.ResultCheckTx, error) {
	return c.env.CheckTx(c.ctx, tx)
}

func (c *Local) NetInfo(ctx context.Context) (*ctypes.ResultNetInfo, error) {
	return c.env.NetInfo(c.ctx)
}

func (c *Local) DumpConsensusState(ctx context.Context) (*ctypes.ResultDumpConsensusState, error) {
	return c.env.DumpConsensusState(c.ctx)
}

func (c *Local) ConsensusState(ctx context.Context) (*ctypes.ResultConsensusState, error) {
	return c.env.GetConsensusState(c.ctx)
}

func (c *Local) ConsensusParams(ctx context.Context, height *int64) (*ctypes.ResultConsensusParams, error) {
	return c.env.ConsensusParams(c.ctx, height)
}

func (c *Local) Health(ctx context.Context) (*ctypes.ResultHealth, error) {
	return c.env.Health(c.ctx)
}

func (c *Local) DialSeeds(ctx context.Context, seeds []string) (*ctypes.ResultDialSeeds, error) {
	return c.env.UnsafeDialSeeds(c.ctx, seeds)
}

func (c *Local) DialPeers(
	ctx context.Context,
	peers []string,
	persistent,
	unconditional,
	private bool,
) (*ctypes.ResultDialPeers, error) {
	return c.env.UnsafeDialPeers(c.ctx, peers, persistent, unconditional, private)
}

func (c *Local) BlockchainInfo(ctx context.Context, minHeight, maxHeight int64) (*ctypes.ResultBlockchainInfo, error) {
	return c.env.BlockchainInfo(c.ctx, minHeight, maxHeight)
}

func (c *Local) Genesis(ctx context.Context) (*ctypes.ResultGenesis, error) {
	return c.env.Genesis(c.ctx)
}

func (c *Local) GenesisChunked(ctx context.Context, id uint) (*ctypes.ResultGenesisChunk, error) {
	return c.env.GenesisChunked(c.ctx, id)
}

func (c *Local) Block(ctx context.Context, height *int64) (*ctypes.ResultBlock, error) {
	return c.env.Block(c.ctx, height)
}

func (c *Local) BlockByHash(ctx context.Context, hash []byte) (*ctypes.ResultBlock, error) {
	return c.env.BlockByHash(c.ctx, hash)
}

func (c *Local) BlockResults(ctx context.Context, height *int64) (*ctypes.ResultBlockResults, error) {
	return c.env.BlockResults(c.ctx, height)
}

func (c *Local) Header(ctx context.Context, height *int64) (*ctypes.ResultHeader, error) {
	return c.env.Header(c.ctx, height)
}

func (c *Local) HeaderByHash(ctx context.Context, hash bytes.HexBytes) (*ctypes.ResultHeader, error) {
	return c.env.HeaderByHash(c.ctx, hash)
}

func (c *Local) Commit(ctx context.Context, height *int64) (*ctypes.ResultCommit, error) {
	return c.env.Commit(c.ctx, height)
}

func (c *Local) Validators(ctx context.Context, height *int64, page, perPage *int) (*ctypes.ResultValidators, error) {
	return c.env.Validators(c.ctx, height, page, perPage)
}

func (c *Local) Tx(ctx context.Context, hash []byte, prove bool) (*ctypes.ResultTx, error) {
	return c.env.Tx(c.ctx, hash, prove)
}

func (c *Local) TxSearch(
	_ context.Context,
	query string,
	prove bool,
	page,
	perPage *int,
	orderBy string,
) (*ctypes.ResultTxSearch, error) {
	return c.env.TxSearch(c.ctx, query, prove, page, perPage, orderBy)
}

func (c *Local) BlockSearch(
	_ context.Context,
	query string,
	page, perPage *int,
	orderBy string,
) (*ctypes.ResultBlockSearch, error) {
	return c.env.BlockSearch(c.ctx, query, page, perPage, orderBy)
}

func (c *Local) BroadcastEvidence(ctx context.Context, ev types.Evidence) (*ctypes.ResultBroadcastEvidence, error) {
	return c.env.BroadcastEvidence(c.ctx, ev)
}

func (c *Local) Subscribe(
	ctx context.Context,
	subscriber,
	query string,
	outCapacity ...int) (out <-chan ctypes.ResultEvent, err error) {
	q, err := tmquery.New(query)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	outCap := 1
	if len(outCapacity) > 0 {
		outCap = outCapacity[0]
	}

	var sub types.Subscription
	if outCap > 0 {
		sub, err = c.EventBus.Subscribe(ctx, subscriber, q, outCap)
	} else {
		sub, err = c.EventBus.SubscribeUnbuffered(ctx, subscriber, q)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	outc := make(chan ctypes.ResultEvent, outCap)
	go c.eventsRoutine(sub, subscriber, q, outc)

	return outc, nil
}

func (c *Local) eventsRoutine(
	sub types.Subscription,
	subscriber string,
	q tmpubsub.Query,
	outc chan<- ctypes.ResultEvent) {
	for {
		select {
		case msg := <-sub.Out():
			result := ctypes.ResultEvent{Query: q.String(), Data: msg.Data(), Events: msg.Events()}
			if cap(outc) == 0 {
				outc <- result
			} else {
				select {
				case outc <- result:
				default:
					c.Logger.Error("wanted to publish ResultEvent, but out channel is full", "result", result, "query", result.Query)
				}
			}
		case <-sub.Canceled():
			if sub.Err() == tmpubsub.ErrUnsubscribed {
				return
			}

			c.Logger.Error("subscription was canceled, resubscribing...", "err", sub.Err(), "query", q.String())
			sub = c.resubscribe(subscriber, q)
			if sub == nil { // client was stopped
				return
			}
		case <-c.Quit():
			return
		}
	}
}

// Try to resubscribe with exponential backoff.
func (c *Local) resubscribe(subscriber string, q tmpubsub.Query) types.Subscription {
	attempts := 0
	for {
		if !c.IsRunning() {
			return nil
		}

		sub, err := c.EventBus.Subscribe(context.Background(), subscriber, q)
		if err == nil {
			return sub
		}

		attempts++
		time.Sleep((10 << uint(attempts)) * time.Millisecond) // 10ms -> 20ms -> 40ms
	}
}

func (c *Local) Unsubscribe(ctx context.Context, subscriber, query string) error {
	q, err := tmquery.New(query)
	if err != nil {
		return fmt.Errorf("failed to parse query: %w", err)
	}
	return c.EventBus.Unsubscribe(ctx, subscriber, q)
}

func (c *Local) UnsubscribeAll(ctx context.Context, subscriber string) error {
	return c.EventBus.UnsubscribeAll(ctx, subscriber)
}
