package client

import (
	"context"
	"time"

	"github.com/pkg/errors"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	tmquery "github.com/tendermint/tendermint/libs/pubsub/query"
	nm "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/rpc/core"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
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
}

// NewLocal configures a client that calls the Node directly.
//
// Note that given how rpc/core works with package singletons, that
// you can only have one node per process.  So make sure test cases
// don't run in parallel, or try to simulate an entire network in
// one process...
func NewLocal(node *nm.Node) *Local {
	node.ConfigureRPC()
	return &Local{
		EventBus: node.EventBus(),
		Logger:   log.NewNopLogger(),
		ctx:      &rpctypes.Context{},
	}
}

var _ Client = (*Local)(nil)

// SetLogger allows to set a logger on the client.
func (c *Local) SetLogger(l log.Logger) {
	c.Logger = l
}

func (c *Local) Status() (*ctypes.ResultStatus, error) {
	return core.Status(c.ctx)
}

func (c *Local) ABCIInfo() (*ctypes.ResultABCIInfo, error) {
	return core.ABCIInfo(c.ctx)
}

func (c *Local) ABCIQuery(path string, data cmn.HexBytes) (*ctypes.ResultABCIQuery, error) {
	return c.ABCIQueryWithOptions(path, data, DefaultABCIQueryOptions)
}

func (c *Local) ABCIQueryWithOptions(path string, data cmn.HexBytes, opts ABCIQueryOptions) (*ctypes.ResultABCIQuery, error) {
	return core.ABCIQuery(c.ctx, path, data, opts.Height, opts.Prove)
}

func (c *Local) BroadcastTxCommit(tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	return core.BroadcastTxCommit(c.ctx, tx)
}

func (c *Local) BroadcastTxAsync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return core.BroadcastTxAsync(c.ctx, tx)
}

func (c *Local) BroadcastTxSync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return core.BroadcastTxSync(c.ctx, tx)
}

func (c *Local) UnconfirmedTxs(limit int) (*ctypes.ResultUnconfirmedTxs, error) {
	return core.UnconfirmedTxs(c.ctx, limit)
}

func (c *Local) NumUnconfirmedTxs() (*ctypes.ResultUnconfirmedTxs, error) {
	return core.NumUnconfirmedTxs(c.ctx)
}

func (c *Local) NetInfo() (*ctypes.ResultNetInfo, error) {
	return core.NetInfo(c.ctx)
}

func (c *Local) DumpConsensusState() (*ctypes.ResultDumpConsensusState, error) {
	return core.DumpConsensusState(c.ctx)
}

func (c *Local) ConsensusState() (*ctypes.ResultConsensusState, error) {
	return core.ConsensusState(c.ctx)
}

func (c *Local) Health() (*ctypes.ResultHealth, error) {
	return core.Health(c.ctx)
}

func (c *Local) DialSeeds(seeds []string) (*ctypes.ResultDialSeeds, error) {
	return core.UnsafeDialSeeds(c.ctx, seeds)
}

func (c *Local) DialPeers(peers []string, persistent bool) (*ctypes.ResultDialPeers, error) {
	return core.UnsafeDialPeers(c.ctx, peers, persistent)
}

func (c *Local) BlockchainInfo(minHeight, maxHeight int64) (*ctypes.ResultBlockchainInfo, error) {
	return core.BlockchainInfo(c.ctx, minHeight, maxHeight)
}

func (c *Local) Genesis() (*ctypes.ResultGenesis, error) {
	return core.Genesis(c.ctx)
}

func (c *Local) Block(height *int64) (*ctypes.ResultBlock, error) {
	return core.Block(c.ctx, height)
}

func (c *Local) BlockResults(height *int64) (*ctypes.ResultBlockResults, error) {
	return core.BlockResults(c.ctx, height)
}

func (c *Local) Commit(height *int64) (*ctypes.ResultCommit, error) {
	return core.Commit(c.ctx, height)
}

func (c *Local) Validators(height *int64) (*ctypes.ResultValidators, error) {
	return core.Validators(c.ctx, height)
}

func (c *Local) Tx(hash []byte, prove bool) (*ctypes.ResultTx, error) {
	return core.Tx(c.ctx, hash, prove)
}

func (c *Local) TxSearch(query string, prove bool, page, perPage int) (*ctypes.ResultTxSearch, error) {
	return core.TxSearch(c.ctx, query, prove, page, perPage)
}

func (c *Local) BroadcastEvidence(ev types.Evidence) (*ctypes.ResultBroadcastEvidence, error) {
	return core.BroadcastEvidence(c.ctx, ev)
}

func (c *Local) Subscribe(ctx context.Context, subscriber, query string, outCapacity ...int) (out <-chan ctypes.ResultEvent, err error) {
	q, err := tmquery.New(query)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse query")
	}
	sub, err := c.EventBus.Subscribe(ctx, subscriber, q)
	if err != nil {
		return nil, errors.Wrap(err, "failed to subscribe")
	}

	outCap := 1
	if len(outCapacity) > 0 {
		outCap = outCapacity[0]
	}

	outc := make(chan ctypes.ResultEvent, outCap)
	go c.eventsRoutine(sub, subscriber, q, outc)

	return outc, nil
}

func (c *Local) eventsRoutine(sub types.Subscription, subscriber string, q tmpubsub.Query, outc chan<- ctypes.ResultEvent) {
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
		case <-sub.Cancelled():
			if sub.Err() == tmpubsub.ErrUnsubscribed {
				return
			}

			c.Logger.Error("subscription was cancelled, resubscribing...", "err", sub.Err(), "query", q.String())
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
		return errors.Wrap(err, "failed to parse query")
	}
	return c.EventBus.Unsubscribe(ctx, subscriber, q)
}

func (c *Local) UnsubscribeAll(ctx context.Context, subscriber string) error {
	return c.EventBus.UnsubscribeAll(ctx, subscriber)
}
