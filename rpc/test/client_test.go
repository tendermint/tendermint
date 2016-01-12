package rpctest

import (
	"fmt"
	"testing"

	_ "github.com/tendermint/tendermint/config/tendermint_test"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

//--------------------------------------------------------------------------------
// Test the HTTP client

func TestURIStatus(t *testing.T) {
	result, err := clientURI.Call("status", map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	testStatus(t, result)
}

func TestJSONStatus(t *testing.T) {
	result, err := clientJSON.Call("status", []interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	testStatus(t, result)
}

func testStatus(t *testing.T, result interface{}) {
	status := result.(*ctypes.TendermintResult).Result.(*ctypes.ResultStatus)
	if status.NodeInfo.Network != chainID {
		t.Fatal(fmt.Errorf("ChainID mismatch: got %s expected %s",
			status.NodeInfo.Network, chainID))
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
	con := newWSCon(t)
	con.Close()
}

// receive a new block message
func TestWSNewBlock(t *testing.T) {
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
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
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
