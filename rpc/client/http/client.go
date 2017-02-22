/*
package httpclient returns a Client implementation that communicates
with a tendermint node over json rpc and websockets.

This is the main implementation you probably want to use in
production code.  There are other implementations when calling
the tendermint node in-process (local), or when you want to mock
out the server for test code (mock).
*/
package httpclient

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/tendermint/go-rpc/client"
	"github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

// Client is a Client implementation that communicates over
// JSONRPC
type Client struct {
	remote   string
	endpoint string
	rpc      *rpcclient.ClientJSONRPC
	ws       *rpcclient.WSClient
}

// New takes a remote endpoint in the form tcp://<host>:<port>
// and the websocket path (which always seems to be "/websocket")
func New(remote, wsEndpoint string) *Client {
	return &Client{
		rpc:      rpcclient.NewClientJSONRPC(remote),
		remote:   remote,
		endpoint: wsEndpoint,
	}
}

func (c *Client) _assertIsClient() client.Client {
	return c
}

func (c *Client) Status() (*ctypes.ResultStatus, error) {
	tmResult := new(ctypes.TMResult)
	_, err := c.rpc.Call("status", []interface{}{}, tmResult)
	if err != nil {
		return nil, errors.Wrap(err, "Status")
	}
	// note: panics if rpc doesn't match.  okay???
	return (*tmResult).(*ctypes.ResultStatus), nil
}

func (c *Client) ABCIInfo() (*ctypes.ResultABCIInfo, error) {
	tmResult := new(ctypes.TMResult)
	_, err := c.rpc.Call("abci_info", []interface{}{}, tmResult)
	if err != nil {
		return nil, errors.Wrap(err, "ABCIInfo")
	}
	return (*tmResult).(*ctypes.ResultABCIInfo), nil
}

func (c *Client) ABCIQuery(path string, data []byte, prove bool) (*ctypes.ResultABCIQuery, error) {
	tmResult := new(ctypes.TMResult)
	_, err := c.rpc.Call("abci_query", []interface{}{path, data, prove}, tmResult)
	if err != nil {
		return nil, errors.Wrap(err, "ABCIQuery")
	}
	return (*tmResult).(*ctypes.ResultABCIQuery), nil
}

func (c *Client) BroadcastTxCommit(tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	tmResult := new(ctypes.TMResult)
	_, err := c.rpc.Call("broadcast_tx_commit", []interface{}{tx}, tmResult)
	if err != nil {
		return nil, errors.Wrap(err, "broadcast_tx_commit")
	}
	return (*tmResult).(*ctypes.ResultBroadcastTxCommit), nil
}

func (c *Client) BroadcastTxAsync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return c.broadcastTX("broadcast_tx_async", tx)
}

func (c *Client) BroadcastTxSync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return c.broadcastTX("broadcast_tx_sync", tx)
}

func (c *Client) broadcastTX(route string, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	tmResult := new(ctypes.TMResult)
	_, err := c.rpc.Call(route, []interface{}{tx}, tmResult)
	if err != nil {
		return nil, errors.Wrap(err, route)
	}
	return (*tmResult).(*ctypes.ResultBroadcastTx), nil
}

func (c *Client) NetInfo() (*ctypes.ResultNetInfo, error) {
	tmResult := new(ctypes.TMResult)
	_, err := c.rpc.Call("net_info", nil, tmResult)
	if err != nil {
		return nil, errors.Wrap(err, "NetInfo")
	}
	return (*tmResult).(*ctypes.ResultNetInfo), nil
}

func (c *Client) DialSeeds(seeds []string) (*ctypes.ResultDialSeeds, error) {
	tmResult := new(ctypes.TMResult)
	// TODO: is this the correct way to marshall seeds?
	_, err := c.rpc.Call("dial_seeds", []interface{}{seeds}, tmResult)
	if err != nil {
		return nil, errors.Wrap(err, "DialSeeds")
	}
	return (*tmResult).(*ctypes.ResultDialSeeds), nil
}

func (c *Client) BlockchainInfo(minHeight, maxHeight int) (*ctypes.ResultBlockchainInfo, error) {
	tmResult := new(ctypes.TMResult)
	_, err := c.rpc.Call("blockchain", []interface{}{minHeight, maxHeight}, tmResult)
	if err != nil {
		return nil, errors.Wrap(err, "BlockchainInfo")
	}
	return (*tmResult).(*ctypes.ResultBlockchainInfo), nil
}

func (c *Client) Genesis() (*ctypes.ResultGenesis, error) {
	tmResult := new(ctypes.TMResult)
	_, err := c.rpc.Call("genesis", nil, tmResult)
	if err != nil {
		return nil, errors.Wrap(err, "Genesis")
	}
	return (*tmResult).(*ctypes.ResultGenesis), nil
}

func (c *Client) Block(height int) (*ctypes.ResultBlock, error) {
	tmResult := new(ctypes.TMResult)
	_, err := c.rpc.Call("block", []interface{}{height}, tmResult)
	if err != nil {
		return nil, errors.Wrap(err, "Block")
	}
	return (*tmResult).(*ctypes.ResultBlock), nil
}

func (c *Client) Commit(height int) (*ctypes.ResultCommit, error) {
	tmResult := new(ctypes.TMResult)
	_, err := c.rpc.Call("commit", []interface{}{height}, tmResult)
	if err != nil {
		return nil, errors.Wrap(err, "Commit")
	}
	return (*tmResult).(*ctypes.ResultCommit), nil
}

func (c *Client) Validators() (*ctypes.ResultValidators, error) {
	tmResult := new(ctypes.TMResult)
	_, err := c.rpc.Call("validators", nil, tmResult)
	if err != nil {
		return nil, errors.Wrap(err, "Validators")
	}
	return (*tmResult).(*ctypes.ResultValidators), nil
}

/** websocket event stuff here... **/

// StartWebsocket starts up a websocket and a listener goroutine
// if already started, do nothing
func (c *Client) StartWebsocket() error {
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
func (c *Client) StopWebsocket() {
	if c.ws != nil {
		c.ws.Stop()
		c.ws = nil
	}
}

// GetEventChannels returns the results and error channel from the websocket
func (c *Client) GetEventChannels() (chan json.RawMessage, chan error) {
	if c.ws == nil {
		return nil, nil
	}
	return c.ws.ResultsCh, c.ws.ErrorsCh
}

func (c *Client) Subscribe(event string) error {
	return errors.Wrap(c.ws.Subscribe(event), "Subscribe")
}

func (c *Client) Unsubscribe(event string) error {
	return errors.Wrap(c.ws.Unsubscribe(event), "Unsubscribe")
}
