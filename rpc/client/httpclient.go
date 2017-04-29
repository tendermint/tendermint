package client

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	data "github.com/tendermint/go-wire/data"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/rpc/lib/client"
	"github.com/tendermint/tendermint/types"
	events "github.com/tendermint/tmlibs/events"
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

func (c *HTTP) ABCIQuery(path string, data data.Bytes, prove bool) (*ctypes.ResultABCIQuery, error) {
	result := new(ctypes.ResultABCIQuery)
	_, err := c.rpc.Call("abci_query",
		map[string]interface{}{"path": path, "data": data, "prove": prove},
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

func (c *HTTP) Block(height int) (*ctypes.ResultBlock, error) {
	result := new(ctypes.ResultBlock)
	_, err := c.rpc.Call("block", map[string]interface{}{"height": height}, result)
	if err != nil {
		return nil, errors.Wrap(err, "Block")
	}
	return result, nil
}

func (c *HTTP) Commit(height int) (*ctypes.ResultCommit, error) {
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

func (c *HTTP) Validators() (*ctypes.ResultValidators, error) {
	result := new(ctypes.ResultValidators)
	_, err := c.rpc.Call("validators", map[string]interface{}{}, result)
	if err != nil {
		return nil, errors.Wrap(err, "Validators")
	}
	return result, nil
}

/** websocket event stuff here... **/

type WSEvents struct {
	types.EventSwitch
	remote   string
	endpoint string
	ws       *rpcclient.WSClient

	// used for signaling the goroutine that feeds ws -> EventSwitch
	quit chan bool
	done chan bool

	// used to maintain counts of actively listened events
	// so we can properly subscribe/unsubscribe
	// FIXME: thread-safety???
	// FIXME: reuse code from tmlibs/events???
	evtCount  map[string]int      // count how many time each event is subscribed
	listeners map[string][]string // keep track of which events each listener is listening to
}

func newWSEvents(remote, endpoint string) *WSEvents {
	return &WSEvents{
		EventSwitch: types.NewEventSwitch(),
		endpoint:    endpoint,
		remote:      remote,
		quit:        make(chan bool, 1),
		done:        make(chan bool, 1),
		evtCount:    map[string]int{},
		listeners:   map[string][]string{},
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
		<-w.done
		w.ws.Stop()
		w.ws = nil
	}
	return stop
}

/** TODO: more intelligent subscriptions! **/
func (w *WSEvents) AddListenerForEvent(listenerID, event string, cb events.EventCallback) {
	// no one listening -> subscribe
	if w.evtCount[event] == 0 {
		w.subscribe(event)
	}
	// if this listener was already listening to this event, return early
	for _, s := range w.listeners[listenerID] {
		if event == s {
			return
		}
	}
	// otherwise, add this event to this listener
	w.evtCount[event] += 1
	w.listeners[listenerID] = append(w.listeners[listenerID], event)
	w.EventSwitch.AddListenerForEvent(listenerID, event, cb)
}

func (w *WSEvents) RemoveListenerForEvent(event string, listenerID string) {
	// if this listener is listening already, splice it out
	found := false
	l := w.listeners[listenerID]
	for i, s := range l {
		if event == s {
			found = true
			w.listeners[listenerID] = append(l[:i], l[i+1:]...)
			break
		}
	}
	// if the listener wasn't already listening to the event, exit early
	if !found {
		return
	}

	// now we can update the subscriptions
	w.evtCount[event] -= 1
	if w.evtCount[event] == 0 {
		w.unsubscribe(event)
	}
	w.EventSwitch.RemoveListenerForEvent(event, listenerID)
}

func (w *WSEvents) RemoveListener(listenerID string) {
	// remove all counts for this listener
	for _, s := range w.listeners[listenerID] {
		w.evtCount[s] -= 1
		if w.evtCount[s] == 0 {
			w.unsubscribe(s)
		}
	}
	w.listeners[listenerID] = nil

	// then let the switch do it's magic
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
	// looks good!  let's fire this baby!
	w.EventSwitch.FireEvent(result.Name, result.Data)
	return nil
}

// no way of exposing these failures, so we panic.
// is this right?  or silently ignore???
func (w *WSEvents) subscribe(event string) {
	err := w.ws.Subscribe(event)
	if err != nil {
		panic(err)
	}
}

func (w *WSEvents) unsubscribe(event string) {
	err := w.ws.Unsubscribe(event)
	if err != nil {
		panic(err)
	}
}
