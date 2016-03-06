package rpctest

import (
	"fmt"
	"testing"

	"github.com/tendermint/tendermint/config/tendermint_test"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

func init() {
	tendermint_test.ResetConfig("rpc_test_client_test")
	initGlobalVariables()
}

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

/*func TestURIBroadcastTx(t *testing.T) {
	testBroadcastTx(t, "HTTP")
}*/

/*func TestJSONBroadcastTx(t *testing.T) {
	testBroadcastTx(t, "JSONRPC")
}*/

// TODO
/*
func testBroadcastTx(t *testing.T, typ string) {
	amt := int64(100)
	toAddr := user[1].Address
	tx := makeDefaultSendTxSigned(t, typ, toAddr, amt)
	receipt := broadcastTx(t, typ, tx)
	if receipt.CreatesContract > 0 {
		t.Fatal("This tx does not create a contract")
	}
	if len(receipt.TxHash) == 0 {
		t.Fatal("Failed to compute tx hash")
	}
	pool := node.MempoolReactor().Mempool
	txs := pool.GetProposalTxs()
	if len(txs) != mempoolCount {
		t.Fatalf("The mem pool has %d txs. Expected %d", len(txs), mempoolCount)
	}
	tx2 := txs[mempoolCount-1].(*types.SendTx)
	n, err := new(int64), new(error)
	buf1, buf2 := new(bytes.Buffer), new(bytes.Buffer)
	tx.WriteSignBytes(chainID, buf1, n, err)
	tx2.WriteSignBytes(chainID, buf2, n, err)
	if bytes.Compare(buf1.Bytes(), buf2.Bytes()) != 0 {
		t.Fatal("inconsistent hashes for mempool tx and sent tx")
	}
}*/

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
