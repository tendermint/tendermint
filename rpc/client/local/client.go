/*
package local returns a Client implementation that
directly executes the rpc functions on a given node.

This implementation is useful for:

* Running tests against a node in-process without the overhead
of going through an http server
* Communication between an ABCI app and tendermin core when they
are compiled in process.

For real clients, you probably want the "http" package.  For more
powerful control during testing, you probably want the "mock" package.
*/
package local

import (
	nm "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/core"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

type Client struct {
	node *nm.Node
}

// New configures this to call the Node directly.
//
// Note that given how rpc/core works with package singletons, that
// you can only have one node per process.  So make sure test cases
// don't run in parallel, or try to simulate an entire network in
// one process...
func New(node *nm.Node) Client {
	node.ConfigureRPC()
	return Client{
		node: node,
	}
}

func (c Client) _assertIsClient() client.Client {
	return c
}

func (c Client) Status() (*ctypes.ResultStatus, error) {
	return core.Status()
}

func (c Client) ABCIInfo() (*ctypes.ResultABCIInfo, error) {
	return core.ABCIInfo()
}

func (c Client) ABCIQuery(path string, data []byte, prove bool) (*ctypes.ResultABCIQuery, error) {
	return core.ABCIQuery(path, data, prove)
}

func (c Client) BroadcastTxCommit(tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	return core.BroadcastTxCommit(tx)
}

func (c Client) BroadcastTxAsync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return core.BroadcastTxAsync(tx)
}

func (c Client) BroadcastTxSync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return core.BroadcastTxSync(tx)
}

func (c Client) NetInfo() (*ctypes.ResultNetInfo, error) {
	return core.NetInfo()
}

func (c Client) DialSeeds(seeds []string) (*ctypes.ResultDialSeeds, error) {
	return core.UnsafeDialSeeds(seeds)
}

func (c Client) BlockchainInfo(minHeight, maxHeight int) (*ctypes.ResultBlockchainInfo, error) {
	return core.BlockchainInfo(minHeight, maxHeight)
}

func (c Client) Genesis() (*ctypes.ResultGenesis, error) {
	return core.Genesis()
}

func (c Client) Block(height int) (*ctypes.ResultBlock, error) {
	return core.Block(height)
}

func (c Client) Commit(height int) (*ctypes.ResultCommit, error) {
	return core.Commit(height)
}

func (c Client) Validators() (*ctypes.ResultValidators, error) {
	return core.Validators()
}

/** websocket event stuff here... **/

/*
// StartWebsocket starts up a websocket and a listener goroutine
// if already started, do nothing
func (c Client) StartWebsocket() error {
	var err error
	if c.ws == nil {
		ws := rpcclient.NewWSClient(c.remote, c.endpoint)
		_, err = ws.Start()
		if err == nil {
			c.ws = ws
		}
	}
	return errors.Wrap(err, "StartWebsocket")
}

// StopWebsocket stops the websocket connection
func (c Client) StopWebsocket() {
	if c.ws != nil {
		c.ws.Stop()
		c.ws = nil
	}
}

// GetEventChannels returns the results and error channel from the websocket
func (c Client) GetEventChannels() (chan json.RawMessage, chan error) {
	if c.ws == nil {
		return nil, nil
	}
	return c.ws.ResultsCh, c.ws.ErrorsCh
}

func (c Client) Subscribe(event string) error {
	return errors.Wrap(c.ws.Subscribe(event), "Subscribe")
}

func (c Client) Unsubscribe(event string) error {
	return errors.Wrap(c.ws.Unsubscribe(event), "Unsubscribe")
}

*/
