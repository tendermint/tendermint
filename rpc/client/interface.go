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

	cmn "github.com/tendermint/tendermint/libs/common"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

// Client wraps most important rpc calls a client would make if you want to
// listen for events, test if it also implements events.EventSwitch.
type Client interface {
	cmn.Service
	ABCIClient
	EventsClient
	HistoryClient
	NetworkClient
	SignClient
	StatusClient
	EvidenceClient
	MempoolClient
}

// ABCIClient groups together the functionality that principally affects the
// ABCI app.
//
// In many cases this will be all we want, so we can accept an interface which
// is easier to mock.
type ABCIClient interface {
	// Reading from abci app
	ABCIInfo() (*ctypes.ResultABCIInfo, error)
	ABCIQuery(path string, data cmn.HexBytes) (*ctypes.ResultABCIQuery, error)
	ABCIQueryWithOptions(path string, data cmn.HexBytes,
		opts ABCIQueryOptions) (*ctypes.ResultABCIQuery, error)

	// Writing to abci app
	BroadcastTxCommit(tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error)
	BroadcastTxAsync(tx types.Tx) (*ctypes.ResultBroadcastTx, error)
	BroadcastTxSync(tx types.Tx) (*ctypes.ResultBroadcastTx, error)
}

// SignClient groups together the functionality needed to get valid signatures
// and prove anything about the chain.
type SignClient interface {
	Block(height *int64) (*ctypes.ResultBlock, error)
	BlockResults(height *int64) (*ctypes.ResultBlockResults, error)
	Commit(height *int64) (*ctypes.ResultCommit, error)
	Validators(height *int64) (*ctypes.ResultValidators, error)
	Tx(hash []byte, prove bool) (*ctypes.ResultTx, error)
	TxSearch(query string, prove bool, page, perPage int) (*ctypes.ResultTxSearch, error)
}

// HistoryClient provides access to data from genesis to now in large chunks.
type HistoryClient interface {
	Genesis() (*ctypes.ResultGenesis, error)
	BlockchainInfo(minHeight, maxHeight int64) (*ctypes.ResultBlockchainInfo, error)
}

// StatusClient provides access to general chain info.
type StatusClient interface {
	Status() (*ctypes.ResultStatus, error)
}

// NetworkClient is general info about the network state. May not be needed
// usually.
type NetworkClient interface {
	NetInfo() (*ctypes.ResultNetInfo, error)
	DumpConsensusState() (*ctypes.ResultDumpConsensusState, error)
	ConsensusState() (*ctypes.ResultConsensusState, error)
	Health() (*ctypes.ResultHealth, error)
}

// EventsClient is reactive, you can subscribe to any message, given the proper
// string. see tendermint/types/events.go
type EventsClient interface {
	// Subscribe subscribes given subscriber to query. Returns a channel with
	// cap=1 onto which events are published. An error is returned if it fails to
	// subscribe. outCapacity can be used optionally to set capacity for the
	// channel. Channel is never closed to prevent accidental reads.
	//
	// ctx cannot be used to unsubscribe. To unsubscribe, use either Unsubscribe
	// or UnsubscribeAll.
	Subscribe(ctx context.Context, subscriber, query string, outCapacity ...int) (out <-chan ctypes.ResultEvent, err error)
	// Unsubscribe unsubscribes given subscriber from query.
	Unsubscribe(ctx context.Context, subscriber, query string) error
	// UnsubscribeAll unsubscribes given subscriber from all the queries.
	UnsubscribeAll(ctx context.Context, subscriber string) error
}

// MempoolClient shows us data about current mempool state.
type MempoolClient interface {
	UnconfirmedTxs(limit int) (*ctypes.ResultUnconfirmedTxs, error)
	NumUnconfirmedTxs() (*ctypes.ResultUnconfirmedTxs, error)
}

// EvidenceClient is used for submitting an evidence of the malicious
// behaviour.
type EvidenceClient interface {
	BroadcastEvidence(ev types.Evidence) (*ctypes.ResultBroadcastEvidence, error)
}
