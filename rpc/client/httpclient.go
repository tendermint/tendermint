package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/pkg/errors"

	data "github.com/tendermint/go-wire/data"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpcclient "github.com/tendermint/tendermint/rpc/lib/client"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
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

// New takes a remote endpoint in the form tcp://<host>:<port>
// and the websocket path (which always seems to be "/websocket")
func NewHTTP(remote, wsEndpoint string) *HTTP {
	return &HTTP{
		rpc:      rpcclient.NewJSONRPCClient(remote),
		remote:   remote,
		WSEvents: newWSEvents(remote, wsEndpoint),
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

func (c *HTTP) ABCIQuery(path string, data data.Bytes) (*ctypes.ResultABCIQuery, error) {
	return c.ABCIQueryWithOptions(path, data, DefaultABCIQueryOptions)
}

func (c *HTTP) ABCIQueryWithOptions(path string, data data.Bytes, opts ABCIQueryOptions) (*ctypes.ResultABCIQuery, error) {
	result := new(ctypes.ResultABCIQuery)
	_, err := c.rpc.Call("abci_query",
		map[string]interface{}{"path": path, "data": data, "height": opts.Height, "trusted": opts.Trusted},
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

func (c *HTTP) BlockchainInfo(minHeight, maxHeight int) (*ctypes.ResultBlockchainInfo, error) {
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

func (c *HTTP) Block(height *int) (*ctypes.ResultBlock, error) {
	result := new(ctypes.ResultBlock)
	_, err := c.rpc.Call("block", map[string]interface{}{"height": height}, result)
	if err != nil {
		return nil, errors.Wrap(err, "Block")
	}
	return result, nil
}

func (c *HTTP) Commit(height *int) (*ctypes.ResultCommit, error) {
	result := new(ctypes.ResultCommit)
	_, err := c.rpc.Call("commit", map[string]interface{}{"height": height}, result)
	if err != nil {
		return nil, errors.Wrap(err, "Commit")
	}
	return result, nil
}

func (c *HTTP) Tx(hash []byte, prove bool) (*ctypes.ResultTx, error) {
	result := new(ctypes.ResultTx)
	query := map[string]interface{}{
		"hash":  hash,
		"prove": prove,
	}
	_, err := c.rpc.Call("tx", query, result)
	if err != nil {
		return nil, errors.Wrap(err, "Tx")
	}
	return result, nil
}

func (c *HTTP) Validators(height *int) (*ctypes.ResultValidators, error) {
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
	remote   string
	endpoint string
	ws       *rpcclient.WSClient

	subscriptions map[string]chan<- interface{}
	mtx           sync.RWMutex

	// used for signaling the goroutine that feeds ws -> EventSwitch
	quit chan bool
	done chan bool
}

func newWSEvents(remote, endpoint string) *WSEvents {
	wsEvents := &WSEvents{
		endpoint:      endpoint,
		remote:        remote,
		quit:          make(chan bool, 1),
		done:          make(chan bool, 1),
		subscriptions: make(map[string]chan<- interface{}),
	}

	wsEvents.BaseService = *cmn.NewBaseService(nil, "WSEvents", wsEvents)
	return wsEvents
}

// Start is the only way I could think the extend OnStart from
// events.eventSwitch.  If only it wasn't private...
// BaseService.Start -> eventSwitch.OnStart -> WSEvents.Start
func (w *WSEvents) Start() (bool, error) {
	ws := rpcclient.NewWSClient(w.remote, w.endpoint, rpcclient.OnReconnect(func() {
		w.redoSubscriptions()
	}))
	started, err := ws.Start()
	if err == nil {
		w.ws = ws
		go w.eventListener()
	}
	return started, errors.Wrap(err, "StartWSEvent")
}

// Stop wraps the BaseService/eventSwitch actions as Start does
func (w *WSEvents) Stop() bool {
	// send a message to quit to stop the eventListener
	w.quit <- true
	<-w.done
	w.ws.Stop()
	w.ws = nil
	return true
}

func (w *WSEvents) Subscribe(ctx context.Context, query string, out chan<- interface{}) error {
	w.mtx.RLock()
	if _, ok := w.subscriptions[query]; ok {
		return errors.New("already subscribed")
	}
	w.mtx.RUnlock()

	err := w.ws.Subscribe(ctx, query)
	if err != nil {
		return errors.Wrap(err, "failed to subscribe")
	}

	w.mtx.Lock()
	w.subscriptions[query] = out
	w.mtx.Unlock()

	return nil
}

func (w *WSEvents) Unsubscribe(ctx context.Context, query string) error {
	err := w.ws.Unsubscribe(ctx, query)
	if err != nil {
		return err
	}

	w.mtx.Lock()
	defer w.mtx.Unlock()
	ch, ok := w.subscriptions[query]
	if ok {
		close(ch)
		delete(w.subscriptions, query)
	}

	return nil
}

func (w *WSEvents) UnsubscribeAll(ctx context.Context) error {
	err := w.ws.UnsubscribeAll(ctx)
	if err != nil {
		return err
	}

	w.mtx.Lock()
	defer w.mtx.Unlock()
	for _, ch := range w.subscriptions {
		close(ch)
	}
	w.subscriptions = make(map[string]chan<- interface{})
	return nil
}

// After being reconnected, it is necessary to redo subscription to server
// otherwise no data will be automatically received.
func (w *WSEvents) redoSubscriptions() {
	for query := range w.subscriptions {
		// NOTE: no timeout for resubscribing
		// FIXME: better logging/handling of errors??
		w.ws.Subscribe(context.Background(), query)
	}
}

// eventListener is an infinite loop pulling all websocket events
// and pushing them to the EventSwitch.
//
// the goroutine only stops by closing quit
func (w *WSEvents) eventListener() {
	for {
		select {
		case resp := <-w.ws.ResponsesCh:
			// res is json.RawMessage
			if resp.Error != nil {
				// FIXME: better logging/handling of errors??
				fmt.Printf("ws err: %+v\n", resp.Error.Error())
				continue
			}
			err := w.parseEvent(*resp.Result)
			if err != nil {
				// FIXME: better logging/handling of errors??
				fmt.Printf("ws result: %+v\n", err)
			}
		case <-w.quit:
			// send a message so we can wait for the routine to exit
			// before cleaning up the w.ws stuff
			w.done <- true
			return
		}
	}
}

// parseEvent unmarshals the json message and converts it into
// some implementation of types.TMEventData, and sends it off
// on the merry way to the EventSwitch
func (w *WSEvents) parseEvent(data []byte) (err error) {
	result := new(ctypes.ResultEvent)
	err = json.Unmarshal(data, result)
	if err != nil {
		// ignore silently (eg. subscribe, unsubscribe and maybe other events)
		// TODO: ?
		return nil
	}
	w.mtx.RLock()
	if ch, ok := w.subscriptions[result.Query]; ok {
		ch <- result.Data
	}
	w.mtx.RUnlock()
	return nil
}
