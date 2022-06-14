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
	"github.com/tendermint/tendermint/rpc/coretypes"
	"github.com/tendermint/tendermint/types"
)

//go:generate ../../scripts/mockery_generate.sh Client

// Client describes the interface of Tendermint RPC client implementations.
type Client interface {
	// Start the client, which will run until the context terminates.
	// An error from Start indicates the client could not start.
	Start(context.Context) error

	// These embedded interfaces define the callable methods of the service.

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
	ABCIInfo(context.Context) (*coretypes.ResultABCIInfo, error)
	ABCIQuery(ctx context.Context, path string, data bytes.HexBytes) (*coretypes.ResultABCIQuery, error)
	ABCIQueryWithOptions(ctx context.Context, path string, data bytes.HexBytes,
		opts ABCIQueryOptions) (*coretypes.ResultABCIQuery, error)

	// Writing to abci app
	BroadcastTx(context.Context, types.Tx) (*coretypes.ResultBroadcastTx, error)
	// These methods are deprecated:
	BroadcastTxCommit(context.Context, types.Tx) (*coretypes.ResultBroadcastTxCommit, error)
	BroadcastTxAsync(context.Context, types.Tx) (*coretypes.ResultBroadcastTx, error)
	BroadcastTxSync(context.Context, types.Tx) (*coretypes.ResultBroadcastTx, error)
}

// SignClient groups together the functionality needed to get valid signatures
// and prove anything about the chain.
type SignClient interface {
	Block(ctx context.Context, height *int64) (*coretypes.ResultBlock, error)
	BlockByHash(ctx context.Context, hash bytes.HexBytes) (*coretypes.ResultBlock, error)
	BlockResults(ctx context.Context, height *int64) (*coretypes.ResultBlockResults, error)
	Header(ctx context.Context, height *int64) (*coretypes.ResultHeader, error)
	HeaderByHash(ctx context.Context, hash bytes.HexBytes) (*coretypes.ResultHeader, error)
	Commit(ctx context.Context, height *int64) (*coretypes.ResultCommit, error)
	Validators(ctx context.Context, height *int64, page, perPage *int) (*coretypes.ResultValidators, error)
	Tx(ctx context.Context, hash bytes.HexBytes, prove bool) (*coretypes.ResultTx, error)

	// TxSearch defines a method to search for a paginated set of transactions by
	// DeliverTx event search criteria.
	TxSearch(
		ctx context.Context,
		query string,
		prove bool,
		page, perPage *int,
		orderBy string,
	) (*coretypes.ResultTxSearch, error)

	// BlockSearch defines a method to search for a paginated set of blocks by
	// FinalizeBlock event search criteria.
	BlockSearch(
		ctx context.Context,
		query string,
		page, perPage *int,
		orderBy string,
	) (*coretypes.ResultBlockSearch, error)
}

// HistoryClient provides access to data from genesis to now in large chunks.
type HistoryClient interface {
	Genesis(context.Context) (*coretypes.ResultGenesis, error)
	GenesisChunked(context.Context, uint) (*coretypes.ResultGenesisChunk, error)
	BlockchainInfo(ctx context.Context, minHeight, maxHeight int64) (*coretypes.ResultBlockchainInfo, error)
}

// StatusClient provides access to general chain info.
type StatusClient interface {
	Status(context.Context) (*coretypes.ResultStatus, error)
}

// NetworkClient is general info about the network state. May not be needed
// usually.
type NetworkClient interface {
	NetInfo(context.Context) (*coretypes.ResultNetInfo, error)
	DumpConsensusState(context.Context) (*coretypes.ResultDumpConsensusState, error)
	ConsensusState(context.Context) (*coretypes.ResultConsensusState, error)
	ConsensusParams(ctx context.Context, height *int64) (*coretypes.ResultConsensusParams, error)
	Health(context.Context) (*coretypes.ResultHealth, error)
}

// EventsClient exposes the methods to retrieve events from the consensus engine.
type EventsClient interface {
	// Events fetches a batch of events from the server matching the given query
	// and time range.
	Events(ctx context.Context, req *coretypes.RequestEvents) (*coretypes.ResultEvents, error)
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
	// Deprecated: This method will be removed in Tendermint v0.37, use Events
	// instead.
	Subscribe(ctx context.Context, subscriber, query string, outCapacity ...int) (out <-chan coretypes.ResultEvent, err error)

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
	UnconfirmedTxs(ctx context.Context, page, perPage *int) (*coretypes.ResultUnconfirmedTxs, error)
	NumUnconfirmedTxs(context.Context) (*coretypes.ResultUnconfirmedTxs, error)
	CheckTx(context.Context, types.Tx) (*coretypes.ResultCheckTx, error)
	RemoveTx(context.Context, types.TxKey) error
}

// EvidenceClient is used for submitting an evidence of the malicious
// behavior.
type EvidenceClient interface {
	BroadcastEvidence(context.Context, types.Evidence) (*coretypes.ResultBroadcastEvidence, error)
}

// RemoteClient is a Client, which can also return the remote network address.
type RemoteClient interface {
	Client

	// Remote returns the remote network address in a string form.
	Remote() string
}
