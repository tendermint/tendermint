package http

import (
	"context"
	"net/http"
	"time"

	"github.com/tendermint/tendermint/libs/bytes"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/coretypes"
	jsonrpcclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
	"github.com/tendermint/tendermint/types"
)

/*
HTTP is a Client implementation that communicates with a Tendermint node over
JSON RPC and WebSockets.

This is the main implementation you probably want to use in production code.
There are other implementations when calling the Tendermint node in-process
(Local), or when you want to mock out the server for test code (mock).

You can subscribe for any event published by Tendermint using Subscribe method.
Note delivery is best-effort. If you don't read events fast enough or network is
slow, Tendermint might cancel the subscription. The client will attempt to
resubscribe (you don't need to do anything). It will keep trying every second
indefinitely until successful.

Request batching is available for JSON RPC requests over HTTP, which conforms to
the JSON RPC specification (https://www.jsonrpc.org/specification#batch). See
the example for more details.

Example:

		c, err := New("http://192.168.1.10:26657")
		if err != nil {
			// handle error
		}

		// call Start/Stop if you're subscribing to events
		err = c.Start()
		if err != nil {
			// handle error
		}
		defer c.Stop()

		res, err := c.Status()
		if err != nil {
			// handle error
		}

		// handle result
*/
type HTTP struct {
	remote string
	rpc    *jsonrpcclient.Client

	*baseRPCClient
	*wsEvents
}

// BatchHTTP provides the same interface as `HTTP`, but allows for batching of
// requests (as per https://www.jsonrpc.org/specification#batch). Do not
// instantiate directly - rather use the HTTP.NewBatch() method to create an
// instance of this struct.
//
// Batching of HTTP requests is thread-safe in the sense that multiple
// goroutines can each create their own batches and send them using the same
// HTTP client. Multiple goroutines could also enqueue transactions in a single
// batch, but ordering of transactions in the batch cannot be guaranteed in such
// an example.
type BatchHTTP struct {
	rpcBatch *jsonrpcclient.RequestBatch
	*baseRPCClient
}

// rpcClient is an internal interface to which our RPC clients (batch and
// non-batch) must conform. Acts as an additional code-level sanity check to
// make sure the implementations stay coherent.
type rpcClient interface {
	rpcclient.ABCIClient
	rpcclient.HistoryClient
	rpcclient.NetworkClient
	rpcclient.SignClient
	rpcclient.StatusClient
}

// baseRPCClient implements the basic RPC method logic without the actual
// underlying RPC call functionality, which is provided by `caller`.
type baseRPCClient struct {
	caller jsonrpcclient.Caller
}

var _ rpcClient = (*HTTP)(nil)
var _ rpcClient = (*BatchHTTP)(nil)
var _ rpcClient = (*baseRPCClient)(nil)

//-----------------------------------------------------------------------------
// HTTP

// New takes a remote endpoint in the form <protocol>://<host>:<port>. An error
// is returned on invalid remote.
func New(remote string) (*HTTP, error) {
	c, err := jsonrpcclient.DefaultHTTPClient(remote)
	if err != nil {
		return nil, err
	}
	return NewWithClient(remote, c)
}

// NewWithTimeout does the same thing as New, except you can set a Timeout for
// http.Client. A Timeout of zero means no timeout.
func NewWithTimeout(remote string, t time.Duration) (*HTTP, error) {
	c, err := jsonrpcclient.DefaultHTTPClient(remote)
	if err != nil {
		return nil, err
	}
	c.Timeout = t
	return NewWithClient(remote, c)
}

// NewWithClient allows you to set a custom http client. An error is returned
// on invalid remote. The function panics when client is nil.
func NewWithClient(remote string, c *http.Client) (*HTTP, error) {
	if c == nil {
		panic("nil http.Client")
	}
	return NewWithClientAndWSOptions(remote, c, DefaultWSOptions())
}

// NewWithClientAndWSOptions allows you to set a custom http client and
// WebSocket options. An error is returned on invalid remote. The function
// panics when client is nil.
func NewWithClientAndWSOptions(remote string, c *http.Client, wso WSOptions) (*HTTP, error) {
	if c == nil {
		panic("nil http.Client")
	}
	rpc, err := jsonrpcclient.NewWithHTTPClient(remote, c)
	if err != nil {
		return nil, err
	}

	wsEvents, err := newWsEvents(remote, wso)
	if err != nil {
		return nil, err
	}

	httpClient := &HTTP{
		rpc:           rpc,
		remote:        remote,
		baseRPCClient: &baseRPCClient{caller: rpc},
		wsEvents:      wsEvents,
	}

	return httpClient, nil
}

var _ rpcclient.Client = (*HTTP)(nil)

// Remote returns the remote network address in a string form.
func (c *HTTP) Remote() string {
	return c.remote
}

// NewBatch creates a new batch client for this HTTP client.
func (c *HTTP) NewBatch() *BatchHTTP {
	rpcBatch := c.rpc.NewRequestBatch()
	return &BatchHTTP{
		rpcBatch: rpcBatch,
		baseRPCClient: &baseRPCClient{
			caller: rpcBatch,
		},
	}
}

//-----------------------------------------------------------------------------
// BatchHTTP

// Send is a convenience function for an HTTP batch that will trigger the
// compilation of the batched requests and send them off using the client as a
// single request. On success, this returns a list of the deserialized results
// from each request in the sent batch.
func (b *BatchHTTP) Send(ctx context.Context) ([]interface{}, error) {
	return b.rpcBatch.Send(ctx)
}

// Clear will empty out this batch of requests and return the number of requests
// that were cleared out.
func (b *BatchHTTP) Clear() int {
	return b.rpcBatch.Clear()
}

// Count returns the number of enqueued requests waiting to be sent.
func (b *BatchHTTP) Count() int {
	return b.rpcBatch.Count()
}

//-----------------------------------------------------------------------------
// baseRPCClient

func (c *baseRPCClient) Status(ctx context.Context) (*coretypes.ResultStatus, error) {
	result := new(coretypes.ResultStatus)
	_, err := c.caller.Call(ctx, "status", map[string]interface{}{}, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *baseRPCClient) ABCIInfo(ctx context.Context) (*coretypes.ResultABCIInfo, error) {
	result := new(coretypes.ResultABCIInfo)
	_, err := c.caller.Call(ctx, "abci_info", map[string]interface{}{}, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *baseRPCClient) ABCIQuery(
	ctx context.Context,
	path string,
	data bytes.HexBytes,
) (*coretypes.ResultABCIQuery, error) {
	return c.ABCIQueryWithOptions(ctx, path, data, rpcclient.DefaultABCIQueryOptions)
}

func (c *baseRPCClient) ABCIQueryWithOptions(
	ctx context.Context,
	path string,
	data bytes.HexBytes,
	opts rpcclient.ABCIQueryOptions) (*coretypes.ResultABCIQuery, error) {
	result := new(coretypes.ResultABCIQuery)
	_, err := c.caller.Call(ctx, "abci_query",
		map[string]interface{}{"path": path, "data": data, "height": opts.Height, "prove": opts.Prove},
		result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *baseRPCClient) BroadcastTxCommit(
	ctx context.Context,
	tx types.Tx,
) (*coretypes.ResultBroadcastTxCommit, error) {
	result := new(coretypes.ResultBroadcastTxCommit)
	_, err := c.caller.Call(ctx, "broadcast_tx_commit", map[string]interface{}{"tx": tx}, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) BroadcastTxAsync(
	ctx context.Context,
	tx types.Tx,
) (*coretypes.ResultBroadcastTx, error) {
	return c.broadcastTX(ctx, "broadcast_tx_async", tx)
}

func (c *baseRPCClient) BroadcastTxSync(
	ctx context.Context,
	tx types.Tx,
) (*coretypes.ResultBroadcastTx, error) {
	return c.broadcastTX(ctx, "broadcast_tx_sync", tx)
}

func (c *baseRPCClient) broadcastTX(
	ctx context.Context,
	route string,
	tx types.Tx,
) (*coretypes.ResultBroadcastTx, error) {
	result := new(coretypes.ResultBroadcastTx)
	_, err := c.caller.Call(ctx, route, map[string]interface{}{"tx": tx}, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) UnconfirmedTxs(
	ctx context.Context,
	limit *int,
) (*coretypes.ResultUnconfirmedTxs, error) {
	result := new(coretypes.ResultUnconfirmedTxs)
	params := make(map[string]interface{})
	if limit != nil {
		params["limit"] = limit
	}
	_, err := c.caller.Call(ctx, "unconfirmed_txs", params, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) NumUnconfirmedTxs(ctx context.Context) (*coretypes.ResultUnconfirmedTxs, error) {
	result := new(coretypes.ResultUnconfirmedTxs)
	_, err := c.caller.Call(ctx, "num_unconfirmed_txs", map[string]interface{}{}, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) CheckTx(ctx context.Context, tx types.Tx) (*coretypes.ResultCheckTx, error) {
	result := new(coretypes.ResultCheckTx)
	_, err := c.caller.Call(ctx, "check_tx", map[string]interface{}{"tx": tx}, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) RemoveTx(ctx context.Context, txKey types.TxKey) error {
	_, err := c.caller.Call(ctx, "remove_tx", map[string]interface{}{"tx_key": txKey}, nil)
	if err != nil {
		return err
	}
	return nil
}

func (c *baseRPCClient) NetInfo(ctx context.Context) (*coretypes.ResultNetInfo, error) {
	result := new(coretypes.ResultNetInfo)
	_, err := c.caller.Call(ctx, "net_info", map[string]interface{}{}, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) DumpConsensusState(ctx context.Context) (*coretypes.ResultDumpConsensusState, error) {
	result := new(coretypes.ResultDumpConsensusState)
	_, err := c.caller.Call(ctx, "dump_consensus_state", map[string]interface{}{}, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) ConsensusState(ctx context.Context) (*coretypes.ResultConsensusState, error) {
	result := new(coretypes.ResultConsensusState)
	_, err := c.caller.Call(ctx, "consensus_state", map[string]interface{}{}, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) ConsensusParams(
	ctx context.Context,
	height *int64,
) (*coretypes.ResultConsensusParams, error) {
	result := new(coretypes.ResultConsensusParams)
	params := make(map[string]interface{})
	if height != nil {
		params["height"] = height
	}
	_, err := c.caller.Call(ctx, "consensus_params", params, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) Health(ctx context.Context) (*coretypes.ResultHealth, error) {
	result := new(coretypes.ResultHealth)
	_, err := c.caller.Call(ctx, "health", map[string]interface{}{}, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) BlockchainInfo(
	ctx context.Context,
	minHeight,
	maxHeight int64,
) (*coretypes.ResultBlockchainInfo, error) {
	result := new(coretypes.ResultBlockchainInfo)
	_, err := c.caller.Call(ctx, "blockchain",
		map[string]interface{}{"minHeight": minHeight, "maxHeight": maxHeight},
		result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) Genesis(ctx context.Context) (*coretypes.ResultGenesis, error) {
	result := new(coretypes.ResultGenesis)
	_, err := c.caller.Call(ctx, "genesis", map[string]interface{}{}, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) GenesisChunked(ctx context.Context, id uint) (*coretypes.ResultGenesisChunk, error) {
	result := new(coretypes.ResultGenesisChunk)
	_, err := c.caller.Call(ctx, "genesis_chunked", map[string]interface{}{"chunk": id}, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) Block(ctx context.Context, height *int64) (*coretypes.ResultBlock, error) {
	result := new(coretypes.ResultBlock)
	params := make(map[string]interface{})
	if height != nil {
		params["height"] = height
	}
	_, err := c.caller.Call(ctx, "block", params, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) BlockByHash(ctx context.Context, hash bytes.HexBytes) (*coretypes.ResultBlock, error) {
	result := new(coretypes.ResultBlock)
	params := map[string]interface{}{
		"hash": hash,
	}
	_, err := c.caller.Call(ctx, "block_by_hash", params, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) BlockResults(
	ctx context.Context,
	height *int64,
) (*coretypes.ResultBlockResults, error) {
	result := new(coretypes.ResultBlockResults)
	params := make(map[string]interface{})
	if height != nil {
		params["height"] = height
	}
	_, err := c.caller.Call(ctx, "block_results", params, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) Commit(ctx context.Context, height *int64) (*coretypes.ResultCommit, error) {
	result := new(coretypes.ResultCommit)
	params := make(map[string]interface{})
	if height != nil {
		params["height"] = height
	}
	_, err := c.caller.Call(ctx, "commit", params, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) Tx(ctx context.Context, hash bytes.HexBytes, prove bool) (*coretypes.ResultTx, error) {
	result := new(coretypes.ResultTx)
	params := map[string]interface{}{
		"hash":  hash,
		"prove": prove,
	}
	_, err := c.caller.Call(ctx, "tx", params, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) TxSearch(
	ctx context.Context,
	query string,
	prove bool,
	page,
	perPage *int,
	orderBy string,
) (*coretypes.ResultTxSearch, error) {

	result := new(coretypes.ResultTxSearch)
	params := map[string]interface{}{
		"query":    query,
		"prove":    prove,
		"order_by": orderBy,
	}

	if page != nil {
		params["page"] = page
	}
	if perPage != nil {
		params["per_page"] = perPage
	}

	_, err := c.caller.Call(ctx, "tx_search", params, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *baseRPCClient) BlockSearch(
	ctx context.Context,
	query string,
	page, perPage *int,
	orderBy string,
) (*coretypes.ResultBlockSearch, error) {

	result := new(coretypes.ResultBlockSearch)
	params := map[string]interface{}{
		"query":    query,
		"order_by": orderBy,
	}

	if page != nil {
		params["page"] = page
	}
	if perPage != nil {
		params["per_page"] = perPage
	}

	_, err := c.caller.Call(ctx, "block_search", params, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *baseRPCClient) Validators(
	ctx context.Context,
	height *int64,
	page,
	perPage *int,
) (*coretypes.ResultValidators, error) {
	result := new(coretypes.ResultValidators)
	params := make(map[string]interface{})
	if page != nil {
		params["page"] = page
	}
	if perPage != nil {
		params["per_page"] = perPage
	}
	if height != nil {
		params["height"] = height
	}
	_, err := c.caller.Call(ctx, "validators", params, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) BroadcastEvidence(
	ctx context.Context,
	ev types.Evidence,
) (*coretypes.ResultBroadcastEvidence, error) {
	result := new(coretypes.ResultBroadcastEvidence)
	_, err := c.caller.Call(ctx, "broadcast_evidence", map[string]interface{}{"evidence": ev}, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}
