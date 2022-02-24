package client

/*
The client package provides a general purpose interface (Client) for connecting
to a tendermint node, as well as higher-level functionality.

The main implementation for production code is client.HTTP, which
connects via http to the jsonrpc interface of the tendermint node.

For connecting to a node running in the same process (eg. when
compiling the abci app in the same process), you can use the client.Local
implementation.

For mocking out server responses during testing to see behavior for
arbitrary return values, use the mock package.

In addition to the Client interface, which should be used externally
for maximum flexibility and testability, and two implementations,
this package also provides helper functions that work on any Client
implementation.
*/

import (
	"context"

	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/service"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

// Client wraps most important rpc calls a client would make if you want to
// listen for events, test if it also implements events.EventSwitch.
type Client interface {
	service.Service
	ABCIClient
	EventsClient
	EvidenceClient
	HistoryClient
	MempoolClient
	NetworkClient
	SignClient
	StatusClient
	SubscriptionClient
}

// ABCIClient groups together the functionality that principally affects the
// ABCI app.
//
// In many cases this will be all we want, so we can accept an interface which
// is easier to mock.
type ABCIClient interface {
	// Reading from abci app
	ABCIInfo(context.Context) (*ctypes.ResultABCIInfo, error)
	ABCIQuery(ctx context.Context, path string, data bytes.HexBytes) (*ctypes.ResultABCIQuery, error)
	ABCIQueryWithOptions(ctx context.Context, path string, data bytes.HexBytes,
		opts ABCIQueryOptions) (*ctypes.ResultABCIQuery, error)

	// Writing to abci app
	BroadcastTxCommit(context.Context, types.Tx) (*ctypes.ResultBroadcastTxCommit, error)
	BroadcastTxAsync(context.Context, types.Tx) (*ctypes.ResultBroadcastTx, error)
	BroadcastTxSync(context.Context, types.Tx) (*ctypes.ResultBroadcastTx, error)
}

// SignClient groups together the functionality needed to get valid signatures
// and prove anything about the chain.
type SignClient interface {
	Block(ctx context.Context, height *int64) (*ctypes.ResultBlock, error)
	BlockByHash(ctx context.Context, hash []byte) (*ctypes.ResultBlock, error)
	BlockResults(ctx context.Context, height *int64) (*ctypes.ResultBlockResults, error)
	Header(ctx context.Context, height *int64) (*ctypes.ResultHeader, error)
	HeaderByHash(ctx context.Context, hash bytes.HexBytes) (*ctypes.ResultHeader, error)
	Commit(ctx context.Context, height *int64) (*ctypes.ResultCommit, error)
	Validators(ctx context.Context, height *int64, page, perPage *int) (*ctypes.ResultValidators, error)
	Tx(ctx context.Context, hash []byte, prove bool) (*ctypes.ResultTx, error)

	// TxSearch defines a method to search for a paginated set of transactions by
	// DeliverTx event search criteria.
	TxSearch(
		ctx context.Context,
		query string,
		prove bool,
		page, perPage *int,
		orderBy string,
	) (*ctypes.ResultTxSearch, error)

	// BlockSearch defines a method to search for a paginated set of blocks by
	// BeginBlock and EndBlock event search criteria.
	BlockSearch(
		ctx context.Context,
		query string,
		page, perPage *int,
		orderBy string,
	) (*ctypes.ResultBlockSearch, error)
}

// HistoryClient provides access to data from genesis to now in large chunks.
type HistoryClient interface {
	Genesis(context.Context) (*ctypes.ResultGenesis, error)
	GenesisChunked(context.Context, uint) (*ctypes.ResultGenesisChunk, error)
	BlockchainInfo(ctx context.Context, minHeight, maxHeight int64) (*ctypes.ResultBlockchainInfo, error)
}

// StatusClient provides access to general chain info.
type StatusClient interface {
	Status(context.Context) (*ctypes.ResultStatus, error)
}

// NetworkClient is general info about the network state. May not be needed
// usually.
type NetworkClient interface {
	NetInfo(context.Context) (*ctypes.ResultNetInfo, error)
	DumpConsensusState(context.Context) (*ctypes.ResultDumpConsensusState, error)
	ConsensusState(context.Context) (*ctypes.ResultConsensusState, error)
	ConsensusParams(ctx context.Context, height *int64) (*ctypes.ResultConsensusParams, error)
	Health(context.Context) (*ctypes.ResultHealth, error)
}

// EventsClient exposes the methods to retrieve events from the consensus engine.
type EventsClient interface {
	// Events fetches a batch of events from the server matching the given query
	// and time range.
	Events(ctx context.Context, req *ctypes.RequestEvents) (*ctypes.ResultEvents, error)
}

// TODO(creachadair): This interface should be removed once the streaming event
// interface is removed in Tendermint v0.37.
type SubscriptionClient interface {
	// Subscribe issues a subscription request for the given subscriber ID and
	// query. This method does not block: If subscription fails, it reports an
	// error, and if subscription succeeds it returns a channel that delivers
	// matching events until the subscription is stopped. The channel is never
	// closed; the client is responsible for knowing when no further data will
	// be sent.
	//
	// The context only governs the initial subscription, it does not control
	// the lifetime of the channel. To cancel a subscription call Unsubscribe or
	// UnsubscribeAll.
	//
	// ctx cannot be used to unsubscribe. To unsubscribe, use either Unsubscribe
	// or UnsubscribeAll.
	Subscribe(ctx context.Context, subscriber, query string, outCapacity ...int) (out <-chan ctypes.ResultEvent, err error)
	// Unsubscribe unsubscribes given subscriber from query.
	//
	// Deprecated: This method will be removed in Tendermint v0.37, use Events
	// instead.
	Unsubscribe(ctx context.Context, subscriber, query string) error

	// UnsubscribeAll unsubscribes given subscriber from all the queries.
	//
	// Deprecated: This method will be removed in Tendermint v0.37, use Events
	// instead.
	UnsubscribeAll(ctx context.Context, subscriber string) error
}

// MempoolClient shows us data about current mempool state.
type MempoolClient interface {
	UnconfirmedTxs(ctx context.Context, limit *int) (*ctypes.ResultUnconfirmedTxs, error)
	NumUnconfirmedTxs(context.Context) (*ctypes.ResultUnconfirmedTxs, error)
	CheckTx(context.Context, types.Tx) (*ctypes.ResultCheckTx, error)
}

// EvidenceClient is used for submitting an evidence of the malicious
// behavior.
type EvidenceClient interface {
	BroadcastEvidence(context.Context, types.Evidence) (*ctypes.ResultBroadcastEvidence, error)
}

// RemoteClient is a Client, which can also return the remote network address.
type RemoteClient interface {
	Client

	// Remote returns the remote network address in a string form.
	Remote() string
}
