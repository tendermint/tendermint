package client

import (
	"fmt"

	"github.com/pkg/errors"
	events "github.com/tendermint/go-events"
	"github.com/tendermint/go-rpc/client"
	wire "github.com/tendermint/go-wire"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
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
	rpc    *rpcclient.ClientJSONRPC
	*WSEvents
}

// New takes a remote endpoint in the form tcp://<host>:<port>
// and the websocket path (which always seems to be "/websocket")
func NewHTTP(remote, wsEndpoint string) *HTTP {
	return &HTTP{
		rpc:      rpcclient.NewClientJSONRPC(remote),
		remote:   remote,
		WSEvents: newWSEvents(remote, wsEndpoint),
	}
}

func (c *HTTP) _assertIsClient() Client {
	return c
}

func (c *HTTP) _assertIsNetworkClient() NetworkClient {
	return c
}

func (c *HTTP) _assertIsEventSwitch() types.EventSwitch {
	return c
}

func (c *HTTP) Status() (*ctypes.ResultStatus, error) {
	tmResult := new(ctypes.TMResult)
	_, err := c.rpc.Call("status", []interface{}{}, tmResult)
	if err != nil {
		return nil, errors.Wrap(err, "Status")
	}
	// note: panics if rpc doesn't match.  okay???
	return (*tmResult).(*ctypes.ResultStatus), nil
}

func (c *HTTP) ABCIInfo() (*ctypes.ResultABCIInfo, error) {
	tmResult := new(ctypes.TMResult)
	_, err := c.rpc.Call("abci_info", []interface{}{}, tmResult)
	if err != nil {
		return nil, errors.Wrap(err, "ABCIInfo")
	}
	return (*tmResult).(*ctypes.ResultABCIInfo), nil
}

func (c *HTTP) ABCIQuery(path string, data []byte, prove bool) (*ctypes.ResultABCIQuery, error) {
	tmResult := new(ctypes.TMResult)
	_, err := c.rpc.Call("abci_query", []interface{}{path, data, prove}, tmResult)
	if err != nil {
		return nil, errors.Wrap(err, "ABCIQuery")
	}
	return (*tmResult).(*ctypes.ResultABCIQuery), nil
}

func (c *HTTP) BroadcastTxCommit(tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	tmResult := new(ctypes.TMResult)
	_, err := c.rpc.Call("broadcast_tx_commit", []interface{}{tx}, tmResult)
	if err != nil {
		return nil, errors.Wrap(err, "broadcast_tx_commit")
	}
	return (*tmResult).(*ctypes.ResultBroadcastTxCommit), nil
}

func (c *HTTP) BroadcastTxAsync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return c.broadcastTX("broadcast_tx_async", tx)
}

func (c *HTTP) BroadcastTxSync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return c.broadcastTX("broadcast_tx_sync", tx)
}

func (c *HTTP) broadcastTX(route string, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	tmResult := new(ctypes.TMResult)
	_, err := c.rpc.Call(route, []interface{}{tx}, tmResult)
	if err != nil {
		return nil, errors.Wrap(err, route)
	}
	return (*tmResult).(*ctypes.ResultBroadcastTx), nil
}

func (c *HTTP) NetInfo() (*ctypes.ResultNetInfo, error) {
	tmResult := new(ctypes.TMResult)
	_, err := c.rpc.Call("net_info", nil, tmResult)
	if err != nil {
		return nil, errors.Wrap(err, "NetInfo")
	}
	return (*tmResult).(*ctypes.ResultNetInfo), nil
}

func (c *HTTP) DumpConsensusState() (*ctypes.ResultDumpConsensusState, error) {
	tmResult := new(ctypes.TMResult)
	_, err := c.rpc.Call("dump_consensus_state", nil, tmResult)
	if err != nil {
		return nil, errors.Wrap(err, "DumpConsensusState")
	}
	return (*tmResult).(*ctypes.ResultDumpConsensusState), nil
}

func (c *HTTP) BlockchainInfo(minHeight, maxHeight int) (*ctypes.ResultBlockchainInfo, error) {
	tmResult := new(ctypes.TMResult)
	_, err := c.rpc.Call("blockchain", []interface{}{minHeight, maxHeight}, tmResult)
	if err != nil {
		return nil, errors.Wrap(err, "BlockchainInfo")
	}
	return (*tmResult).(*ctypes.ResultBlockchainInfo), nil
}

func (c *HTTP) Genesis() (*ctypes.ResultGenesis, error) {
	tmResult := new(ctypes.TMResult)
	_, err := c.rpc.Call("genesis", nil, tmResult)
	if err != nil {
		return nil, errors.Wrap(err, "Genesis")
	}
	return (*tmResult).(*ctypes.ResultGenesis), nil
}

func (c *HTTP) Block(height int) (*ctypes.ResultBlock, error) {
	tmResult := new(ctypes.TMResult)
	_, err := c.rpc.Call("block", []interface{}{height}, tmResult)
	if err != nil {
		return nil, errors.Wrap(err, "Block")
	}
	return (*tmResult).(*ctypes.ResultBlock), nil
}

func (c *HTTP) Commit(height int) (*ctypes.ResultCommit, error) {
	tmResult := new(ctypes.TMResult)
	_, err := c.rpc.Call("commit", []interface{}{height}, tmResult)
	if err != nil {
		return nil, errors.Wrap(err, "Commit")
	}
	return (*tmResult).(*ctypes.ResultCommit), nil
}

func (c *HTTP) Validators() (*ctypes.ResultValidators, error) {
	tmResult := new(ctypes.TMResult)
	_, err := c.rpc.Call("validators", nil, tmResult)
	if err != nil {
		return nil, errors.Wrap(err, "Validators")
	}
	return (*tmResult).(*ctypes.ResultValidators), nil
}

/** websocket event stuff here... **/

type WSEvents struct {
	types.EventSwitch
	remote   string
	endpoint string
	ws       *rpcclient.WSClient
	quit     chan bool
}

func newWSEvents(remote, endpoint string) *WSEvents {
	return &WSEvents{
		EventSwitch: types.NewEventSwitch(),
		endpoint:    endpoint,
		remote:      remote,
		quit:        make(chan bool, 1),
	}
}

func (w *WSEvents) _assertIsEventSwitch() types.EventSwitch {
	return w
}

// Start is the only way I could think the extend OnStart from
// events.eventSwitch.  If only it wasn't private...
// BaseService.Start -> eventSwitch.OnStart -> WSEvents.Start
func (w *WSEvents) Start() (bool, error) {
	st, err := w.EventSwitch.Start()
	// if we did start, then OnStart here...
	if st && err == nil {
		ws := rpcclient.NewWSClient(w.remote, w.endpoint)
		_, err = ws.Start()
		if err == nil {
			w.ws = ws
			go w.eventListener()
		}
	}
	return st, errors.Wrap(err, "StartWSEvent")
}

// Stop wraps the BaseService/eventSwitch actions as Start does
func (w *WSEvents) Stop() bool {
	stop := w.EventSwitch.Stop()
	if stop {
		// send a message to quit to stop the eventListener
		w.quit <- true
		w.ws.Stop()
	}
	return stop
}

/** TODO: more intelligent subscriptions! **/
func (w *WSEvents) AddListenerForEvent(listenerID, event string, cb events.EventCallback) {
	w.subscribe(event)
	w.EventSwitch.AddListenerForEvent(listenerID, event, cb)
}

func (w *WSEvents) RemoveListenerForEvent(event string, listenerID string) {
	w.unsubscribe(event)
	w.EventSwitch.RemoveListenerForEvent(event, listenerID)
}

func (w *WSEvents) RemoveListener(listenerID string) {
	w.EventSwitch.RemoveListener(listenerID)
}

// eventListener is an infinite loop pulling all websocket events
// and pushing them to the EventSwitch.
//
// the goroutine only stops by closing quit
func (w *WSEvents) eventListener() {
	for {
		select {
		case res := <-w.ws.ResultsCh:
			// res is json.RawMessage
			err := w.parseEvent(res)
			if err != nil {
				// FIXME: better logging/handling of errors??
				fmt.Printf("ws result: %+v\n", err)
			}
		case err := <-w.ws.ErrorsCh:
			// FIXME: better logging/handling of errors??
			fmt.Printf("ws err: %+v\n", err)
		case <-w.quit:
			// only way to finish this method
			return
		}
	}
}

// parseEvent unmarshals the json message and converts it into
// some implementation of types.TMEventData, and sends it off
// on the merry way to the EventSwitch
func (w *WSEvents) parseEvent(data []byte) (err error) {
	result := new(ctypes.TMResult)
	wire.ReadJSONPtr(result, data, &err)
	if err != nil {
		return err
	}
	event, ok := (*result).(*ctypes.ResultEvent)
	if !ok {
		// ignore silently (eg. subscribe, unsubscribe and maybe other events)
		return nil
		// or report loudly???
		// return errors.Errorf("unknown message: %#v", *result)
	}
	// looks good!  let's fire this baby!
	w.EventSwitch.FireEvent(event.Name, event.Data)
	return nil
}

func (w *WSEvents) subscribe(event string) error {
	return errors.Wrap(w.ws.Subscribe(event), "Subscribe")
}

func (w *WSEvents) unsubscribe(event string) error {
	return errors.Wrap(w.ws.Unsubscribe(event), "Unsubscribe")
}
