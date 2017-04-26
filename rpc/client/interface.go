/*
package client provides a general purpose interface (Client) for connecting
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
package client

import (
	data "github.com/tendermint/go-wire/data"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

// ABCIClient groups together the functionality that principally
// affects the ABCI app. In many cases this will be all we want,
// so we can accept an interface which is easier to mock
type ABCIClient interface {
	// reading from abci app
	ABCIInfo() (*ctypes.ResultABCIInfo, error)
	ABCIQuery(path string, data data.Bytes, prove bool) (*ctypes.ResultABCIQuery, error)

	// writing to abci app
	BroadcastTxCommit(tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error)
	BroadcastTxAsync(tx types.Tx) (*ctypes.ResultBroadcastTx, error)
	BroadcastTxSync(tx types.Tx) (*ctypes.ResultBroadcastTx, error)
}

// SignClient groups together the interfaces need to get valid
// signatures and prove anything about the chain
type SignClient interface {
	Block(height int) (*ctypes.ResultBlock, error)
	Commit(height int) (*ctypes.ResultCommit, error)
	Validators() (*ctypes.ResultValidators, error)
	Tx(hash []byte, prove bool) (*ctypes.ResultTx, error)
}

// HistoryClient shows us data from genesis to now in large chunks.
type HistoryClient interface {
	Genesis() (*ctypes.ResultGenesis, error)
	BlockchainInfo(minHeight, maxHeight int) (*ctypes.ResultBlockchainInfo, error)
}

type StatusClient interface {
	// general chain info
	Status() (*ctypes.ResultStatus, error)
}

// Client wraps most important rpc calls a client would make
// if you want to listen for events, test if it also
// implements events.EventSwitch
type Client interface {
	ABCIClient
	SignClient
	HistoryClient
	StatusClient

	// this Client is reactive, you can subscribe to any TMEventData
	// type, given the proper string. see tendermint/types/events.go
	types.EventSwitch
}

// NetworkClient is general info about the network state.  May not
// be needed usually.
//
// Not included in the Client interface, but generally implemented
// by concrete implementations.
type NetworkClient interface {
	NetInfo() (*ctypes.ResultNetInfo, error)
	DumpConsensusState() (*ctypes.ResultDumpConsensusState, error)
}
