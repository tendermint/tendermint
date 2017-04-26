/*
package mock returns a Client implementation that
accepts various (mock) implementations of the various methods.

This implementation is useful for using in tests, when you don't
need a real server, but want a high-level of control about
the server response you want to mock (eg. error handling),
or if you just want to record the calls to verify in your tests.

For real clients, you probably want the "http" package.  If you
want to directly call a tendermint node in process, you can use the
"local" package.
*/
package mock

import (
	"reflect"

	data "github.com/tendermint/go-wire/data"
	"github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/core"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

// Client wraps arbitrary implementations of the various interfaces.
//
// We provide a few choices to mock out each one in this package.
// Nothing hidden here, so no New function, just construct it from
// some parts, and swap them out them during the tests.
type Client struct {
	client.ABCIClient
	client.SignClient
	client.HistoryClient
	client.StatusClient
	// create a mock with types.NewEventSwitch()
	types.EventSwitch
}

func (c Client) _assertIsClient() client.Client {
	return c
}

// Call is used by recorders to save a call and response.
// It can also be used to configure mock responses.
//
type Call struct {
	Name     string
	Args     interface{}
	Response interface{}
	Error    error
}

// GetResponse will generate the apporiate response for us, when
// using the Call struct to configure a Mock handler.
//
// When configuring a response, if only one of Response or Error is
// set then that will always be returned. If both are set, then
// we return Response if the Args match the set args, Error otherwise.
func (c Call) GetResponse(args interface{}) (interface{}, error) {
	// handle the case with no response
	if c.Response == nil {
		if c.Error == nil {
			panic("Misconfigured call, you must set either Response or Error")
		}
		return nil, c.Error
	}
	// response without error
	if c.Error == nil {
		return c.Response, nil
	}
	// have both, we must check args....
	if reflect.DeepEqual(args, c.Args) {
		return c.Response, nil
	}
	return nil, c.Error
}

func (c Client) Status() (*ctypes.ResultStatus, error) {
	return core.Status()
}

func (c Client) ABCIInfo() (*ctypes.ResultABCIInfo, error) {
	return core.ABCIInfo()
}

func (c Client) ABCIQuery(path string, data data.Bytes, prove bool) (*ctypes.ResultABCIQuery, error) {
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
