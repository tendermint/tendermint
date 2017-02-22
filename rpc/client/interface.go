/*
package client provides a general purpose interface for connecting
to a tendermint node, as well as higher-level functionality.

The main implementation for production code is http, which connects
via http to the jsonrpc interface of the tendermint node.

For connecting to a node running in the same process (eg. when
compiling the abci app in the same process), you can use the local
implementation.

For mocking out server responses during testing to see behavior for
arbitrary return values, use the mock package.

In addition to the Client interface, which should be used externally
for maximum flexibility and testability, this package also provides
a wrapper that accepts any Client implementation and adds some
higher-level functionality.
*/
package client

import (
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

type Client interface {
	// general chain info
	Status() (*ctypes.ResultStatus, error)
	NetInfo() (*ctypes.ResultNetInfo, error)
	Genesis() (*ctypes.ResultGenesis, error)

	// reading from abci app
	ABCIInfo() (*ctypes.ResultABCIInfo, error)
	ABCIQuery(path string, data []byte, prove bool) (*ctypes.ResultABCIQuery, error)

	// writing to abci app
	BroadcastTxCommit(tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error)
	BroadcastTxAsync(tx types.Tx) (*ctypes.ResultBroadcastTx, error)
	BroadcastTxSync(tx types.Tx) (*ctypes.ResultBroadcastTx, error)

	// validating block info
	BlockchainInfo(minHeight, maxHeight int) (*ctypes.ResultBlockchainInfo, error)
	Block(height int) (*ctypes.ResultBlock, error)
	Commit(height int) (*ctypes.ResultCommit, error)
	Validators() (*ctypes.ResultValidators, error)

	// TODO: add some sort of generic subscription mechanism...

	// remove this???
	// DialSeeds(seeds []string) (*ctypes.ResultDialSeeds, error)
}
