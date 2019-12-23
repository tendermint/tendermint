package mock

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

import (
	"reflect"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/core"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
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
	client.EventsClient
	client.EvidenceClient
	client.MempoolClient
	cmn.Service
}

var _ client.Client = Client{}

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
	return core.Status(&rpctypes.Context{})
}

func (c Client) ABCIInfo() (*ctypes.ResultABCIInfo, error) {
	return core.ABCIInfo(&rpctypes.Context{})
}

func (c Client) ABCIQuery(path string, data cmn.HexBytes) (*ctypes.ResultABCIQuery, error) {
	return c.ABCIQueryWithOptions(path, data, client.DefaultABCIQueryOptions)
}

func (c Client) ABCIQueryWithOptions(path string, data cmn.HexBytes, opts client.ABCIQueryOptions) (*ctypes.ResultABCIQuery, error) {
	return core.ABCIQuery(&rpctypes.Context{}, path, data, opts.Height, opts.Prove)
}

func (c Client) BroadcastTxCommit(tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	return core.BroadcastTxCommit(&rpctypes.Context{}, tx)
}

func (c Client) BroadcastTxAsync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return core.BroadcastTxAsync(&rpctypes.Context{}, tx)
}

func (c Client) BroadcastTxSync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return core.BroadcastTxSync(&rpctypes.Context{}, tx)
}

func (c Client) NetInfo() (*ctypes.ResultNetInfo, error) {
	return core.NetInfo(&rpctypes.Context{})
}

func (c Client) ConsensusState() (*ctypes.ResultConsensusState, error) {
	return core.ConsensusState(&rpctypes.Context{})
}

func (c Client) DumpConsensusState() (*ctypes.ResultDumpConsensusState, error) {
	return core.DumpConsensusState(&rpctypes.Context{})
}

func (c Client) Health() (*ctypes.ResultHealth, error) {
	return core.Health(&rpctypes.Context{})
}

func (c Client) DialSeeds(seeds []string) (*ctypes.ResultDialSeeds, error) {
	return core.UnsafeDialSeeds(&rpctypes.Context{}, seeds)
}

func (c Client) DialPeers(peers []string, persistent bool) (*ctypes.ResultDialPeers, error) {
	return core.UnsafeDialPeers(&rpctypes.Context{}, peers, persistent)
}

func (c Client) BlockchainInfo(minHeight, maxHeight int64) (*ctypes.ResultBlockchainInfo, error) {
	return core.BlockchainInfo(&rpctypes.Context{}, minHeight, maxHeight)
}

func (c Client) Genesis() (*ctypes.ResultGenesis, error) {
	return core.Genesis(&rpctypes.Context{})
}

func (c Client) Block(height *int64) (*ctypes.ResultBlock, error) {
	return core.Block(&rpctypes.Context{}, height)
}

func (c Client) Commit(height *int64) (*ctypes.ResultCommit, error) {
	return core.Commit(&rpctypes.Context{}, height)
}

func (c Client) Validators(height *int64) (*ctypes.ResultValidators, error) {
	return core.Validators(&rpctypes.Context{}, height)
}

func (c Client) BroadcastEvidence(ev types.Evidence) (*ctypes.ResultBroadcastEvidence, error) {
	return core.BroadcastEvidence(&rpctypes.Context{}, ev)
}
