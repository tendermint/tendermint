package rpc

import (
	"bytes"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/tendermint/tendermint/binary"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/vm"
	"testing"
)

func unmarshalValidateBlockchain(t *testing.T, con *websocket.Conn, eid string) {
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

func unmarshalValidateSend(amt uint64, toAddr []byte) func(string, []byte) error {
	return func(eid string, b []byte) error {
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
}

func unmarshalValidateCall(amt uint64, returnCode []byte) func(string, []byte) error {
	return func(eid string, b []byte) error {
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
			return fmt.Errorf("Call did not return correctly. Got %x, expected %x", ret, returnCode)
		}
		return nil
	}
}

func unmarshalValidateCallCall(origin, returnCode []byte, txid *[]byte) func(string, []byte) error {
	return func(eid string, b []byte) error {
		// unmarshall and assert somethings
		var response struct {
			Event string
			Data  struct {
				CallData  *vm.CallData
				Origin    []byte
				TxId      []byte
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
		if bytes.Compare(response.Data.Origin, origin) != 0 {
			return fmt.Errorf("Origin does not match up! Got %x, expected %x", response.Data.Origin, origin)
		}
		ret := response.Data.Return
		if bytes.Compare(ret, returnCode) != 0 {
			return fmt.Errorf("Call did not return correctly. Got %x, expected %x", ret, returnCode)
		}
		if bytes.Compare(response.Data.TxId, *txid) != 0 {
			return fmt.Errorf("TxIds do not match up! Got %x, expected %x", response.Data.TxId, *txid)
		}
		// calldata := response.Data.CallData
		return nil
	}
}
