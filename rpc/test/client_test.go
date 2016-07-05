package rpctest

import (
	"bytes"
	"fmt"
	"testing"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
	tmsp "github.com/tendermint/tmsp/types"
)

//--------------------------------------------------------------------------------
// Test the HTTP client
//--------------------------------------------------------------------------------

//--------------------------------------------------------------------------------
// status

func TestURIStatus(t *testing.T) {
	tmResult := new(ctypes.TMResult)
	_, err := clientURI.Call("status", map[string]interface{}{}, tmResult)
	if err != nil {
		t.Fatal(err)
	}
	testStatus(t, tmResult)
}

func TestJSONStatus(t *testing.T) {
	tmResult := new(ctypes.TMResult)
	_, err := clientJSON.Call("status", []interface{}{}, tmResult)
	if err != nil {
		t.Fatal(err)
	}
	testStatus(t, tmResult)
}

func testStatus(t *testing.T, statusI interface{}) {
	tmRes := statusI.(*ctypes.TMResult)
	status := (*tmRes).(*ctypes.ResultStatus)
	if status.NodeInfo.Network != chainID {
		t.Fatal(fmt.Errorf("ChainID mismatch: got %s expected %s",
			status.NodeInfo.Network, chainID))
	}
}

//--------------------------------------------------------------------------------
// broadcast tx sync

var testTx = []byte{0x1, 0x2, 0x3, 0x4, 0x5}

func TestURIBroadcastTxSync(t *testing.T) {
	config.Set("block_size", 0)
	defer config.Set("block_size", -1)
	tmResult := new(ctypes.TMResult)
	_, err := clientURI.Call("broadcast_tx_sync", map[string]interface{}{"tx": testTx}, tmResult)
	if err != nil {
		t.Fatal(err)
	}
	testBroadcastTxSync(t, tmResult)
}

func TestJSONBroadcastTxSync(t *testing.T) {
	config.Set("block_size", 0)
	defer config.Set("block_size", -1)
	tmResult := new(ctypes.TMResult)
	_, err := clientJSON.Call("broadcast_tx_sync", []interface{}{testTx}, tmResult)
	if err != nil {
		t.Fatal(err)
	}
	testBroadcastTxSync(t, tmResult)
}

func testBroadcastTxSync(t *testing.T, resI interface{}) {
	tmRes := resI.(*ctypes.TMResult)
	res := (*tmRes).(*ctypes.ResultBroadcastTx)
	if res.Code != tmsp.CodeType_OK {
		t.Fatalf("BroadcastTxSync got non-zero exit code: %v. %X; %s", res.Code, res.Data, res.Log)
	}
	mem := node.MempoolReactor().Mempool
	if mem.Size() != 1 {
		t.Fatalf("Mempool size should have been 1. Got %d", mem.Size())
	}

	txs := mem.Reap(1)
	if !bytes.Equal(txs[0], testTx) {
		t.Fatalf("Tx in mempool does not match test tx. Got %X, expected %X", txs[0], testTx)
	}

	mem.Flush()
}

//--------------------------------------------------------------------------------
// broadcast tx commit

func TestURIBroadcastTxCommit(t *testing.T) {
	tmResult := new(ctypes.TMResult)
	_, err := clientURI.Call("broadcast_tx_commit", map[string]interface{}{"tx": testTx}, tmResult)
	if err != nil {
		t.Fatal(err)
	}
	testBroadcastTxCommit(t, tmResult)
}

func TestJSONBroadcastTxCommit(t *testing.T) {
	tmResult := new(ctypes.TMResult)
	_, err := clientJSON.Call("broadcast_tx_commit", []interface{}{testTx}, tmResult)
	if err != nil {
		t.Fatal(err)
	}
	testBroadcastTxCommit(t, tmResult)
}

func testBroadcastTxCommit(t *testing.T, resI interface{}) {
	tmRes := resI.(*ctypes.TMResult)
	res := (*tmRes).(*ctypes.ResultBroadcastTx)
	if res.Code != tmsp.CodeType_OK {
		t.Fatalf("BroadcastTxCommit got non-zero exit code: %v. %X; %s", res.Code, res.Data, res.Log)
	}
	mem := node.MempoolReactor().Mempool
	if mem.Size() != 0 {
		t.Fatalf("Mempool size should have been 0. Got %d", mem.Size())
	}
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
			t.Fatal(err)
		}
	}
	testUnsafeSetConfig(t)
}

func TestJSONUnsafeSetConfig(t *testing.T) {
	for _, testCase := range testCasesUnsafeSetConfig {
		tmResult := new(ctypes.TMResult)
		_, err := clientJSON.Call("unsafe_set_config", []interface{}{testCase[0], testCase[1], testCase[2]}, tmResult)
		if err != nil {
			t.Fatal(err)
		}
	}
	testUnsafeSetConfig(t)
}

func testUnsafeSetConfig(t *testing.T) {
	s := config.GetString("key1")
	if s != stringVal {
		t.Fatalf("got %v, expected %v", s, stringVal)
	}

	i := config.GetInt("key2")
	if i != intVal {
		t.Fatalf("got %v, expected %v", i, intVal)
	}

	b := config.GetBool("key3")
	if b != boolVal {
		t.Fatalf("got %v, expected %v", b, boolVal)
	}
}
