package rpctest

import (
	"bytes"
	crand "crypto/rand"
	"fmt"
	"math/rand"
	"testing"
	"time"

	abci "github.com/tendermint/abci/types"
	. "github.com/tendermint/go-common"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

//--------------------------------------------------------------------------------
// Test the HTTP client
// These tests assume the dummy app
//--------------------------------------------------------------------------------

//--------------------------------------------------------------------------------
// status

func TestURIStatus(t *testing.T) {
	tmResult := new(ctypes.TMResult)
	_, err := clientURI.Call("status", map[string]interface{}{}, tmResult)
	if err != nil {
		panic(err)
	}
	testStatus(t, tmResult)
}

func TestJSONStatus(t *testing.T) {
	tmResult := new(ctypes.TMResult)
	_, err := clientJSON.Call("status", []interface{}{}, tmResult)
	if err != nil {
		panic(err)
	}
	testStatus(t, tmResult)
}

func testStatus(t *testing.T, statusI interface{}) {
	tmRes := statusI.(*ctypes.TMResult)
	status := (*tmRes).(*ctypes.ResultStatus)
	if status.NodeInfo.Network != chainID {
		panic(Fmt("ChainID mismatch: got %s expected %s",
			status.NodeInfo.Network, chainID))
	}
}

//--------------------------------------------------------------------------------
// broadcast tx sync

// random bytes (excluding byte('='))
func randBytes() []byte {
	n := rand.Intn(10) + 2
	buf := make([]byte, n)
	_, err := crand.Read(buf)
	if err != nil {
		panic(err)
	}
	return bytes.Replace(buf, []byte("="), []byte{100}, -1)
}

func TestURIBroadcastTxSync(t *testing.T) {
	config.Set("block_size", 0)
	defer config.Set("block_size", -1)
	tmResult := new(ctypes.TMResult)
	tx := randBytes()
	_, err := clientURI.Call("broadcast_tx_sync", map[string]interface{}{"tx": tx}, tmResult)
	if err != nil {
		panic(err)
	}
	testBroadcastTxSync(t, tmResult, tx)
}

func TestJSONBroadcastTxSync(t *testing.T) {
	config.Set("block_size", 0)
	defer config.Set("block_size", -1)
	tmResult := new(ctypes.TMResult)
	tx := randBytes()
	_, err := clientJSON.Call("broadcast_tx_sync", []interface{}{tx}, tmResult)
	if err != nil {
		panic(err)
	}
	testBroadcastTxSync(t, tmResult, tx)
}

func testBroadcastTxSync(t *testing.T, resI interface{}, tx []byte) {
	tmRes := resI.(*ctypes.TMResult)
	res := (*tmRes).(*ctypes.ResultBroadcastTx)
	if res.Code != abci.CodeType_OK {
		panic(Fmt("BroadcastTxSync got non-zero exit code: %v. %X; %s", res.Code, res.Data, res.Log))
	}
	mem := node.MempoolReactor().Mempool
	if mem.Size() != 1 {
		panic(Fmt("Mempool size should have been 1. Got %d", mem.Size()))
	}

	txs := mem.Reap(1)
	if !bytes.Equal(txs[0], tx) {
		panic(Fmt("Tx in mempool does not match test tx. Got %X, expected %X", txs[0], tx))
	}

	mem.Flush()
}

//--------------------------------------------------------------------------------
// query

func testTxKV() ([]byte, []byte, []byte) {
	k := randBytes()
	v := randBytes()
	return k, v, []byte(Fmt("%s=%s", k, v))
}

func sendTx() ([]byte, []byte) {
	tmResult := new(ctypes.TMResult)
	k, v, tx := testTxKV()
	_, err := clientJSON.Call("broadcast_tx_commit", []interface{}{tx}, tmResult)
	if err != nil {
		panic(err)
	}
	fmt.Println("SENT TX", tx)
	fmt.Printf("SENT TX %X\n", tx)
	fmt.Printf("k %X; v %X", k, v)
	return k, v
}

func TestURIABCIQuery(t *testing.T) {
	k, v := sendTx()
	time.Sleep(time.Second)
	tmResult := new(ctypes.TMResult)
	_, err := clientURI.Call("abci_query", map[string]interface{}{"path": "", "data": k, "prove": false}, tmResult)
	if err != nil {
		panic(err)
	}
	testABCIQuery(t, tmResult, v)
}

func TestJSONABCIQuery(t *testing.T) {
	k, v := sendTx()
	tmResult := new(ctypes.TMResult)
	_, err := clientJSON.Call("abci_query", []interface{}{"", k, false}, tmResult)
	if err != nil {
		panic(err)
	}
	testABCIQuery(t, tmResult, v)
}

func testABCIQuery(t *testing.T, statusI interface{}, value []byte) {
	tmRes := statusI.(*ctypes.TMResult)
	resQuery := (*tmRes).(*ctypes.ResultABCIQuery)
	if !resQuery.Response.Code.IsOK() {
		panic(Fmt("Query returned an err: %v", resQuery))
	}

	// XXX: specific to value returned by the dummy
	if len(resQuery.Response.Value) == 0 {
		panic(Fmt("Query error. Found no value"))
	}
}

//--------------------------------------------------------------------------------
// broadcast tx commit

func TestURIBroadcastTxCommit(t *testing.T) {
	tmResult := new(ctypes.TMResult)
	tx := randBytes()
	_, err := clientURI.Call("broadcast_tx_commit", map[string]interface{}{"tx": tx}, tmResult)
	if err != nil {
		panic(err)
	}
	testBroadcastTxCommit(t, tmResult, tx)
}

func TestJSONBroadcastTxCommit(t *testing.T) {
	tmResult := new(ctypes.TMResult)
	tx := randBytes()
	_, err := clientJSON.Call("broadcast_tx_commit", []interface{}{tx}, tmResult)
	if err != nil {
		panic(err)
	}
	testBroadcastTxCommit(t, tmResult, tx)
}

func testBroadcastTxCommit(t *testing.T, resI interface{}, tx []byte) {
	tmRes := resI.(*ctypes.TMResult)
	res := (*tmRes).(*ctypes.ResultBroadcastTxCommit)
	checkTx := res.CheckTx
	if checkTx.Code != abci.CodeType_OK {
		panic(Fmt("BroadcastTxCommit got non-zero exit code from CheckTx: %v. %X; %s", checkTx.Code, checkTx.Data, checkTx.Log))
	}
	deliverTx := res.DeliverTx
	if deliverTx.Code != abci.CodeType_OK {
		panic(Fmt("BroadcastTxCommit got non-zero exit code from CheckTx: %v. %X; %s", deliverTx.Code, deliverTx.Data, deliverTx.Log))
	}
	mem := node.MempoolReactor().Mempool
	if mem.Size() != 0 {
		panic(Fmt("Mempool size should have been 0. Got %d", mem.Size()))
	}

	// TODO: find tx in block
}

//--------------------------------------------------------------------------------
// Test the websocket service

var wsTyp = "JSONRPC"

// make a simple connection to the server
func TestWSConnect(t *testing.T) {
	wsc := newWSClient(t)
	wsc.Stop()
}

// receive a new block message
func TestWSNewBlock(t *testing.T) {
	wsc := newWSClient(t)
	eid := types.EventStringNewBlock()
	subscribe(t, wsc, eid)
	defer func() {
		unsubscribe(t, wsc, eid)
		wsc.Stop()
	}()
	waitForEvent(t, wsc, eid, true, func() {}, func(eid string, b interface{}) error {
		fmt.Println("Check:", b)
		return nil
	})
}

// receive a few new block messages in a row, with increasing height
func TestWSBlockchainGrowth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	wsc := newWSClient(t)
	eid := types.EventStringNewBlock()
	subscribe(t, wsc, eid)
	defer func() {
		unsubscribe(t, wsc, eid)
		wsc.Stop()
	}()

	// listen for NewBlock, ensure height increases by 1

	var initBlockN int
	for i := 0; i < 3; i++ {
		waitForEvent(t, wsc, eid, true, func() {}, func(eid string, eventData interface{}) error {
			block := eventData.(types.EventDataNewBlock).Block
			if i == 0 {
				initBlockN = block.Header.Height
			} else {
				if block.Header.Height != initBlockN+i {
					return fmt.Errorf("Expected block %d, got block %d", initBlockN+i, block.Header.Height)
				}
			}

			return nil
		})
	}
}

func TestWSTxEvent(t *testing.T) {
	wsc := newWSClient(t)
	tx := randBytes()

	// listen for the tx I am about to submit
	eid := types.EventStringTx(types.Tx(tx))
	subscribe(t, wsc, eid)
	defer func() {
		unsubscribe(t, wsc, eid)
		wsc.Stop()
	}()

	// send an tx
	tmResult := new(ctypes.TMResult)
	_, err := clientJSON.Call("broadcast_tx_sync", []interface{}{tx}, tmResult)
	if err != nil {
		t.Fatal("Error submitting event")
	}

	waitForEvent(t, wsc, eid, true, func() {}, func(eid string, b interface{}) error {
		evt, ok := b.(types.EventDataTx)
		if !ok {
			t.Fatal("Got wrong event type", b)
		}
		if bytes.Compare([]byte(evt.Tx), tx) != 0 {
			t.Error("Event returned different tx")
		}
		if evt.Code != abci.CodeType_OK {
			t.Error("Event returned tx error code", evt.Code)
		}
		return nil
	})
}

/* TODO: this with dummy app..
func TestWSDoubleFire(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	con := newWSCon(t)
	eid := types.EventStringAccInput(user[0].Address)
	subscribe(t, con, eid)
	defer func() {
		unsubscribe(t, con, eid)
		con.Close()
	}()
	amt := int64(100)
	toAddr := user[1].Address
	// broadcast the transaction, wait to hear about it
	waitForEvent(t, con, eid, true, func() {
		tx := makeDefaultSendTxSigned(t, wsTyp, toAddr, amt)
		broadcastTx(t, wsTyp, tx)
	}, func(eid string, b []byte) error {
		return nil
	})
	// but make sure we don't hear about it twice
	waitForEvent(t, con, eid, false, func() {
	}, func(eid string, b []byte) error {
		return nil
	})
}*/

//--------------------------------------------------------------------------------
// unsafe_set_config

var stringVal = "my string"
var intVal = 987654321
var boolVal = true

// don't change these
var testCasesUnsafeSetConfig = [][]string{
	[]string{"string", "key1", stringVal},
	[]string{"int", "key2", fmt.Sprintf("%v", intVal)},
	[]string{"bool", "key3", fmt.Sprintf("%v", boolVal)},
}

func TestURIUnsafeSetConfig(t *testing.T) {
	for _, testCase := range testCasesUnsafeSetConfig {
		tmResult := new(ctypes.TMResult)
		_, err := clientURI.Call("unsafe_set_config", map[string]interface{}{
			"type":  testCase[0],
			"key":   testCase[1],
			"value": testCase[2],
		}, tmResult)
		if err != nil {
			panic(err)
		}
	}
	testUnsafeSetConfig(t)
}

func TestJSONUnsafeSetConfig(t *testing.T) {
	for _, testCase := range testCasesUnsafeSetConfig {
		tmResult := new(ctypes.TMResult)
		_, err := clientJSON.Call("unsafe_set_config", []interface{}{testCase[0], testCase[1], testCase[2]}, tmResult)
		if err != nil {
			panic(err)
		}
	}
	testUnsafeSetConfig(t)
}

func testUnsafeSetConfig(t *testing.T) {
	s := config.GetString("key1")
	if s != stringVal {
		panic(Fmt("got %v, expected %v", s, stringVal))
	}

	i := config.GetInt("key2")
	if i != intVal {
		panic(Fmt("got %v, expected %v", i, intVal))
	}

	b := config.GetBool("key3")
	if b != boolVal {
		panic(Fmt("got %v, expected %v", b, boolVal))
	}
}
