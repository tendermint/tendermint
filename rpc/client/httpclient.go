package client

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	amino "github.com/tendermint/go-amino"
	cmn "github.com/tendermint/tendermint/libs/common"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpcclient "github.com/tendermint/tendermint/rpc/lib/client"
	"github.com/tendermint/tendermint/types"
)

/*
HTTP is a Client implementation that communicates
with a tendermint node over json rpc and websockets.

This is the main implementation you probably want to use in
production code.  There are other implementations when calling
the tendermint node in-process (local), or when you want to mock
out the server for test code (mock).
*/
type HTTP struct {
	remote string
	rpc    *rpcclient.JSONRPCClient
	*WSEvents
}

// NewHTTP takes a remote endpoint in the form tcp://<host>:<port>
// and the websocket path (which always seems to be "/websocket")
func NewHTTP(remote, wsEndpoint string) *HTTP {
	rc := rpcclient.NewJSONRPCClient(remote)
	cdc := rc.Codec()
	ctypes.RegisterAmino(cdc)
	rc.SetCodec(cdc)

	return &HTTP{
		rpc:      rc,
		remote:   remote,
		WSEvents: newWSEvents(cdc, remote, wsEndpoint),
	}
}

var (
	_ Client        = (*HTTP)(nil)
	_ NetworkClient = (*HTTP)(nil)
	_ EventsClient  = (*HTTP)(nil)
)

func (c *HTTP) Status() (*ctypes.ResultStatus, error) {
	result := new(ctypes.ResultStatus)
	_, err := c.rpc.Call("status", map[string]interface{}{}, result)
	if err != nil {
		return nil, errors.Wrap(err, "Status")
	}
	return result, nil
}

func (c *HTTP) ABCIInfo() (*ctypes.ResultABCIInfo, error) {
	result := new(ctypes.ResultABCIInfo)
	_, err := c.rpc.Call("abci_info", map[string]interface{}{}, result)
	if err != nil {
		return nil, errors.Wrap(err, "ABCIInfo")
	}
	return result, nil
}

func (c *HTTP) ABCIQuery(path string, data cmn.HexBytes) (*ctypes.ResultABCIQuery, error) {
	return c.ABCIQueryWithOptions(path, data, DefaultABCIQueryOptions)
}

func (c *HTTP) ABCIQueryWithOptions(path string, data cmn.HexBytes, opts ABCIQueryOptions) (*ctypes.ResultABCIQuery, error) {
	result := new(ctypes.ResultABCIQuery)
	_, err := c.rpc.Call("abci_query",
		map[string]interface{}{"path": path, "data": data, "height": opts.Height, "prove": opts.Prove},
		result)
	if err != nil {
		return nil, errors.Wrap(err, "ABCIQuery")
	}
	return result, nil
}

func (c *HTTP) BroadcastTxCommit(tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	result := new(ctypes.ResultBroadcastTxCommit)
	_, err := c.rpc.Call("broadcast_tx_commit", map[string]interface{}{"tx": tx}, result)
	if err != nil {
		return nil, errors.Wrap(err, "broadcast_tx_commit")
	}
	return result, nil
}

func (c *HTTP) BroadcastTxAsync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return c.broadcastTX("broadcast_tx_async", tx)
}

func (c *HTTP) BroadcastTxSync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return c.broadcastTX("broadcast_tx_sync", tx)
}

func (c *HTTP) broadcastTX(route string, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	result := new(ctypes.ResultBroadcastTx)
	_, err := c.rpc.Call(route, map[string]interface{}{"tx": tx}, result)
	if err != nil {
		return nil, errors.Wrap(err, route)
	}
	return result, nil
}

func (c *HTTP) UnconfirmedTxs(limit int) (*ctypes.ResultUnconfirmedTxs, error) {
	result := new(ctypes.ResultUnconfirmedTxs)
	_, err := c.rpc.Call("unconfirmed_txs", map[string]interface{}{"limit": limit}, result)
	if err != nil {
		return nil, errors.Wrap(err, "unconfirmed_txs")
	}
	return result, nil
}

func (c *HTTP) NumUnconfirmedTxs() (*ctypes.ResultUnconfirmedTxs, error) {
	result := new(ctypes.ResultUnconfirmedTxs)
	_, err := c.rpc.Call("num_unconfirmed_txs", map[string]interface{}{}, result)
	if err != nil {
		return nil, errors.Wrap(err, "num_unconfirmed_txs")
	}
	return result, nil
}

func (c *HTTP) NetInfo() (*ctypes.ResultNetInfo, error) {
	result := new(ctypes.ResultNetInfo)
	_, err := c.rpc.Call("net_info", map[string]interface{}{}, result)
	if err != nil {
		return nil, errors.Wrap(err, "NetInfo")
	}
	return result, nil
}

func (c *HTTP) DumpConsensusState() (*ctypes.ResultDumpConsensusState, error) {
	result := new(ctypes.ResultDumpConsensusState)
	_, err := c.rpc.Call("dump_consensus_state", map[string]interface{}{}, result)
	if err != nil {
		return nil, errors.Wrap(err, "DumpConsensusState")
	}
	return result, nil
}

func (c *HTTP) ConsensusState() (*ctypes.ResultConsensusState, error) {
	result := new(ctypes.ResultConsensusState)
	_, err := c.rpc.Call("consensus_state", map[string]interface{}{}, result)
	if err != nil {
		return nil, errors.Wrap(err, "ConsensusState")
	}
	return result, nil
}

func (c *HTTP) Health() (*ctypes.ResultHealth, error) {
	result := new(ctypes.ResultHealth)
	_, err := c.rpc.Call("health", map[string]interface{}{}, result)
	if err != nil {
		return nil, errors.Wrap(err, "Health")
	}
	return result, nil
}

func (c *HTTP) BlockchainInfo(minHeight, maxHeight int64) (*ctypes.ResultBlockchainInfo, error) {
	result := new(ctypes.ResultBlockchainInfo)
	_, err := c.rpc.Call("blockchain",
		map[string]interface{}{"minHeight": minHeight, "maxHeight": maxHeight},
		result)
	if err != nil {
		return nil, errors.Wrap(err, "BlockchainInfo")
	}
	return result, nil
}

func (c *HTTP) Genesis() (*ctypes.ResultGenesis, error) {
	result := new(ctypes.ResultGenesis)
	_, err := c.rpc.Call("genesis", map[string]interface{}{}, result)
	if err != nil {
		return nil, errors.Wrap(err, "Genesis")
	}
	return result, nil
}

func (c *HTTP) Block(height *int64) (*ctypes.ResultBlock, error) {
	result := new(ctypes.ResultBlock)
	_, err := c.rpc.Call("block", map[string]interface{}{"height": height}, result)
	if err != nil {
		return nil, errors.Wrap(err, "Block")
	}
	return result, nil
}

func (c *HTTP) BlockResults(height *int64) (*ctypes.ResultBlockResults, error) {
	result := new(ctypes.ResultBlockResults)
	_, err := c.rpc.Call("block_results", map[string]interface{}{"height": height}, result)
	if err != nil {
		return nil, errors.Wrap(err, "Block Result")
	}
	return result, nil
}

func (c *HTTP) Commit(height *int64) (*ctypes.ResultCommit, error) {
	result := new(ctypes.ResultCommit)
	_, err := c.rpc.Call("commit", map[string]interface{}{"height": height}, result)
	if err != nil {
		return nil, errors.Wrap(err, "Commit")
	}
	return result, nil
}

func (c *HTTP) Tx(hash []byte, prove bool) (*ctypes.ResultTx, error) {
	result := new(ctypes.ResultTx)
	params := map[string]interface{}{
		"hash":  hash,
		"prove": prove,
	}
	_, err := c.rpc.Call("tx", params, result)
	if err != nil {
		return nil, errors.Wrap(err, "Tx")
	}
	return result, nil
}

func (c *HTTP) TxSearch(query string, prove bool, page, perPage int) (*ctypes.ResultTxSearch, error) {
	result := new(ctypes.ResultTxSearch)
	params := map[string]interface{}{
		"query":    query,
		"prove":    prove,
		"page":     page,
		"per_page": perPage,
	}
	_, err := c.rpc.Call("tx_search", params, result)
	if err != nil {
		return nil, errors.Wrap(err, "TxSearch")
	}
	return result, nil
}

func (c *HTTP) Validators(height *int64) (*ctypes.ResultValidators, error) {
	result := new(ctypes.ResultValidators)
	_, err := c.rpc.Call("validators", map[string]interface{}{"height": height}, result)
	if err != nil {
		return nil, errors.Wrap(err, "Validators")
	}
	return result, nil
}

/** websocket event stuff here... **/

type WSEvents struct {
	cmn.BaseService
	cdc      *amino.Codec
	remote   string
	endpoint string
	ws       *rpcclient.WSClient

	mtx           sync.RWMutex
	subscriptions map[string]chan<- interface{}
}

func newWSEvents(cdc *amino.Codec, remote, endpoint string) *WSEvents {
	wsEvents := &WSEvents{
		cdc:           cdc,
		endpoint:      endpoint,
		remote:        remote,
		subscriptions: make(map[string]chan<- interface{}),
	}

	wsEvents.BaseService = *cmn.NewBaseService(nil, "WSEvents", wsEvents)
	return wsEvents
}

func (w *WSEvents) OnStart() error {
	w.ws = rpcclient.NewWSClient(w.remote, w.endpoint, rpcclient.OnReconnect(func() {
		w.redoSubscriptions()
	}))
	w.ws.SetCodec(w.cdc)

	err := w.ws.Start()
	if err != nil {
		return err
	}

	go w.eventListener()
	return nil
}

// Stop wraps the BaseService/eventSwitch actions as Start does
func (w *WSEvents) OnStop() {
	err := w.ws.Stop()
	if err != nil {
		w.Logger.Error("failed to stop WSClient", "err", err)
	}
}

func (w *WSEvents) Subscribe(ctx context.Context, subscriber string, query tmpubsub.Query, out chan<- interface{}) error {
	q := query.String()

	err := w.ws.Subscribe(ctx, q)
	if err != nil {
		return err
	}

	w.mtx.Lock()
	// subscriber param is ignored because Tendermint will override it with
	// remote IP anyway.
	w.subscriptions[q] = out
	w.mtx.Unlock()

	return nil
}

func (w *WSEvents) Unsubscribe(ctx context.Context, subscriber string, query tmpubsub.Query) error {
	q := query.String()

	err := w.ws.Unsubscribe(ctx, q)
	if err != nil {
		return err
	}

	w.mtx.Lock()
	ch, ok := w.subscriptions[q]
	if ok {
		close(ch)
		delete(w.subscriptions, q)
	}
	w.mtx.Unlock()

	return nil
}

func (w *WSEvents) UnsubscribeAll(ctx context.Context, subscriber string) error {
	err := w.ws.UnsubscribeAll(ctx)
	if err != nil {
		return err
	}

	w.mtx.Lock()
	for _, ch := range w.subscriptions {
		close(ch)
	}
	w.subscriptions = make(map[string]chan<- interface{})
	w.mtx.Unlock()

	return nil
}

// After being reconnected, it is necessary to redo subscription to server
// otherwise no data will be automatically received.
func (w *WSEvents) redoSubscriptions() {
	for q := range w.subscriptions {
		// NOTE: no timeout for resubscribing
		// FIXME: better logging/handling of errors??
		w.ws.Subscribe(context.Background(), q)
	}
}

// eventListener is an infinite loop pulling all websocket events
// and pushing them to the EventSwitch.
//
// the goroutine only stops by closing quit
func (w *WSEvents) eventListener() {
	for {
		select {
		case resp, ok := <-w.ws.ResponsesCh:
			if !ok {
				return
			}
			if resp.Error != nil {
				w.Logger.Error("WS error", "err", resp.Error.Error())
				continue
			}
			result := new(ctypes.ResultEvent)
			err := w.cdc.UnmarshalJSON(resp.Result, result)
			if err != nil {
				w.Logger.Error("failed to unmarshal response", "err", err)
				continue
			}
			// NOTE: writing also happens inside mutex so we can't close a channel in
			// Unsubscribe/UnsubscribeAll.
			w.mtx.RLock()
			if ch, ok := w.subscriptions[result.Query]; ok {
				ch <- result.Data
			}
			w.mtx.RUnlock()
		case <-w.Quit():
			return
		}
	}
}
