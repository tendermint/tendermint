package http

import (
	"context"
	"errors"
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

var (
	_ rpcClient = (*HTTP)(nil)
	_ rpcClient = (*BatchHTTP)(nil)
	_ rpcClient = (*baseRPCClient)(nil)
)

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

// NewWithClient constructs an RPC client using a custom HTTP client.
// An error is reported if c == nil or remote is an invalid address.
func NewWithClient(remote string, c *http.Client) (*HTTP, error) {
	if c == nil {
		return nil, errors.New("nil client")
	}
	rpc, err := jsonrpcclient.NewWithHTTPClient(remote, c)
	if err != nil {
		return nil, err
	}

	wsEvents, err := newWsEvents(remote)
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
	if err := c.caller.Call(ctx, "status", nil, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) ABCIInfo(ctx context.Context) (*coretypes.ResultABCIInfo, error) {
	result := new(coretypes.ResultABCIInfo)
	if err := c.caller.Call(ctx, "abci_info", nil, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) ABCIQuery(ctx context.Context, path string, data bytes.HexBytes) (*coretypes.ResultABCIQuery, error) {
	return c.ABCIQueryWithOptions(ctx, path, data, rpcclient.DefaultABCIQueryOptions)
}

func (c *baseRPCClient) ABCIQueryWithOptions(ctx context.Context, path string, data bytes.HexBytes, opts rpcclient.ABCIQueryOptions) (*coretypes.ResultABCIQuery, error) {
	result := new(coretypes.ResultABCIQuery)
	if err := c.caller.Call(ctx, "abci_query", &coretypes.RequestABCIQuery{
		Path:   path,
		Data:   data,
		Height: coretypes.Int64(opts.Height),
		Prove:  opts.Prove,
	}, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) BroadcastTxCommit(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTxCommit, error) {
	result := new(coretypes.ResultBroadcastTxCommit)
	if err := c.caller.Call(ctx, "broadcast_tx_commit", &coretypes.RequestBroadcastTx{
		Tx: tx,
	}, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) BroadcastTxAsync(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	return c.broadcastTX(ctx, "broadcast_tx_async", tx)
}

func (c *baseRPCClient) BroadcastTxSync(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	return c.broadcastTX(ctx, "broadcast_tx_sync", tx)
}

func (c *baseRPCClient) BroadcastTx(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	return c.broadcastTX(ctx, "broadcast_tx_sync", tx)
}

func (c *baseRPCClient) broadcastTX(ctx context.Context, route string, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	result := new(coretypes.ResultBroadcastTx)
	if err := c.caller.Call(ctx, route, &coretypes.RequestBroadcastTx{Tx: tx}, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) UnconfirmedTxs(ctx context.Context, page *int, perPage *int) (*coretypes.ResultUnconfirmedTxs, error) {
	result := new(coretypes.ResultUnconfirmedTxs)

	if err := c.caller.Call(ctx, "unconfirmed_txs", &coretypes.RequestUnconfirmedTxs{
		Page:    coretypes.Int64Ptr(page),
		PerPage: coretypes.Int64Ptr(perPage),
	}, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) NumUnconfirmedTxs(ctx context.Context) (*coretypes.ResultUnconfirmedTxs, error) {
	result := new(coretypes.ResultUnconfirmedTxs)
	if err := c.caller.Call(ctx, "num_unconfirmed_txs", nil, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) CheckTx(ctx context.Context, tx types.Tx) (*coretypes.ResultCheckTx, error) {
	result := new(coretypes.ResultCheckTx)
	if err := c.caller.Call(ctx, "check_tx", &coretypes.RequestCheckTx{Tx: tx}, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) RemoveTx(ctx context.Context, txKey types.TxKey) error {
	if err := c.caller.Call(ctx, "remove_tx", &coretypes.RequestRemoveTx{TxKey: txKey}, nil); err != nil {
		return err
	}
	return nil
}

func (c *baseRPCClient) NetInfo(ctx context.Context) (*coretypes.ResultNetInfo, error) {
	result := new(coretypes.ResultNetInfo)
	if err := c.caller.Call(ctx, "net_info", nil, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) DumpConsensusState(ctx context.Context) (*coretypes.ResultDumpConsensusState, error) {
	result := new(coretypes.ResultDumpConsensusState)
	if err := c.caller.Call(ctx, "dump_consensus_state", nil, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) ConsensusState(ctx context.Context) (*coretypes.ResultConsensusState, error) {
	result := new(coretypes.ResultConsensusState)
	if err := c.caller.Call(ctx, "consensus_state", nil, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) ConsensusParams(ctx context.Context, height *int64) (*coretypes.ResultConsensusParams, error) {
	result := new(coretypes.ResultConsensusParams)
	if err := c.caller.Call(ctx, "consensus_params", &coretypes.RequestConsensusParams{
		Height: (*coretypes.Int64)(height),
	}, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) Events(ctx context.Context, req *coretypes.RequestEvents) (*coretypes.ResultEvents, error) {
	result := new(coretypes.ResultEvents)
	if err := c.caller.Call(ctx, "events", req, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) Health(ctx context.Context) (*coretypes.ResultHealth, error) {
	result := new(coretypes.ResultHealth)
	if err := c.caller.Call(ctx, "health", nil, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) BlockchainInfo(ctx context.Context, minHeight, maxHeight int64) (*coretypes.ResultBlockchainInfo, error) {
	result := new(coretypes.ResultBlockchainInfo)
	if err := c.caller.Call(ctx, "blockchain", &coretypes.RequestBlockchainInfo{
		MinHeight: coretypes.Int64(minHeight),
		MaxHeight: coretypes.Int64(maxHeight),
	}, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) Genesis(ctx context.Context) (*coretypes.ResultGenesis, error) {
	result := new(coretypes.ResultGenesis)
	if err := c.caller.Call(ctx, "genesis", nil, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) GenesisChunked(ctx context.Context, id uint) (*coretypes.ResultGenesisChunk, error) {
	result := new(coretypes.ResultGenesisChunk)
	if err := c.caller.Call(ctx, "genesis_chunked", &coretypes.RequestGenesisChunked{
		Chunk: coretypes.Int64(id),
	}, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) Block(ctx context.Context, height *int64) (*coretypes.ResultBlock, error) {
	result := new(coretypes.ResultBlock)
	if err := c.caller.Call(ctx, "block", &coretypes.RequestBlockInfo{
		Height: (*coretypes.Int64)(height),
	}, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) BlockByHash(ctx context.Context, hash bytes.HexBytes) (*coretypes.ResultBlock, error) {
	result := new(coretypes.ResultBlock)
	if err := c.caller.Call(ctx, "block_by_hash", &coretypes.RequestBlockByHash{Hash: hash}, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) BlockResults(ctx context.Context, height *int64) (*coretypes.ResultBlockResults, error) {
	result := new(coretypes.ResultBlockResults)
	if err := c.caller.Call(ctx, "block_results", &coretypes.RequestBlockInfo{
		Height: (*coretypes.Int64)(height),
	}, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) Header(ctx context.Context, height *int64) (*coretypes.ResultHeader, error) {
	result := new(coretypes.ResultHeader)
	if err := c.caller.Call(ctx, "header", &coretypes.RequestBlockInfo{
		Height: (*coretypes.Int64)(height),
	}, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) HeaderByHash(ctx context.Context, hash bytes.HexBytes) (*coretypes.ResultHeader, error) {
	result := new(coretypes.ResultHeader)
	if err := c.caller.Call(ctx, "header_by_hash", &coretypes.RequestBlockByHash{
		Hash: hash,
	}, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) Commit(ctx context.Context, height *int64) (*coretypes.ResultCommit, error) {
	result := new(coretypes.ResultCommit)
	if err := c.caller.Call(ctx, "commit", &coretypes.RequestBlockInfo{
		Height: (*coretypes.Int64)(height),
	}, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) Tx(ctx context.Context, hash bytes.HexBytes, prove bool) (*coretypes.ResultTx, error) {
	result := new(coretypes.ResultTx)
	if err := c.caller.Call(ctx, "tx", &coretypes.RequestTx{Hash: hash, Prove: prove}, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) TxSearch(ctx context.Context, query string, prove bool, page, perPage *int, orderBy string) (*coretypes.ResultTxSearch, error) {
	result := new(coretypes.ResultTxSearch)
	if err := c.caller.Call(ctx, "tx_search", &coretypes.RequestTxSearch{
		Query:   query,
		Prove:   prove,
		OrderBy: orderBy,
		Page:    coretypes.Int64Ptr(page),
		PerPage: coretypes.Int64Ptr(perPage),
	}, result); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *baseRPCClient) BlockSearch(ctx context.Context, query string, page, perPage *int, orderBy string) (*coretypes.ResultBlockSearch, error) {
	result := new(coretypes.ResultBlockSearch)
	if err := c.caller.Call(ctx, "block_search", &coretypes.RequestBlockSearch{
		Query:   query,
		OrderBy: orderBy,
		Page:    coretypes.Int64Ptr(page),
		PerPage: coretypes.Int64Ptr(perPage),
	}, result); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *baseRPCClient) Validators(ctx context.Context, height *int64, page, perPage *int) (*coretypes.ResultValidators, error) {
	result := new(coretypes.ResultValidators)
	if err := c.caller.Call(ctx, "validators", &coretypes.RequestValidators{
		Height:  (*coretypes.Int64)(height),
		Page:    coretypes.Int64Ptr(page),
		PerPage: coretypes.Int64Ptr(perPage),
	}, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) BroadcastEvidence(ctx context.Context, ev types.Evidence) (*coretypes.ResultBroadcastEvidence, error) {
	result := new(coretypes.ResultBroadcastEvidence)
	if err := c.caller.Call(ctx, "broadcast_evidence", &coretypes.RequestBroadcastEvidence{
		Evidence: ev,
	}, result); err != nil {
		return nil, err
	}
	return result, nil
}
