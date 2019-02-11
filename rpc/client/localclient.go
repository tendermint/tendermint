package client

import (
	"context"

	"github.com/pkg/errors"

	cmn "github.com/tendermint/tendermint/libs/common"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	tmquery "github.com/tendermint/tendermint/libs/pubsub/query"
	nm "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/rpc/core"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
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
*/
type Local struct {
	*types.EventBus
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
	}
}

var (
	_ Client        = (*Local)(nil)
	_ NetworkClient = Local{}
	_ EventsClient  = (*Local)(nil)
)

func (Local) Status() (*ctypes.ResultStatus, error) {
	return core.Status()
}

func (Local) ABCIInfo() (*ctypes.ResultABCIInfo, error) {
	return core.ABCIInfo()
}

func (c *Local) ABCIQuery(path string, data cmn.HexBytes) (*ctypes.ResultABCIQuery, error) {
	return c.ABCIQueryWithOptions(path, data, DefaultABCIQueryOptions)
}

func (Local) ABCIQueryWithOptions(path string, data cmn.HexBytes, opts ABCIQueryOptions) (*ctypes.ResultABCIQuery, error) {
	return core.ABCIQuery(path, data, opts.Height, opts.Prove)
}

func (Local) BroadcastTxCommit(tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	return core.BroadcastTxCommit(tx)
}

func (Local) BroadcastTxAsync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return core.BroadcastTxAsync(tx)
}

func (Local) BroadcastTxSync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return core.BroadcastTxSync(tx)
}

func (Local) UnconfirmedTxs(limit int) (*ctypes.ResultUnconfirmedTxs, error) {
	return core.UnconfirmedTxs(limit)
}

func (Local) NumUnconfirmedTxs() (*ctypes.ResultUnconfirmedTxs, error) {
	return core.NumUnconfirmedTxs()
}

func (Local) NetInfo() (*ctypes.ResultNetInfo, error) {
	return core.NetInfo()
}

func (Local) DumpConsensusState() (*ctypes.ResultDumpConsensusState, error) {
	return core.DumpConsensusState()
}

func (Local) ConsensusState() (*ctypes.ResultConsensusState, error) {
	return core.ConsensusState()
}

func (Local) Health() (*ctypes.ResultHealth, error) {
	return core.Health()
}

func (Local) DialSeeds(seeds []string) (*ctypes.ResultDialSeeds, error) {
	return core.UnsafeDialSeeds(seeds)
}

func (Local) DialPeers(peers []string, persistent bool) (*ctypes.ResultDialPeers, error) {
	return core.UnsafeDialPeers(peers, persistent)
}

func (Local) BlockchainInfo(minHeight, maxHeight int64) (*ctypes.ResultBlockchainInfo, error) {
	return core.BlockchainInfo(minHeight, maxHeight)
}

func (Local) Genesis() (*ctypes.ResultGenesis, error) {
	return core.Genesis()
}

func (Local) Block(height *int64) (*ctypes.ResultBlock, error) {
	return core.Block(height)
}

func (Local) BlockResults(height *int64) (*ctypes.ResultBlockResults, error) {
	return core.BlockResults(height)
}

func (Local) Commit(height *int64) (*ctypes.ResultCommit, error) {
	return core.Commit(height)
}

func (Local) Validators(height *int64) (*ctypes.ResultValidators, error) {
	return core.Validators(height)
}

func (Local) Tx(hash []byte, prove bool) (*ctypes.ResultTx, error) {
	return core.Tx(hash, prove)
}

func (Local) TxSearch(query string, prove bool, page, perPage int) (*ctypes.ResultTxSearch, error) {
	return core.TxSearch(query, prove, page, perPage)
}

// Subscribe implements EventsClient by using local eventBus to subscribe given
// subscriber to query.
func (c *Local) Subscribe(ctx context.Context, subscriber, query string, outCapacity ...int) (out <-chan ctypes.ResultEvent, err error) {
	q, err := tmquery.New(query)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse query")
	}
	sub, err := c.EventBus.Subscribe(ctx, subscriber, q)
	if err != nil {
		return nil, errors.Wrap(err, "failed to subscribe")
	}

	outCap := 0
	if len(outCapacity) > 0 {
		outCap = outCapacity[0]
	}

	outc := make(chan ctypes.ResultEvent, outCap)
	go func(sub types.Subscription) {
		for {
			select {
			case msg := <-sub.Out():
				if cap(outc) == 0 {
					outc <- ctypes.ResultEvent{Query: query, Data: msg.Data(), Tags: msg.Tags()}
				} else {
					select {
					case outc <- ctypes.ResultEvent{Query: query, Data: msg.Data(), Tags: msg.Tags()}:
					default:
						// XXX: log error
					}
				}
			case <-sub.Cancelled():
				if sub.Err() != tmpubsub.ErrUnsubscribed {
					// resubscribe with exponential timeout
					var err error
					sub, err = c.EventBus.Subscribe(ctx, subscriber, q)
					if err != nil {
						// TODO
					}
				}
				return
			}
		}
	}(sub)

	return outc, nil
}

// Unsubscribe implements EventsClient by using local eventBus to unsubscribe
// given subscriber from query.
func (c *Local) Unsubscribe(ctx context.Context, subscriber, query string) error {
	q, err := tmquery.New(query)
	if err != nil {
		return errors.Wrap(err, "failed to parse query")
	}
	return c.EventBus.Unsubscribe(ctx, subscriber, q)
}

// UnsubscribeAll implements EventsClient by using local eventBus to
// unsubscribe given subscriber from all queries.
func (c *Local) UnsubscribeAll(ctx context.Context, subscriber string) error {
	return c.EventBus.UnsubscribeAll(ctx, subscriber)
}
