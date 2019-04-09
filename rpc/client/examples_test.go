package client_test

import (
	"bytes"
	"fmt"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/rpc/client"
)

func ExampleHTTP_simple() {
	// Assumes the Tendermint RPC endpoint is available through
	// http://127.0.0.1:26657 and is running the kvstore proxy app
	c := client.NewHTTP("127.0.0.1:26657", "/websocket")

	// Generate a random key/value pair
	k := []byte(cmn.RandStr(8))
	v := []byte(cmn.RandStr(8))
	tx := append(k, append([]byte("="), v...)...)

	// Broadcast the transaction and wait for it to commit (rather use
	// c.BroadcastTxSync though in production)
	bres, err := c.BroadcastTxCommit(tx)
	if err != nil {
		panic(err)
	}
	if bres.CheckTx.IsErr() || bres.DeliverTx.IsErr() {
		panic("BroadcastTxCommit transaction failed")
	}

	// Now try to fetch the value for the key
	qres, err := c.ABCIQuery("/key", k)
	if err != nil {
		panic(err)
	}
	if qres.Response.IsErr() {
		panic("ABCIQuery failed")
	}
	if !bytes.Equal(qres.Response.Key, k) {
		panic("returned key does not match queried key")
	}
	if !bytes.Equal(qres.Response.Value, v) {
		panic("returned value does not match sent value")
	}
}

func ExampleHTTP_batching() {
	makeTxKV := func() []byte {
		k := []byte(cmn.RandStr(8))
		v := []byte(cmn.RandStr(8))
		return append(k, append([]byte("="), v...)...)
	}

	// Assumes the Tendermint RPC endpoint is available through
	// http://127.0.0.1:26657 and is running the kvstore proxy app
	c := client.NewHTTP("127.0.0.1:26657", "/websocket")

	// Generate two random key/value pairs and their equivalent transactions
	tx1 := makeTxKV()
	tx2 := makeTxKV()

	// Create a new batch
	batch := c.NewBatch()

	// Queue up the first transaction
	_, err := batch.BroadcastTxCommit(tx1)
	if err != nil {
		panic(err)
	}

	// Then queue up the second transaction
	_, err = batch.BroadcastTxCommit(tx2)
	if err != nil {
		panic(err)
	}

	// Send the batch of 2 transactions
	results, err := batch.Send()
	if err != nil {
		panic(err)
	}
	// `results` will now contain the deserialized result objects from each of
	// the sent BroadcastTxCommit transactions

	// Do something useful with the results here
	for _, result := range results {
		fmt.Println(result)
	}
}
