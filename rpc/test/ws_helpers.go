package rpctest

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tendermint/go-wire"
	_ "github.com/tendermint/tendermint/config/tendermint_test"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/rpc/types"
	"github.com/tendermint/tendermint/types"
)

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
				var response ctypes.Response
				var err error
				wire.ReadJSON(&response, p, &err)
				if err != nil {
					errCh <- err
					break
				}
				event, ok := response.Result.(*ctypes.ResultEvent)
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
	var response ctypes.Response
	var err error
	wire.ReadJSON(&response, b, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	block := response.Result.(*ctypes.ResultEvent).Data.(types.EventDataNewBlock).Block
	return block, nil
}

func unmarshalResponseNameReg(b []byte) (*types.NameTx, error) {
	// unmarshall and assert somethings
	var response ctypes.Response
	var err error
	wire.ReadJSON(&response, b, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	tx := response.Result.(*ctypes.ResultEvent).Data.(types.EventDataTx).Tx.(*types.NameTx)
	return tx, nil
}

func unmarshalValidateBlockchain(t *testing.T, con *websocket.Conn, eid string) {
	var initBlockN int
	for i := 0; i < 2; i++ {
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

func unmarshalValidateSend(amt int64, toAddr []byte) func(string, []byte) error {
	return func(eid string, b []byte) error {
		// unmarshal and assert correctness
		var response ctypes.Response
		var err error
		wire.ReadJSON(&response, b, &err)
		if err != nil {
			return err
		}
		if response.Error != "" {
			return fmt.Errorf(response.Error)
		}
		if eid != response.Result.(*ctypes.ResultEvent).Event {
			return fmt.Errorf("Eventid is not correct. Got %s, expected %s", response.Result.(*ctypes.ResultEvent).Event, eid)
		}
		tx := response.Result.(*ctypes.ResultEvent).Data.(types.EventDataTx).Tx.(*types.SendTx)
		if !bytes.Equal(tx.Inputs[0].Address, user[0].Address) {
			return fmt.Errorf("Senders do not match up! Got %x, expected %x", tx.Inputs[0].Address, user[0].Address)
		}
		if tx.Inputs[0].Amount != amt {
			return fmt.Errorf("Amt does not match up! Got %d, expected %d", tx.Inputs[0].Amount, amt)
		}
		if !bytes.Equal(tx.Outputs[0].Address, toAddr) {
			return fmt.Errorf("Receivers do not match up! Got %x, expected %x", tx.Outputs[0].Address, user[0].Address)
		}
		return nil
	}
}

func unmarshalValidateTx(amt int64, returnCode []byte) func(string, []byte) error {
	return func(eid string, b []byte) error {
		// unmarshall and assert somethings
		var response ctypes.Response
		var err error
		wire.ReadJSON(&response, b, &err)
		if err != nil {
			return err
		}
		if response.Error != "" {
			return fmt.Errorf(response.Error)
		}
		var data = response.Result.(*ctypes.ResultEvent).Data.(types.EventDataTx)
		if data.Exception != "" {
			return fmt.Errorf(data.Exception)
		}
		tx := data.Tx.(*types.CallTx)
		if !bytes.Equal(tx.Input.Address, user[0].Address) {
			return fmt.Errorf("Senders do not match up! Got %x, expected %x",
				tx.Input.Address, user[0].Address)
		}
		if tx.Input.Amount != amt {
			return fmt.Errorf("Amt does not match up! Got %d, expected %d",
				tx.Input.Amount, amt)
		}
		ret := data.Return
		if !bytes.Equal(ret, returnCode) {
			return fmt.Errorf("Tx did not return correctly. Got %x, expected %x", ret, returnCode)
		}
		return nil
	}
}

func unmarshalValidateCall(origin, returnCode []byte, txid *[]byte) func(string, []byte) error {
	return func(eid string, b []byte) error {
		// unmarshall and assert somethings
		var response ctypes.Response
		var err error
		wire.ReadJSON(&response, b, &err)
		if err != nil {
			return err
		}
		if response.Error != "" {
			return fmt.Errorf(response.Error)
		}
		var data = response.Result.(*ctypes.ResultEvent).Data.(types.EventDataCall)
		if data.Exception != "" {
			return fmt.Errorf(data.Exception)
		}
		if !bytes.Equal(data.Origin, origin) {
			return fmt.Errorf("Origin does not match up! Got %x, expected %x",
				data.Origin, origin)
		}
		ret := data.Return
		if !bytes.Equal(ret, returnCode) {
			return fmt.Errorf("Call did not return correctly. Got %x, expected %x", ret, returnCode)
		}
		if !bytes.Equal(data.TxID, *txid) {
			return fmt.Errorf("TxIDs do not match up! Got %x, expected %x",
				data.TxID, *txid)
		}
		return nil
	}
}
