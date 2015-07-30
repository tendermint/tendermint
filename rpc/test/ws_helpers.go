package rpctest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/tendermint/tendermint/Godeps/_workspace/src/github.com/gorilla/websocket"
	_ "github.com/tendermint/tendermint/config/tendermint_test"
	"github.com/tendermint/tendermint/rpc/server"
	"github.com/tendermint/tendermint/rpc/types"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/wire"
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
		Id:      "",
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
		Id:      "",
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

	// Write pings repeatedly
	// TODO: Maybe move this out to something that manages the con?
	go func() {
		pingTicker := time.NewTicker((time.Second * rpcserver.WSReadTimeoutSeconds) / 2)
		for {
			select {
			case <-quitCh:
				pingTicker.Stop()
				return
			case <-pingTicker.C:
				con.WriteControl(websocket.PingMessage, []byte("whatevs"), time.Now().Add(time.Second))
			}
		}
	}()

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
				var response struct {
					Result rpctypes.RPCEventResult `json:"result"`
				}
				if err := json.Unmarshal(p, &response); err != nil {
					errCh <- err
					break
				}
				if response.Result.Event == eventid {
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
			}
		} else {
			con.Close()
			t.Fatalf("%s event was not expected", eventid)
		}
	case err := <-errCh:
		t.Fatal(err)
	}
}

//--------------------------------------------------------------------------------

func unmarshalResponseNewBlock(b []byte) (*types.Block, error) {
	// unmarshall and assert somethings
	var response struct {
		JSONRPC string `json:"jsonrpc"`
		Id      string `json:"id"`
		Result  struct {
			Event string       `json:"event"`
			Data  *types.Block `json:"data"`
		} `json:"result"`
		Error string `json:"error"`
	}
	var err error
	wire.ReadJSON(&response, b, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	block := response.Result.Data
	return block, nil
}

func unmarshalResponseNameReg(b []byte) (*types.NameTx, error) {
	// unmarshall and assert somethings
	var response struct {
		JSONRPC string `json:"jsonrpc"`
		Id      string `json:"id"`
		Result  struct {
			Event string        `json:"event"`
			Data  *types.NameTx `json:"data"`
		} `json:"result"`
		Error string `json:"error"`
	}
	var err error
	wire.ReadJSON(&response, b, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	tx := response.Result.Data
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
		var response struct {
			JSONRPC string `json:"jsonrpc"`
			Id      string `json:"id"`
			Result  struct {
				Event string        `json:"event"`
				Data  *types.SendTx `json:"data"`
			} `json:"result"`
			Error string `json:"error"`
		}
		var err error
		wire.ReadJSON(&response, b, &err)
		if err != nil {
			return err
		}
		if response.Error != "" {
			return fmt.Errorf(response.Error)
		}
		if eid != response.Result.Event {
			return fmt.Errorf("Eventid is not correct. Got %s, expected %s", response.Result.Event, eid)
		}
		tx := response.Result.Data
		if bytes.Compare(tx.Inputs[0].Address, user[0].Address) != 0 {
			return fmt.Errorf("Senders do not match up! Got %x, expected %x", tx.Inputs[0].Address, user[0].Address)
		}
		if tx.Inputs[0].Amount != amt {
			return fmt.Errorf("Amt does not match up! Got %d, expected %d", tx.Inputs[0].Amount, amt)
		}
		if bytes.Compare(tx.Outputs[0].Address, toAddr) != 0 {
			return fmt.Errorf("Receivers do not match up! Got %x, expected %x", tx.Outputs[0].Address, user[0].Address)
		}
		return nil
	}
}

func unmarshalValidateCall(amt int64, returnCode []byte) func(string, []byte) error {
	return func(eid string, b []byte) error {
		// unmarshall and assert somethings
		var response struct {
			JSONRPC string `json:"jsonrpc"`
			Id      string `json:"id"`
			Result  struct {
				Event string `json:"event"`
				Data  struct {
					Tx        types.CallTx `json:"tx"`
					Return    []byte       `json:"return"`
					Exception string       `json:"exception"`
				} `json:"data"`
			} `json:"result"`
			Error string `json:"error"`
		}
		var err error
		wire.ReadJSON(&response, b, &err)
		if err != nil {
			return err
		}
		if response.Error != "" {
			return fmt.Errorf(response.Error)
		}
		if response.Result.Data.Exception != "" {
			return fmt.Errorf(response.Result.Data.Exception)
		}
		tx := response.Result.Data.Tx
		if bytes.Compare(tx.Input.Address, user[0].Address) != 0 {
			return fmt.Errorf("Senders do not match up! Got %x, expected %x", tx.Input.Address, user[0].Address)
		}
		if tx.Input.Amount != amt {
			return fmt.Errorf("Amt does not match up! Got %d, expected %d", tx.Input.Amount, amt)
		}
		ret := response.Result.Data.Return
		if bytes.Compare(ret, returnCode) != 0 {
			return fmt.Errorf("Call did not return correctly. Got %x, expected %x", ret, returnCode)
		}
		return nil
	}
}

func unmarshalValidateCallCall(origin, returnCode []byte, txid *[]byte) func(string, []byte) error {
	return func(eid string, b []byte) error {
		// unmarshall and assert somethings
		var response struct {
			JSONRPC string `json:"jsonrpc"`
			Id      string `json:"id"`
			Result  struct {
				Event string             `json:"event"`
				Data  types.EventMsgCall `json:"data"`
			} `json:"result"`
			Error string `json:"error"`
		}
		var err error
		wire.ReadJSON(&response, b, &err)
		if err != nil {
			return err
		}
		if response.Error != "" {
			return fmt.Errorf(response.Error)
		}
		if response.Result.Data.Exception != "" {
			return fmt.Errorf(response.Result.Data.Exception)
		}
		if bytes.Compare(response.Result.Data.Origin, origin) != 0 {
			return fmt.Errorf("Origin does not match up! Got %x, expected %x", response.Result.Data.Origin, origin)
		}
		ret := response.Result.Data.Return
		if bytes.Compare(ret, returnCode) != 0 {
			return fmt.Errorf("Call did not return correctly. Got %x, expected %x", ret, returnCode)
		}
		if bytes.Compare(response.Result.Data.TxID, *txid) != 0 {
			return fmt.Errorf("TxIDs do not match up! Got %x, expected %x", response.Result.Data.TxID, *txid)
		}
		return nil
	}
}
