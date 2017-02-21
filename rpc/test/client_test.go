package rpctest

import (
	"bytes"
	crand "crypto/rand"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	_, err := GetURIClient().Call("status", map[string]interface{}{}, tmResult)
	require.Nil(t, err)
	testStatus(t, tmResult)
}

func TestJSONStatus(t *testing.T) {
	tmResult := new(ctypes.TMResult)
	_, err := GetJSONClient().Call("status", []interface{}{}, tmResult)
	require.Nil(t, err)
	testStatus(t, tmResult)
}

func testStatus(t *testing.T, statusI interface{}) {
	chainID := GetConfig().GetString("chain_id")

	tmRes := statusI.(*ctypes.TMResult)
	status := (*tmRes).(*ctypes.ResultStatus)
	assert.Equal(t, chainID, status.NodeInfo.Network)
}

//--------------------------------------------------------------------------------
// broadcast tx sync

// random bytes (excluding byte('='))
func randBytes(t *testing.T) []byte {
	n := rand.Intn(10) + 2
	buf := make([]byte, n)
	_, err := crand.Read(buf)
	require.Nil(t, err)
	return bytes.Replace(buf, []byte("="), []byte{100}, -1)
}

func TestURIBroadcastTxSync(t *testing.T) {
	config.Set("block_size", 0)
	defer config.Set("block_size", -1)
	tmResult := new(ctypes.TMResult)
	tx := randBytes(t)
	_, err := GetURIClient().Call("broadcast_tx_sync", map[string]interface{}{"tx": tx}, tmResult)
	require.Nil(t, err)
	testBroadcastTxSync(t, tmResult, tx)
}

func TestJSONBroadcastTxSync(t *testing.T) {
	config.Set("block_size", 0)
	defer config.Set("block_size", -1)
	tmResult := new(ctypes.TMResult)
	tx := randBytes(t)
	_, err := GetJSONClient().Call("broadcast_tx_sync", []interface{}{tx}, tmResult)
	require.Nil(t, err)
	testBroadcastTxSync(t, tmResult, tx)
}

func testBroadcastTxSync(t *testing.T, resI interface{}, tx []byte) {
	tmRes := resI.(*ctypes.TMResult)
	res := (*tmRes).(*ctypes.ResultBroadcastTx)
	require.Equal(t, abci.CodeType_OK, res.Code)
	mem := node.MempoolReactor().Mempool
	require.Equal(t, 1, mem.Size())
	txs := mem.Reap(1)
	require.EqualValues(t, tx, txs[0])
	mem.Flush()
}

//--------------------------------------------------------------------------------
// query

func testTxKV(t *testing.T) ([]byte, []byte, []byte) {
	k := randBytes(t)
	v := randBytes(t)
	return k, v, []byte(Fmt("%s=%s", k, v))
}

func sendTx(t *testing.T) ([]byte, []byte) {
	tmResult := new(ctypes.TMResult)
	k, v, tx := testTxKV(t)
	_, err := GetJSONClient().Call("broadcast_tx_commit", []interface{}{tx}, tmResult)
	require.Nil(t, err)
	return k, v
}

func TestURIABCIQuery(t *testing.T) {
	k, v := sendTx(t)
	time.Sleep(time.Second)
	tmResult := new(ctypes.TMResult)
	_, err := GetURIClient().Call("abci_query", map[string]interface{}{"path": "", "data": k, "prove": false}, tmResult)
	require.Nil(t, err)
	testABCIQuery(t, tmResult, v)
}

func TestJSONABCIQuery(t *testing.T) {
	k, v := sendTx(t)
	tmResult := new(ctypes.TMResult)
	_, err := GetJSONClient().Call("abci_query", []interface{}{"", k, false}, tmResult)
	require.Nil(t, err)
	testABCIQuery(t, tmResult, v)
}

func testABCIQuery(t *testing.T, statusI interface{}, value []byte) {
	tmRes := statusI.(*ctypes.TMResult)
	resQuery := (*tmRes).(*ctypes.ResultABCIQuery)
	require.EqualValues(t, 0, resQuery.Response.Code)

	// XXX: specific to value returned by the dummy
	require.NotEqual(t, 0, len(resQuery.Response.Value))
}

//--------------------------------------------------------------------------------
// broadcast tx commit

func TestURIBroadcastTxCommit(t *testing.T) {
	tmResult := new(ctypes.TMResult)
	tx := randBytes(t)
	_, err := GetURIClient().Call("broadcast_tx_commit", map[string]interface{}{"tx": tx}, tmResult)
	require.Nil(t, err)
	testBroadcastTxCommit(t, tmResult, tx)
}

func TestJSONBroadcastTxCommit(t *testing.T) {
	tmResult := new(ctypes.TMResult)
	tx := randBytes(t)
	_, err := GetJSONClient().Call("broadcast_tx_commit", []interface{}{tx}, tmResult)
	require.Nil(t, err)
	testBroadcastTxCommit(t, tmResult, tx)
}

func testBroadcastTxCommit(t *testing.T, resI interface{}, tx []byte) {
	require := require.New(t)
	tmRes := resI.(*ctypes.TMResult)
	res := (*tmRes).(*ctypes.ResultBroadcastTxCommit)
	checkTx := res.CheckTx
	require.Equal(abci.CodeType_OK, checkTx.Code)
	deliverTx := res.DeliverTx
	require.Equal(abci.CodeType_OK, deliverTx.Code)
	mem := node.MempoolReactor().Mempool
	require.Equal(0, mem.Size())
	// TODO: find tx in block
}

//--------------------------------------------------------------------------------
// Test the websocket service

var wsTyp = "JSONRPC"

// make a simple connection to the server
func TestWSConnect(t *testing.T) {
	wsc := GetWSClient()
	wsc.Stop()
}

// receive a new block message
func TestWSNewBlock(t *testing.T) {
	wsc := GetWSClient()
	eid := types.EventStringNewBlock()
	require.Nil(t, wsc.Subscribe(eid))

	defer func() {
		require.Nil(t, wsc.Unsubscribe(eid))
		wsc.Stop()
	}()
	waitForEvent(t, wsc, eid, true, func() {}, func(eid string, b interface{}) error {
		// fmt.Println("Check:", b)
		return nil
	})
}

// receive a few new block messages in a row, with increasing height
func TestWSBlockchainGrowth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	wsc := GetWSClient()
	eid := types.EventStringNewBlock()
	require.Nil(t, wsc.Subscribe(eid))

	defer func() {
		require.Nil(t, wsc.Unsubscribe(eid))
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
	require := require.New(t)
	wsc := GetWSClient()
	tx := randBytes(t)

	// listen for the tx I am about to submit
	eid := types.EventStringTx(types.Tx(tx))
	require.Nil(wsc.Subscribe(eid))

	defer func() {
		require.Nil(wsc.Unsubscribe(eid))
		wsc.Stop()
	}()

	// send an tx
	tmResult := new(ctypes.TMResult)
	_, err := GetJSONClient().Call("broadcast_tx_sync", []interface{}{tx}, tmResult)
	require.Nil(err)

	waitForEvent(t, wsc, eid, true, func() {}, func(eid string, b interface{}) error {
		evt, ok := b.(types.EventDataTx)
		require.True(ok, "Got wrong event type: %#v", b)
		require.Equal(tx, []byte(evt.Tx), "Returned different tx")
		require.Equal(abci.CodeType_OK, evt.Code)
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
		_, err := GetURIClient().Call("unsafe_set_config", map[string]interface{}{
			"type":  testCase[0],
			"key":   testCase[1],
			"value": testCase[2],
		}, tmResult)
		require.Nil(t, err)
	}
	testUnsafeSetConfig(t)
}

func TestJSONUnsafeSetConfig(t *testing.T) {
	for _, testCase := range testCasesUnsafeSetConfig {
		tmResult := new(ctypes.TMResult)
		_, err := GetJSONClient().Call("unsafe_set_config", []interface{}{testCase[0], testCase[1], testCase[2]}, tmResult)
		require.Nil(t, err)
	}
	testUnsafeSetConfig(t)
}

func testUnsafeSetConfig(t *testing.T) {
	require := require.New(t)
	s := config.GetString("key1")
	require.Equal(stringVal, s)

	i := config.GetInt("key2")
	require.Equal(intVal, i)

	b := config.GetBool("key3")
	require.Equal(boolVal, b)
}
