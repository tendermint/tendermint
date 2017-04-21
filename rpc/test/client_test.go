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
	rpc "github.com/tendermint/go-rpc/client"
	"github.com/tendermint/tendermint/rpc/core"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/state/txindex/null"
	"github.com/tendermint/tendermint/types"
)

//--------------------------------------------------------------------------------
// Test the HTTP client
// These tests assume the dummy app
//--------------------------------------------------------------------------------

//--------------------------------------------------------------------------------
// status

func TestURIStatus(t *testing.T) {
	testStatus(t, GetURIClient())
}

func TestJSONStatus(t *testing.T) {
	testStatus(t, GetJSONClient())
}

func testStatus(t *testing.T, client rpc.HTTPClient) {
	chainID := GetConfig().GetString("chain_id")
	tmResult := new(ctypes.TMResult)
	_, err := client.Call("status", map[string]interface{}{}, tmResult)
	require.Nil(t, err)

	status := (*tmResult).(*ctypes.ResultStatus)
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
	testBroadcastTxSync(t, GetURIClient())
}

func TestJSONBroadcastTxSync(t *testing.T) {
	testBroadcastTxSync(t, GetJSONClient())
}

func testBroadcastTxSync(t *testing.T, client rpc.HTTPClient) {
	config.Set("block_size", 0)
	defer config.Set("block_size", -1)
	tmResult := new(ctypes.TMResult)
	tx := randBytes(t)
	_, err := client.Call("broadcast_tx_sync", map[string]interface{}{"tx": tx}, tmResult)
	require.Nil(t, err)

	res := (*tmResult).(*ctypes.ResultBroadcastTx)
	require.Equal(t, abci.CodeType_OK, res.Code)
	mem := node.MempoolReactor().Mempool
	require.Equal(t, 1, mem.Size())
	txs := mem.Reap(1)
	require.EqualValues(t, tx, txs[0])
	mem.Flush()
}

//--------------------------------------------------------------------------------
// query

func testTxKV(t *testing.T) ([]byte, []byte, types.Tx) {
	k := randBytes(t)
	v := randBytes(t)
	return k, v, types.Tx(Fmt("%s=%s", k, v))
}

func sendTx(t *testing.T, client rpc.HTTPClient) ([]byte, []byte) {
	tmResult := new(ctypes.TMResult)
	k, v, tx := testTxKV(t)
	txBytes := []byte(tx) // XXX
	_, err := client.Call("broadcast_tx_commit", map[string]interface{}{"tx": txBytes}, tmResult)
	require.Nil(t, err)
	return k, v
}

func TestURIABCIQuery(t *testing.T) {
	testABCIQuery(t, GetURIClient())
}

func TestJSONABCIQuery(t *testing.T) {
	testABCIQuery(t, GetURIClient())
}

func testABCIQuery(t *testing.T, client rpc.HTTPClient) {
	k, _ := sendTx(t, client)
	time.Sleep(time.Millisecond * 500)
	tmResult := new(ctypes.TMResult)
	_, err := client.Call("abci_query",
		map[string]interface{}{"path": "", "data": k, "prove": false}, tmResult)
	require.Nil(t, err)

	resQuery := (*tmResult).(*ctypes.ResultABCIQuery)
	require.EqualValues(t, 0, resQuery.Response.Code)

	// XXX: specific to value returned by the dummy
	require.NotEqual(t, 0, len(resQuery.Response.Value))
}

//--------------------------------------------------------------------------------
// broadcast tx commit

func TestURIBroadcastTxCommit(t *testing.T) {
	testBroadcastTxCommit(t, GetURIClient())
}

func TestJSONBroadcastTxCommit(t *testing.T) {
	testBroadcastTxCommit(t, GetJSONClient())
}

func testBroadcastTxCommit(t *testing.T, client rpc.HTTPClient) {
	require := require.New(t)

	tmResult := new(ctypes.TMResult)
	tx := randBytes(t)
	_, err := client.Call("broadcast_tx_commit", map[string]interface{}{"tx": tx}, tmResult)
	require.Nil(err)

	res := (*tmResult).(*ctypes.ResultBroadcastTxCommit)
	checkTx := res.CheckTx
	require.Equal(abci.CodeType_OK, checkTx.Code)
	deliverTx := res.DeliverTx
	require.Equal(abci.CodeType_OK, deliverTx.Code)
	mem := node.MempoolReactor().Mempool
	require.Equal(0, mem.Size())
	// TODO: find tx in block
}

//--------------------------------------------------------------------------------
// query tx

func TestURITx(t *testing.T) {
	testTx(t, GetURIClient(), true)

	core.SetTxIndexer(&null.TxIndex{})
	testTx(t, GetJSONClient(), false)
	core.SetTxIndexer(node.ConsensusState().GetState().TxIndexer)
}

func TestJSONTx(t *testing.T) {
	testTx(t, GetJSONClient(), true)

	core.SetTxIndexer(&null.TxIndex{})
	testTx(t, GetJSONClient(), false)
	core.SetTxIndexer(node.ConsensusState().GetState().TxIndexer)
}

func testTx(t *testing.T, client rpc.HTTPClient, withIndexer bool) {
	assert, require := assert.New(t), require.New(t)

	// first we broadcast a tx
	tmResult := new(ctypes.TMResult)
	txBytes := randBytes(t)
	tx := types.Tx(txBytes)
	_, err := client.Call("broadcast_tx_commit", map[string]interface{}{"tx": txBytes}, tmResult)
	require.Nil(err)

	res := (*tmResult).(*ctypes.ResultBroadcastTxCommit)
	checkTx := res.CheckTx
	require.Equal(abci.CodeType_OK, checkTx.Code)
	deliverTx := res.DeliverTx
	require.Equal(abci.CodeType_OK, deliverTx.Code)
	mem := node.MempoolReactor().Mempool
	require.Equal(0, mem.Size())

	txHash := tx.Hash()
	txHash2 := types.Tx("a different tx").Hash()

	cases := []struct {
		valid bool
		hash  []byte
		prove bool
	}{
		// only valid if correct hash provided
		{true, txHash, false},
		{true, txHash, true},
		{false, txHash2, false},
		{false, txHash2, true},
		{false, nil, false},
		{false, nil, true},
	}

	for i, tc := range cases {
		idx := fmt.Sprintf("%d", i)

		// now we query for the tx.
		// since there's only one tx, we know index=0.
		tmResult = new(ctypes.TMResult)
		query := map[string]interface{}{
			"hash":  tc.hash,
			"prove": tc.prove,
		}
		_, err = client.Call("tx", query, tmResult)
		valid := (withIndexer && tc.valid)
		if !valid {
			require.NotNil(err, idx)
		} else {
			require.Nil(err, idx)
			res2 := (*tmResult).(*ctypes.ResultTx)
			assert.Equal(tx, res2.Tx, idx)
			assert.Equal(res.Height, res2.Height, idx)
			assert.Equal(0, res2.Index, idx)
			assert.Equal(abci.CodeType_OK, res2.TxResult.Code, idx)
			// time to verify the proof
			proof := res2.Proof
			if tc.prove && assert.Equal(tx, proof.Data, idx) {
				assert.True(proof.Proof.Verify(proof.Index, proof.Total, tx.Hash(), proof.RootHash), idx)
			}
		}
	}

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
	_, err := GetJSONClient().Call("broadcast_tx_sync", map[string]interface{}{"tx": tx}, tmResult)
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
		_, err := GetJSONClient().Call("unsafe_set_config",
			map[string]interface{}{"type": testCase[0], "key": testCase[1], "value": testCase[2]},
			tmResult)
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
