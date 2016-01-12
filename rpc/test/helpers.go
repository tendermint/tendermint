package rpctest

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-p2p"
	"github.com/tendermint/go-wire"

	client "github.com/tendermint/go-rpc/client"
	"github.com/tendermint/go-rpc/types"
	_ "github.com/tendermint/tendermint/config/tendermint_test"
	nm "github.com/tendermint/tendermint/node"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

// global variables for use across all tests
var (
	node *nm.Node

	mempoolCount = 0

	chainID string

	rpcAddr, requestAddr, websocketAddr string

	clientURI  *client.ClientURI
	clientJSON *client.ClientJSONRPC
)

// initialize config and create new node
func init() {
	initConfig()

	chainID = config.GetString("chain_id")
	rpcAddr = config.GetString("rpc_laddr")
	requestAddr = "http://" + rpcAddr
	websocketAddr = "ws://" + rpcAddr + "/websocket"

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
	node = nm.NewNode()
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
func newWSCon(t *testing.T) *websocket.Conn {
	dialer := websocket.DefaultDialer
	rHeader := http.Header{}
	con, r, err := dialer.Dial(websocketAddr, rHeader)
	fmt.Println("response", r)
	if err != nil {
		t.Fatal(err)
	}
	return con
}

// subscribe to an event
func subscribe(t *testing.T, con *websocket.Conn, eventid string) {
	err := con.WriteJSON(rpctypes.RPCRequest{
		JSONRPC: "2.0",
		ID:      "",
		Method:  "subscribe",
		Params:  []interface{}{eventid},
	})
	if err != nil {
		t.Fatal(err)
	}
}

// unsubscribe from an event
func unsubscribe(t *testing.T, con *websocket.Conn, eventid string) {
	err := con.WriteJSON(rpctypes.RPCRequest{
		JSONRPC: "2.0",
		ID:      "",
		Method:  "unsubscribe",
		Params:  []interface{}{eventid},
	})
	if err != nil {
		t.Fatal(err)
	}
}

// wait for an event; do things that might trigger events, and check them when they are received
// the check function takes an event id and the byte slice read off the ws
func waitForEvent(t *testing.T, con *websocket.Conn, eventid string, dieOnTimeout bool, f func(), check func(string, []byte) error) {
	// go routine to wait for webscoket msg
	goodCh := make(chan []byte)
	errCh := make(chan error)
	quitCh := make(chan struct{})
	defer close(quitCh)

	// Read message
	go func() {
		for {
			_, p, err := con.ReadMessage()
			if err != nil {
				errCh <- err
				break
			} else {
				// if the event id isnt what we're waiting on
				// ignore it
				var response rpctypes.RPCResponse
				var err error
				wire.ReadJSON(&response, p, &err)
				if err != nil {
					errCh <- err
					break
				}
				event, ok := response.Result.(*ctypes.TendermintResult).Result.(*ctypes.ResultEvent)
				if ok && event.Event == eventid {
					goodCh <- p
					break
				}
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
			con.Close()
			t.Fatalf("%s event was not received in time", eventid)
		}
		// else that's great, we didn't hear the event
		// and we shouldn't have
	case p := <-goodCh:
		if dieOnTimeout {
			// message was received and expected
			// run the check
			err := check(eventid, p)
			if err != nil {
				t.Fatal(err)
				panic(err) // Show the stack trace.
			}
		} else {
			con.Close()
			t.Fatalf("%s event was not expected", eventid)
		}
	case err := <-errCh:
		t.Fatal(err)
		panic(err) // Show the stack trace.
	}
}

//--------------------------------------------------------------------------------

func unmarshalResponseNewBlock(b []byte) (*types.Block, error) {
	// unmarshall and assert somethings
	var response rpctypes.RPCResponse
	var err error
	wire.ReadJSON(&response, b, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	block := response.Result.(*ctypes.TendermintResult).Result.(*ctypes.ResultEvent).Data.(types.EventDataNewBlock).Block
	return block, nil
}

func unmarshalValidateBlockchain(t *testing.T, con *websocket.Conn, eid string) {
	var initBlockN int
	for i := 0; i < 3; i++ {
		waitForEvent(t, con, eid, true, func() {}, func(eid string, b []byte) error {
			block, err := unmarshalResponseNewBlock(b)
			if err != nil {
				return err
			}
			if i == 0 {
				initBlockN = block.Header.Height
			} else {
				if block.Header.Height != initBlockN+i {
					return fmt.Errorf("Expected block %d, got block %d", i, block.Header.Height)
				}
			}

			return nil
		})
	}
}
