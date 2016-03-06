package rpctest

import (
	"testing"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-p2p"
	"github.com/tendermint/go-wire"

	client "github.com/tendermint/go-rpc/client"
	nm "github.com/tendermint/tendermint/node"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

// global variables for use across all tests
var (
	node              *nm.Node
	mempoolCount      = 0
	chainID           string
	rpcAddr           string
	requestAddr       string
	websocketAddr     string
	websocketEndpoint string
	clientURI         *client.ClientURI
	clientJSON        *client.ClientJSONRPC
)

// initialize config and create new node
func initGlobalVariables() {
	chainID = config.GetString("chain_id")
	rpcAddr = config.GetString("rpc_laddr")
	requestAddr = rpcAddr
	websocketAddr = rpcAddr
	websocketEndpoint = "/websocket"

	clientURI = client.NewClientURI(requestAddr)
	clientJSON = client.NewClientJSONRPC(requestAddr)

	// TODO: change consensus/state.go timeouts to be shorter

	// start a node
	ready := make(chan struct{})
	go newNode(ready)
	<-ready
}

// create a new node and sleep forever
func newNode(ready chan struct{}) {
	// Create & start node
	privValidatorFile := config.GetString("priv_validator_file")
	privValidator := types.LoadOrGenPrivValidator(privValidatorFile)
	node = nm.NewNode(privValidator)
	l := p2p.NewDefaultListener("tcp", config.GetString("node_laddr"), true)
	node.AddListener(l)
	node.Start()

	// Run the RPC server.
	node.StartRPC()
	ready <- struct{}{}

	// Sleep forever
	ch := make(chan struct{})
	<-ch
}

//--------------------------------------------------------------------------------
// Utilities for testing the websocket service

// create a new connection
func newWSClient(t *testing.T) *client.WSClient {
	wsc := client.NewWSClient(websocketAddr, websocketEndpoint)
	if _, err := wsc.Start(); err != nil {
		t.Fatal(err)
	}
	return wsc
}

// subscribe to an event
func subscribe(t *testing.T, wsc *client.WSClient, eventid string) {
	if err := wsc.Subscribe(eventid); err != nil {
		t.Fatal(err)
	}
}

// unsubscribe from an event
func unsubscribe(t *testing.T, wsc *client.WSClient, eventid string) {
	if err := wsc.Unsubscribe(eventid); err != nil {
		t.Fatal(err)
	}
}

// wait for an event; do things that might trigger events, and check them when they are received
// the check function takes an event id and the byte slice read off the ws
func waitForEvent(t *testing.T, wsc *client.WSClient, eventid string, dieOnTimeout bool, f func(), check func(string, interface{}) error) {
	// go routine to wait for webscoket msg
	goodCh := make(chan interface{})
	errCh := make(chan error)

	// Read message
	go func() {
		var err error
	LOOP:
		for {
			select {
			case r := <-wsc.ResultsCh:
				result := new(ctypes.TMResult)
				wire.ReadJSONPtr(result, r, &err)
				if err != nil {
					errCh <- err
					break LOOP
				}
				event, ok := (*result).(*ctypes.ResultEvent)
				if ok && event.Name == eventid {
					goodCh <- event.Data
					break LOOP
				}
			case err := <-wsc.ErrorsCh:
				errCh <- err
				break LOOP
			case <-wsc.Quit:
				break LOOP
			}
		}
	}()

	// do stuff (transactions)
	f()

	// wait for an event or timeout
	timeout := time.NewTimer(10 * time.Second)
	select {
	case <-timeout.C:
		if dieOnTimeout {
			wsc.Stop()
			t.Fatalf("%s event was not received in time", eventid)
		}
		// else that's great, we didn't hear the event
		// and we shouldn't have
	case eventData := <-goodCh:
		if dieOnTimeout {
			// message was received and expected
			// run the check
			if err := check(eventid, eventData); err != nil {
				t.Fatal(err) // Show the stack trace.
			}
		} else {
			wsc.Stop()
			t.Fatalf("%s event was not expected", eventid)
		}
	case err := <-errCh:
		t.Fatal(err)
		panic(err) // Show the stack trace.

	}
}

//--------------------------------------------------------------------------------
