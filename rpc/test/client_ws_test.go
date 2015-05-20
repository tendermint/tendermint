package rpctest

import (
	"fmt"
	"testing"

	"github.com/tendermint/tendermint/account"
	_ "github.com/tendermint/tendermint/config/tendermint_test"
	"github.com/tendermint/tendermint/types"
)

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
	// listen for NewBlock, ensure height increases by 1
	unmarshalValidateBlockchain(t, con, eid)
}

// send a transaction and validate the events from listening for both sender and receiver
func TestWSSend(t *testing.T) {
	toAddr := []byte{20, 143, 25, 63, 16, 177, 83, 29, 91, 91, 54, 23, 233, 46, 190, 121, 122, 34, 86, 54}
	amt := uint64(100)

	con := newWSCon(t)
	eidInput := types.EventStringAccInput(userByteAddr)
	eidOutput := types.EventStringAccOutput(toAddr)
	subscribe(t, con, eidInput)
	subscribe(t, con, eidOutput)
	defer func() {
		unsubscribe(t, con, eidInput)
		unsubscribe(t, con, eidOutput)
		con.Close()
	}()
	waitForEvent(t, con, eidInput, true, func() {
		broadcastTx(t, "JSONRPC", userByteAddr, toAddr, nil, userBytePriv, amt, 0, 0)
	}, unmarshalValidateSend(amt, toAddr))
	waitForEvent(t, con, eidOutput, true, func() {}, unmarshalValidateSend(amt, toAddr))
}

// ensure events are only fired once for a given transaction
func TestWSDoubleFire(t *testing.T) {
	con := newWSCon(t)
	eid := types.EventStringAccInput(userByteAddr)
	subscribe(t, con, eid)
	defer func() {
		unsubscribe(t, con, eid)
		con.Close()
	}()
	amt := uint64(100)
	toAddr := []byte{20, 143, 25, 63, 16, 177, 83, 29, 91, 91, 54, 23, 233, 46, 190, 121, 122, 34, 86, 54}
	// broadcast the transaction, wait to hear about it
	waitForEvent(t, con, eid, true, func() {
		broadcastTx(t, "JSONRPC", userByteAddr, toAddr, nil, userBytePriv, amt, 0, 0)
	}, func(eid string, b []byte) error {
		return nil
	})
	// but make sure we don't hear about it twice
	waitForEvent(t, con, eid, false, func() {
	}, func(eid string, b []byte) error {
		return nil
	})
}

// create a contract, wait for the create event
func TestWSCreate(t *testing.T) {
	con := newWSCon(t)
	eid1 := types.EventStringAccCreate(userByteAddr)
	subscribe(t, con, eid1)
	defer func() {
		unsubscribe(t, con, eid1)
		con.Close()
	}()
	amt := uint64(10000)
	code, returnCode, _ := simpleContract()
	var contractAddr []byte
	// wait for the contract to be created
	waitForEvent(t, con, eid1, true, func() {
		_, receipt := broadcastTx(t, "JSONRPC", userByteAddr, nil, code, userBytePriv, amt, 1000, 1000)
		contractAddr = receipt.ContractAddr
	}, unmarshalValidateCreate(amt, returnCode))
}

// create a contract, wait for the input event, and send it a msg, validate the return
func TestWSCallWait(t *testing.T) {
	con := newWSCon(t)
	eid1 := types.EventStringAccInput(userByteAddr)
	subscribe(t, con, eid1)
	defer func() {
		unsubscribe(t, con, eid1)
		con.Close()
	}()
	amt := uint64(10000)
	code, returnCode, returnVal := simpleContract()
	var contractAddr []byte
	// wait for the contract to be created
	waitForEvent(t, con, eid1, true, func() {
		_, receipt := broadcastTx(t, "JSONRPC", userByteAddr, nil, code, userBytePriv, amt, 1000, 1000)
		contractAddr = receipt.ContractAddr
	}, unmarshalValidateCall(amt, returnCode))

	// susbscribe to the new contract
	amt = uint64(10001)
	eid2 := types.EventStringAccOutput(contractAddr)
	subscribe(t, con, eid2)
	defer func() {
		unsubscribe(t, con, eid2)
	}()
	// get the return value from a call
	data := []byte{0x1} // just needs to be non empty for this to be a CallTx
	waitForEvent(t, con, eid2, true, func() {
		broadcastTx(t, "JSONRPC", userByteAddr, contractAddr, data, userBytePriv, amt, 1000, 1000)
	}, unmarshalValidateCall(amt, returnVal))
}

// create a contract and send it a msg without waiting. wait for contract event
// and validate return
func TestWSCallNoWait(t *testing.T) {
	con := newWSCon(t)
	amt := uint64(10000)
	code, _, returnVal := simpleContract()

	_, receipt := broadcastTx(t, "JSONRPC", userByteAddr, nil, code, userBytePriv, amt, 1000, 1000)
	contractAddr := receipt.ContractAddr

	// susbscribe to the new contract
	amt = uint64(10001)
	eid := types.EventStringAccOutput(contractAddr)
	subscribe(t, con, eid)
	defer func() {
		unsubscribe(t, con, eid)
		con.Close()
	}()
	// get the return value from a call
	data := []byte{0x1} // just needs to be non empty for this to be a CallTx
	waitForEvent(t, con, eid, true, func() {
		broadcastTx(t, "JSONRPC", userByteAddr, contractAddr, data, userBytePriv, amt, 1000, 1000)
	}, unmarshalValidateCall(amt, returnVal))
}

// create two contracts, one of which calls the other
func TestWSCallCall(t *testing.T) {
	con := newWSCon(t)
	amt := uint64(10000)
	code, _, returnVal := simpleContract()
	txid := new([]byte)

	// deploy the two contracts
	_, receipt := broadcastTx(t, "JSONRPC", userByteAddr, nil, code, userBytePriv, amt, 1000, 1000)
	contractAddr1 := receipt.ContractAddr
	code, _, _ = simpleCallContract(contractAddr1)
	_, receipt = broadcastTx(t, "JSONRPC", userByteAddr, nil, code, userBytePriv, amt, 1000, 1000)
	contractAddr2 := receipt.ContractAddr

	// susbscribe to the new contracts
	amt = uint64(10001)
	eid1 := types.EventStringAccReceive(contractAddr1)
	subscribe(t, con, eid1)
	defer func() {
		unsubscribe(t, con, eid1)
		con.Close()
	}()
	// call contract2, which should call contract1, and wait for ev1
	data := []byte{0x1} // just needs to be non empty for this to be a CallTx

	// let the contract get created first
	waitForEvent(t, con, eid1, true, func() {
	}, func(eid string, b []byte) error {
		return nil
	})
	// call it
	waitForEvent(t, con, eid1, true, func() {
		tx, _ := broadcastTx(t, "JSONRPC", userByteAddr, contractAddr2, data, userBytePriv, amt, 1000, 1000)
		*txid = account.HashSignBytes(tx)
	}, unmarshalValidateCallCall(userByteAddr, returnVal, txid))
}
