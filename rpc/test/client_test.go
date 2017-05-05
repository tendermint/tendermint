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
	"github.com/tendermint/go-wire/data"
	. "github.com/tendermint/tmlibs/common"

	"github.com/tendermint/tendermint/rpc/core"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpc "github.com/tendermint/tendermint/rpc/lib/client"
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
	moniker := GetConfig().Moniker
	result := new(ctypes.ResultStatus)
	_, err := client.Call("status", map[string]interface{}{}, result)
	require.Nil(t, err)
	assert.Equal(t, moniker, result.NodeInfo.Moniker)
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
	mem := node.MempoolReactor().Mempool
	initMemSize := mem.Size()
	result := new(ctypes.ResultBroadcastTx)
	tx := randBytes(t)
	_, err := client.Call("broadcast_tx_sync", map[string]interface{}{"tx": tx}, result)
	require.Nil(t, err)

	require.Equal(t, abci.CodeType_OK, result.Code)
	require.Equal(t, initMemSize+1, mem.Size())
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
	result := new(ctypes.ResultBroadcastTxCommit)
	k, v, tx := testTxKV(t)
	_, err := client.Call("broadcast_tx_commit", map[string]interface{}{"tx": tx}, result)
	require.Nil(t, err)
	require.NotNil(t, 0, result.DeliverTx, "%#v", result)
	require.EqualValues(t, 0, result.CheckTx.Code, "%#v", result)
	require.EqualValues(t, 0, result.DeliverTx.Code, "%#v", result)
	return k, v
}

func TestURIABCIQuery(t *testing.T) {
	testABCIQuery(t, GetURIClient())
}

func TestJSONABCIQuery(t *testing.T) {
	testABCIQuery(t, GetJSONClient())
}

func testABCIQuery(t *testing.T, client rpc.HTTPClient) {
	k, _ := sendTx(t, client)
	time.Sleep(time.Millisecond * 500)
	result := new(ctypes.ResultABCIQuery)
	_, err := client.Call("abci_query",
		map[string]interface{}{"path": "", "data": data.Bytes(k), "prove": false}, result)
	require.Nil(t, err)

	require.EqualValues(t, 0, result.Code)

	// XXX: specific to value returned by the dummy
	require.NotEqual(t, 0, len(result.Value))
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

	result := new(ctypes.ResultBroadcastTxCommit)
	tx := randBytes(t)
	_, err := client.Call("broadcast_tx_commit", map[string]interface{}{"tx": tx}, result)
	require.Nil(err)

	checkTx := result.CheckTx
	require.Equal(abci.CodeType_OK, checkTx.Code)
	deliverTx := result.DeliverTx
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
	defer core.SetTxIndexer(node.ConsensusState().GetState().TxIndexer)

	testTx(t, GetURIClient(), false)
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
	result := new(ctypes.ResultBroadcastTxCommit)
	txBytes := randBytes(t)
	tx := types.Tx(txBytes)
	_, err := client.Call("broadcast_tx_commit", map[string]interface{}{"tx": txBytes}, result)
	require.Nil(err)

	checkTx := result.CheckTx
	require.Equal(abci.CodeType_OK, checkTx.Code)
	deliverTx := result.DeliverTx
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
		result2 := new(ctypes.ResultTx)
		query := map[string]interface{}{
			"hash":  tc.hash,
			"prove": tc.prove,
		}
		_, err = client.Call("tx", query, result2)
		valid := (withIndexer && tc.valid)
		if !valid {
			require.NotNil(err, idx)
		} else {
			require.Nil(err, idx)
			assert.Equal(tx, result2.Tx, idx)
			assert.Equal(result.Height, result2.Height, idx)
			assert.Equal(0, result2.Index, idx)
			assert.Equal(abci.CodeType_OK, result2.TxResult.Code, idx)
			// time to verify the proof
			proof := result2.Proof
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
			block := eventData.(types.TMEventData).Unwrap().(types.EventDataNewBlock).Block
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
	result := new(ctypes.ResultBroadcastTx)
	_, err := GetJSONClient().Call("broadcast_tx_sync", map[string]interface{}{"tx": tx}, result)
	require.Nil(err)

	waitForEvent(t, wsc, eid, true, func() {}, func(eid string, b interface{}) error {
		evt, ok := b.(types.TMEventData).Unwrap().(types.EventDataTx)
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
