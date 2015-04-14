package rpc

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/tendermint/tendermint/binary"
	"github.com/tendermint/tendermint/rpc"
	"github.com/tendermint/tendermint/types"
	"net/http"
	"testing"
	"time"
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
	err := con.WriteJSON(rpc.WSRequest{
		Type:  "subscribe",
		Event: eventid,
	})
	if err != nil {
		t.Fatal(err)
	}
}

// unsubscribe from an event
func unsubscribe(t *testing.T, con *websocket.Conn, eventid string) {
	err := con.WriteJSON(rpc.WSRequest{
		Type:  "unsubscribe",
		Event: eventid,
	})
	if err != nil {
		t.Fatal(err)
	}
}

// wait for an event, do things that might trigger events, and check them when they are received
func waitForEvent(t *testing.T, con *websocket.Conn, eventid string, dieOnTimeout bool, f func(), check func(string, []byte) error) {
	// go routine to wait for webscoket msg
	gch := make(chan []byte)
	ech := make(chan error)
	go func() {
		typ, p, err := con.ReadMessage()
		fmt.Println("RESPONSE:", typ, string(p), err)
		if err != nil {
			ech <- err
		} else {
			gch <- p
		}
	}()

	// do stuff (transactions)
	f()

	// if the event is not received in 20 seconds, die
	ticker := time.Tick(10 * time.Second)
	select {
	case <-ticker:
		if dieOnTimeout {
			con.Close()
			t.Fatalf("%s event was not received in time", eventid)
		}
		// else that's great, we didn't hear the event
	case p := <-gch:
		if dieOnTimeout {
			// message was received and expected
			// run the check
			err := check(eventid, p)
			if err != nil {
				t.Fatal(err)
			}
		} else {
			con.Close()
			t.Fatalf("%s event was not expected", eventid)
		}
	case err := <-ech:
		t.Fatal(err)
	}
}

func unmarshalResponseNewBlock(b []byte) (*types.Block, error) {
	// unmarshall and assert somethings
	var response struct {
		Event string
		Data  *types.Block
		Error string
	}
	var err error
	binary.ReadJSON(&response, b, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	block := response.Data
	return block, nil
}

//--------------------------------------------------------------------------------
// Test the websocket service

// make a simple connection to the server
func TestWSConnect(t *testing.T) {
	con := newWSCon(t)
	con.Close()
}

// receive a new block message
func _TestWSNewBlock(t *testing.T) {
	con := newWSCon(t)
	eid := types.EventStringNewBlock()
	subscribe(t, con, eid)
	defer func() {
		unsubscribe(t, con, eid)
		con.Close()
	}()
	waitForEvent(t, con, eid, true, func() {}, func(eid string, b []byte) error {
		fmt.Println("Check:", string(b))
		return nil
	})
}

// receive a few new block messages in a row, with increasing height
func TestWSBlockchainGrowth(t *testing.T) {
	con := newWSCon(t)
	eid := types.EventStringNewBlock()
	subscribe(t, con, eid)
	defer func() {
		unsubscribe(t, con, eid)
		con.Close()
	}()
	var initBlockN uint
	for i := 0; i < 2; i++ {
		waitForEvent(t, con, eid, true, func() {}, func(eid string, b []byte) error {
			block, err := unmarshalResponseNewBlock(b)
			if err != nil {
				return err
			}
			if i == 0 {
				initBlockN = block.Header.Height
			} else {
				if block.Header.Height != initBlockN+uint(i) {
					return fmt.Errorf("Expected block %d, got block %d", i, block.Header.Height)
				}
			}

			return nil
		})
	}
}

// send a transaction and validate the events from listening for both sender and receiver
func TestWSSend(t *testing.T) {
	toAddr := []byte{20, 143, 25, 63, 16, 177, 83, 29, 91, 91, 54, 23, 233, 46, 190, 121, 122, 34, 86, 54}
	amt := uint64(100)

	con := newWSCon(t)
	eidInput := types.EventStringAccInput(byteAddr)
	eidOutput := types.EventStringAccOutput(toAddr)
	subscribe(t, con, eidInput)
	subscribe(t, con, eidOutput)
	defer func() {
		unsubscribe(t, con, eidInput)
		unsubscribe(t, con, eidOutput)
		con.Close()
	}()
	checkerFunc := func(eid string, b []byte) error {
		// unmarshal and assert correctness
		var response struct {
			Event string
			Data  types.SendTx
			Error string
		}
		var err error
		binary.ReadJSON(&response, b, &err)
		if err != nil {
			return err
		}
		if response.Error != "" {
			return fmt.Errorf(response.Error)
		}
		if eid != response.Event {
			return fmt.Errorf("Eventid is not correct. Got %s, expected %s", response.Event, eid)
		}
		tx := response.Data
		if bytes.Compare(tx.Inputs[0].Address, byteAddr) != 0 {
			return fmt.Errorf("Senders do not match up! Got %x, expected %x", tx.Inputs[0].Address, byteAddr)
		}
		if tx.Inputs[0].Amount != amt {
			return fmt.Errorf("Amt does not match up! Got %d, expected %d", tx.Inputs[0].Amount, amt)
		}
		if bytes.Compare(tx.Outputs[0].Address, toAddr) != 0 {
			return fmt.Errorf("Receivers do not match up! Got %x, expected %x", tx.Outputs[0].Address, byteAddr)
		}
		return nil
	}
	waitForEvent(t, con, eidInput, true, func() {
		broadcastTx(t, "JSONRPC", byteAddr, toAddr, nil, byteKey, amt, 0, 0)
	}, checkerFunc)
	waitForEvent(t, con, eidOutput, true, func() {}, checkerFunc)
}

// ensure events are only fired once for a given transaction
func TestWSDoubleFire(t *testing.T) {
	con := newWSCon(t)
	eid := types.EventStringAccInput(byteAddr)
	subscribe(t, con, eid)
	defer func() {
		unsubscribe(t, con, eid)
		con.Close()
	}()
	amt := uint64(100)
	toAddr := []byte{20, 143, 25, 63, 16, 177, 83, 29, 91, 91, 54, 23, 233, 46, 190, 121, 122, 34, 86, 54}
	// broadcast the transaction, wait to hear about it
	waitForEvent(t, con, eid, true, func() {
		broadcastTx(t, "JSONRPC", byteAddr, toAddr, nil, byteKey, amt, 0, 0)
	}, func(eid string, b []byte) error {
		return nil
	})
	// but make sure we don't hear about it twice
	waitForEvent(t, con, eid, false, func() {
	}, func(eid string, b []byte) error {
		return nil
	})
}

// create a contract and send it a msg, validate the return
func TestWSCall(t *testing.T) {
	byteAddr, _ := hex.DecodeString(userAddr)
	con := newWSCon(t)
	eid := types.EventStringAccInput(byteAddr)
	subscribe(t, con, eid)
	defer func() {
		unsubscribe(t, con, eid)
		con.Close()
	}()
	amt := uint64(10000)
	code, returnCode, returnVal := simpleCallContract()
	var contractAddr []byte
	// wait for the contract to be created
	waitForEvent(t, con, eid, true, func() {
		_, receipt := broadcastTx(t, "JSONRPC", byteAddr, nil, code, byteKey, amt, 1000, 1000)
		contractAddr = receipt.ContractAddr

	}, func(eid string, b []byte) error {
		// unmarshall and assert somethings
		var response struct {
			Event string
			Data  struct {
				Tx        types.CallTx
				Return    []byte
				Exception string
			}
			Error string
		}
		var err error
		binary.ReadJSON(&response, b, &err)
		if err != nil {
			return err
		}
		if response.Error != "" {
			return fmt.Errorf(response.Error)
		}
		if response.Data.Exception != "" {
			return fmt.Errorf(response.Data.Exception)
		}
		tx := response.Data.Tx
		if bytes.Compare(tx.Input.Address, byteAddr) != 0 {
			return fmt.Errorf("Senders do not match up! Got %x, expected %x", tx.Input.Address, byteAddr)
		}
		if tx.Input.Amount != amt {
			return fmt.Errorf("Amt does not match up! Got %d, expected %d", tx.Input.Amount, amt)
		}
		ret := response.Data.Return
		if bytes.Compare(ret, returnCode) != 0 {
			return fmt.Errorf("Create did not return correct byte code for new contract. Got %x, expected %x", ret, returnCode)
		}
		return nil
	})

	// get the return value from a call
	data := []byte{0x1} // just needs to be non empty for this to be a CallTx
	waitForEvent(t, con, eid, true, func() {
		broadcastTx(t, "JSONRPC", byteAddr, contractAddr, data, byteKey, amt, 1000, 1000)
	}, func(eid string, b []byte) error {
		// unmarshall and assert somethings
		var response struct {
			Event string
			Data  struct {
				Tx        types.CallTx
				Return    []byte
				Exception string
			}
			Error string
		}
		var err error
		binary.ReadJSON(&response, b, &err)
		if err != nil {
			return err
		}
		if response.Error != "" {
			return fmt.Errorf(response.Error)
		}
		ret := response.Data.Return
		if bytes.Compare(ret, returnVal) != 0 {
			return fmt.Errorf("Call did not return correctly. Got %x, expected %x", ret, returnVal)
		}
		return nil
	})
}
