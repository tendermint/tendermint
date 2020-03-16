package client

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	amino "github.com/tendermint/go-amino"

	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	"github.com/tendermint/tendermint/libs/service"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpcclient "github.com/tendermint/tendermint/rpc/lib/client"
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

		c, err := NewHTTP("http://192.168.1.10:26657", "/websocket")
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
	rpc    *rpcclient.JSONRPCClient

	*baseRPCClient
	*WSEvents
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
	rpcBatch *rpcclient.JSONRPCRequestBatch
	*baseRPCClient
}

// rpcClient is an internal interface to which our RPC clients (batch and
// non-batch) must conform. Acts as an additional code-level sanity check to
// make sure the implementations stay coherent.
type rpcClient interface {
	ABCIClient
	HistoryClient
	NetworkClient
	SignClient
	StatusClient
}

// baseRPCClient implements the basic RPC method logic without the actual
// underlying RPC call functionality, which is provided by `caller`.
type baseRPCClient struct {
	caller rpcclient.JSONRPCCaller
}

var _ rpcClient = (*HTTP)(nil)
var _ rpcClient = (*BatchHTTP)(nil)
var _ rpcClient = (*baseRPCClient)(nil)

//-----------------------------------------------------------------------------
// HTTP

// NewHTTP takes a remote endpoint in the form <protocol>://<host>:<port> and
// the websocket path (which always seems to be "/websocket")
// An error is returned on invalid remote. The function panics when remote is nil.
func NewHTTP(remote, wsEndpoint string) (*HTTP, error) {
	httpClient, err := rpcclient.DefaultHTTPClient(remote)
	if err != nil {
		return nil, err
	}
	return NewHTTPWithClient(remote, wsEndpoint, httpClient)
}

// Create timeout enabled http client
func NewHTTPWithTimeout(remote, wsEndpoint string, timeout uint) (*HTTP, error) {
	httpClient, err := rpcclient.DefaultHTTPClient(remote)
	if err != nil {
		return nil, err
	}
	httpClient.Timeout = time.Duration(timeout) * time.Second
	return NewHTTPWithClient(remote, wsEndpoint, httpClient)
}

// NewHTTPWithClient allows for setting a custom http client (See NewHTTP).
// An error is returned on invalid remote. The function panics when remote is nil.
func NewHTTPWithClient(remote, wsEndpoint string, client *http.Client) (*HTTP, error) {
	if client == nil {
		panic("nil http.Client provided")
	}

	rc, err := rpcclient.NewJSONRPCClientWithHTTPClient(remote, client)
	if err != nil {
		return nil, err
	}
	cdc := rc.Codec()
	ctypes.RegisterAmino(cdc)
	rc.SetCodec(cdc)

	wsEvents, err := newWSEvents(cdc, remote, wsEndpoint)
	if err != nil {
		return nil, err
	}

	httpClient := &HTTP{
		rpc:           rc,
		remote:        remote,
		baseRPCClient: &baseRPCClient{caller: rc},
		WSEvents:      wsEvents,
	}

	return httpClient, nil
}

var _ Client = (*HTTP)(nil)

// SetLogger sets a logger.
func (c *HTTP) SetLogger(l log.Logger) {
	c.WSEvents.SetLogger(l)
}

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
func (b *BatchHTTP) Send() ([]interface{}, error) {
	return b.rpcBatch.Send()
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

func (c *baseRPCClient) Status() (*ctypes.ResultStatus, error) {
	result := new(ctypes.ResultStatus)
	_, err := c.caller.Call("status", map[string]interface{}{}, result)
	if err != nil {
		return nil, errors.Wrap(err, "Status")
	}
	return result, nil
}

func (c *baseRPCClient) ABCIInfo() (*ctypes.ResultABCIInfo, error) {
	result := new(ctypes.ResultABCIInfo)
	_, err := c.caller.Call("abci_info", map[string]interface{}{}, result)
	if err != nil {
		return nil, errors.Wrap(err, "ABCIInfo")
	}
	return result, nil
}

func (c *baseRPCClient) ABCIQuery(path string, data bytes.HexBytes) (*ctypes.ResultABCIQuery, error) {
	return c.ABCIQueryWithOptions(path, data, DefaultABCIQueryOptions)
}

func (c *baseRPCClient) ABCIQueryWithOptions(
	path string,
	data bytes.HexBytes,
	opts ABCIQueryOptions) (*ctypes.ResultABCIQuery, error) {
	result := new(ctypes.ResultABCIQuery)
	_, err := c.caller.Call("abci_query",
		map[string]interface{}{"path": path, "data": data, "height": opts.Height, "prove": opts.Prove},
		result)
	if err != nil {
		return nil, errors.Wrap(err, "ABCIQuery")
	}
	return result, nil
}

func (c *baseRPCClient) BroadcastTxCommit(tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	result := new(ctypes.ResultBroadcastTxCommit)
	_, err := c.caller.Call("broadcast_tx_commit", map[string]interface{}{"tx": tx}, result)
	if err != nil {
		return nil, errors.Wrap(err, "broadcast_tx_commit")
	}
	return result, nil
}

func (c *baseRPCClient) BroadcastTxAsync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return c.broadcastTX("broadcast_tx_async", tx)
}

func (c *baseRPCClient) BroadcastTxSync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return c.broadcastTX("broadcast_tx_sync", tx)
}

func (c *baseRPCClient) broadcastTX(route string, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	result := new(ctypes.ResultBroadcastTx)
	_, err := c.caller.Call(route, map[string]interface{}{"tx": tx}, result)
	if err != nil {
		return nil, errors.Wrap(err, route)
	}
	return result, nil
}

func (c *baseRPCClient) UnconfirmedTxs(limit int) (*ctypes.ResultUnconfirmedTxs, error) {
	result := new(ctypes.ResultUnconfirmedTxs)
	_, err := c.caller.Call("unconfirmed_txs", map[string]interface{}{"limit": limit}, result)
	if err != nil {
		return nil, errors.Wrap(err, "unconfirmed_txs")
	}
	return result, nil
}

func (c *baseRPCClient) NumUnconfirmedTxs() (*ctypes.ResultUnconfirmedTxs, error) {
	result := new(ctypes.ResultUnconfirmedTxs)
	_, err := c.caller.Call("num_unconfirmed_txs", map[string]interface{}{}, result)
	if err != nil {
		return nil, errors.Wrap(err, "num_unconfirmed_txs")
	}
	return result, nil
}

func (c *baseRPCClient) NetInfo() (*ctypes.ResultNetInfo, error) {
	result := new(ctypes.ResultNetInfo)
	_, err := c.caller.Call("net_info", map[string]interface{}{}, result)
	if err != nil {
		return nil, errors.Wrap(err, "NetInfo")
	}
	return result, nil
}

func (c *baseRPCClient) DumpConsensusState() (*ctypes.ResultDumpConsensusState, error) {
	result := new(ctypes.ResultDumpConsensusState)
	_, err := c.caller.Call("dump_consensus_state", map[string]interface{}{}, result)
	if err != nil {
		return nil, errors.Wrap(err, "DumpConsensusState")
	}
	return result, nil
}

func (c *baseRPCClient) ConsensusState() (*ctypes.ResultConsensusState, error) {
	result := new(ctypes.ResultConsensusState)
	_, err := c.caller.Call("consensus_state", map[string]interface{}{}, result)
	if err != nil {
		return nil, errors.Wrap(err, "ConsensusState")
	}
	return result, nil
}

func (c *baseRPCClient) ConsensusParams(height *int64) (*ctypes.ResultConsensusParams, error) {
	result := new(ctypes.ResultConsensusParams)
	_, err := c.caller.Call("consensus_params", map[string]interface{}{"height": height}, result)
	if err != nil {
		return nil, errors.Wrap(err, "ConsensusParams")
	}
	return result, nil
}

func (c *baseRPCClient) Health() (*ctypes.ResultHealth, error) {
	result := new(ctypes.ResultHealth)
	_, err := c.caller.Call("health", map[string]interface{}{}, result)
	if err != nil {
		return nil, errors.Wrap(err, "Health")
	}
	return result, nil
}

func (c *baseRPCClient) BlockchainInfo(minHeight, maxHeight int64) (*ctypes.ResultBlockchainInfo, error) {
	result := new(ctypes.ResultBlockchainInfo)
	_, err := c.caller.Call("blockchain",
		map[string]interface{}{"minHeight": minHeight, "maxHeight": maxHeight},
		result)
	if err != nil {
		return nil, errors.Wrap(err, "BlockchainInfo")
	}
	return result, nil
}

func (c *baseRPCClient) Genesis() (*ctypes.ResultGenesis, error) {
	result := new(ctypes.ResultGenesis)
	_, err := c.caller.Call("genesis", map[string]interface{}{}, result)
	if err != nil {
		return nil, errors.Wrap(err, "Genesis")
	}
	return result, nil
}

func (c *baseRPCClient) Block(height *int64) (*ctypes.ResultBlock, error) {
	result := new(ctypes.ResultBlock)
	_, err := c.caller.Call("block", map[string]interface{}{"height": height}, result)
	if err != nil {
		return nil, errors.Wrap(err, "Block")
	}
	return result, nil
}

func (c *baseRPCClient) BlockResults(height *int64) (*ctypes.ResultBlockResults, error) {
	result := new(ctypes.ResultBlockResults)
	_, err := c.caller.Call("block_results", map[string]interface{}{"height": height}, result)
	if err != nil {
		return nil, errors.Wrap(err, "Block Result")
	}
	return result, nil
}

func (c *baseRPCClient) Commit(height *int64) (*ctypes.ResultCommit, error) {
	result := new(ctypes.ResultCommit)
	_, err := c.caller.Call("commit", map[string]interface{}{"height": height}, result)
	if err != nil {
		return nil, errors.Wrap(err, "Commit")
	}
	return result, nil
}

func (c *baseRPCClient) Tx(hash []byte, prove bool) (*ctypes.ResultTx, error) {
	result := new(ctypes.ResultTx)
	params := map[string]interface{}{
		"hash":  hash,
		"prove": prove,
	}
	_, err := c.caller.Call("tx", params, result)
	if err != nil {
		return nil, errors.Wrap(err, "Tx")
	}
	return result, nil
}

func (c *baseRPCClient) TxSearch(query string, prove bool, page, perPage int, orderBy string) (
	*ctypes.ResultTxSearch, error) {
	result := new(ctypes.ResultTxSearch)
	params := map[string]interface{}{
		"query":    query,
		"prove":    prove,
		"page":     page,
		"per_page": perPage,
		"order_by": orderBy,
	}
	_, err := c.caller.Call("tx_search", params, result)
	if err != nil {
		return nil, errors.Wrap(err, "TxSearch")
	}
	return result, nil
}

func (c *baseRPCClient) Validators(height *int64, page, perPage int) (*ctypes.ResultValidators, error) {
	result := new(ctypes.ResultValidators)
	_, err := c.caller.Call("validators", map[string]interface{}{
		"height":   height,
		"page":     page,
		"per_page": perPage,
	}, result)
	if err != nil {
		return nil, errors.Wrap(err, "Validators")
	}
	return result, nil
}

func (c *baseRPCClient) BroadcastEvidence(ev types.Evidence) (*ctypes.ResultBroadcastEvidence, error) {
	result := new(ctypes.ResultBroadcastEvidence)
	_, err := c.caller.Call("broadcast_evidence", map[string]interface{}{"evidence": ev}, result)
	if err != nil {
		return nil, errors.Wrap(err, "BroadcastEvidence")
	}
	return result, nil
}

//-----------------------------------------------------------------------------
// WSEvents

var errNotRunning = errors.New("client is not running. Use .Start() method to start")

// WSEvents is a wrapper around WSClient, which implements EventsClient.
type WSEvents struct {
	service.BaseService
	cdc      *amino.Codec
	remote   string
	endpoint string
	ws       *rpcclient.WSClient

	mtx           sync.RWMutex
	subscriptions map[string]chan ctypes.ResultEvent // query -> chan
}

func newWSEvents(cdc *amino.Codec, remote, endpoint string) (*WSEvents, error) {
	w := &WSEvents{
		cdc:           cdc,
		endpoint:      endpoint,
		remote:        remote,
		subscriptions: make(map[string]chan ctypes.ResultEvent),
	}
	w.BaseService = *service.NewBaseService(nil, "WSEvents", w)

	var err error
	w.ws, err = rpcclient.NewWSClient(w.remote, w.endpoint, rpcclient.OnReconnect(func() {
		// resubscribe immediately
		w.redoSubscriptionsAfter(0 * time.Second)
	}))
	if err != nil {
		return nil, err
	}
	w.ws.SetCodec(w.cdc)
	w.ws.SetLogger(w.Logger)

	return w, nil
}

// OnStart implements service.Service by starting WSClient and event loop.
func (w *WSEvents) OnStart() error {
	if err := w.ws.Start(); err != nil {
		return err
	}

	go w.eventListener()

	return nil
}

// OnStop implements service.Service by stopping WSClient.
func (w *WSEvents) OnStop() {
	_ = w.ws.Stop()
}

// Subscribe implements EventsClient by using WSClient to subscribe given
// subscriber to query. By default, returns a channel with cap=1. Error is
// returned if it fails to subscribe.
//
// Channel is never closed to prevent clients from seeing an erroneous event.
//
// It returns an error if WSEvents is not running.
func (w *WSEvents) Subscribe(ctx context.Context, subscriber, query string,
	outCapacity ...int) (out <-chan ctypes.ResultEvent, err error) {

	if !w.IsRunning() {
		return nil, errNotRunning
	}

	if err := w.ws.Subscribe(ctx, query); err != nil {
		return nil, err
	}

	outCap := 1
	if len(outCapacity) > 0 {
		outCap = outCapacity[0]
	}

	outc := make(chan ctypes.ResultEvent, outCap)
	w.mtx.Lock()
	// subscriber param is ignored because Tendermint will override it with
	// remote IP anyway.
	w.subscriptions[query] = outc
	w.mtx.Unlock()

	return outc, nil
}

// Unsubscribe implements EventsClient by using WSClient to unsubscribe given
// subscriber from query.
//
// It returns an error if WSEvents is not running.
func (w *WSEvents) Unsubscribe(ctx context.Context, subscriber, query string) error {
	if !w.IsRunning() {
		return errNotRunning
	}

	if err := w.ws.Unsubscribe(ctx, query); err != nil {
		return err
	}

	w.mtx.Lock()
	_, ok := w.subscriptions[query]
	if ok {
		delete(w.subscriptions, query)
	}
	w.mtx.Unlock()

	return nil
}

// UnsubscribeAll implements EventsClient by using WSClient to unsubscribe
// given subscriber from all the queries.
//
// It returns an error if WSEvents is not running.
func (w *WSEvents) UnsubscribeAll(ctx context.Context, subscriber string) error {
	if !w.IsRunning() {
		return errNotRunning
	}

	if err := w.ws.UnsubscribeAll(ctx); err != nil {
		return err
	}

	w.mtx.Lock()
	w.subscriptions = make(map[string]chan ctypes.ResultEvent)
	w.mtx.Unlock()

	return nil
}

// After being reconnected, it is necessary to redo subscription to server
// otherwise no data will be automatically received.
func (w *WSEvents) redoSubscriptionsAfter(d time.Duration) {
	time.Sleep(d)

	w.mtx.RLock()
	defer w.mtx.RUnlock()
	for q := range w.subscriptions {
		err := w.ws.Subscribe(context.Background(), q)
		if err != nil {
			w.Logger.Error("Failed to resubscribe", "err", err)
		}
	}
}

func isErrAlreadySubscribed(err error) bool {
	return strings.Contains(err.Error(), tmpubsub.ErrAlreadySubscribed.Error())
}

func (w *WSEvents) eventListener() {
	for {
		select {
		case resp, ok := <-w.ws.ResponsesCh:
			if !ok {
				return
			}

			if resp.Error != nil {
				w.Logger.Error("WS error", "err", resp.Error.Error())
				// Error can be ErrAlreadySubscribed or max client (subscriptions per
				// client) reached or Tendermint exited.
				// We can ignore ErrAlreadySubscribed, but need to retry in other
				// cases.
				if !isErrAlreadySubscribed(resp.Error) {
					// Resubscribe after 1 second to give Tendermint time to restart (if
					// crashed).
					w.redoSubscriptionsAfter(1 * time.Second)
				}
				continue
			}

			result := new(ctypes.ResultEvent)
			err := w.cdc.UnmarshalJSON(resp.Result, result)
			if err != nil {
				w.Logger.Error("failed to unmarshal response", "err", err)
				continue
			}

			w.mtx.RLock()
			if out, ok := w.subscriptions[result.Query]; ok {
				if cap(out) == 0 {
					out <- *result
				} else {
					select {
					case out <- *result:
					default:
						w.Logger.Error("wanted to publish ResultEvent, but out channel is full", "result", result, "query", result.Query)
					}
				}
			}
			w.mtx.RUnlock()
		case <-w.Quit():
			return
		}
	}
}
