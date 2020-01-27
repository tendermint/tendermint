package client_test

import (
	"bytes"
	"fmt"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctest "github.com/tendermint/tendermint/rpc/test"
)

func ExampleHTTP_simple() {
	// Start a tendermint node (and kvstore) in the background to test against
	app := kvstore.NewApplication()
	node := rpctest.StartTendermint(app, rpctest.SuppressStdout, rpctest.RecreateConfig)
	defer rpctest.StopTendermint(node)

	// Create our RPC client
	rpcAddr := rpctest.GetConfig().RPC.ListenAddress
	c, err := client.NewHTTP(rpcAddr, "/websocket")
	if err != nil {
		panic(err)
	}

	// Create a transaction
	k := []byte("name")
	v := []byte("satoshi")
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

	fmt.Println("Sent tx     :", string(tx))
	fmt.Println("Queried for :", string(qres.Response.Key))
	fmt.Println("Got value   :", string(qres.Response.Value))

	// Output:
	// Sent tx     : name=satoshi
	// Queried for : name
	// Got value   : satoshi
}

func ExampleHTTP_batching() {
	// Start a tendermint node (and kvstore) in the background to test against
	app := kvstore.NewApplication()
	node := rpctest.StartTendermint(app, rpctest.SuppressStdout, rpctest.RecreateConfig)
	defer rpctest.StopTendermint(node)

	// Create our RPC client
	rpcAddr := rpctest.GetConfig().RPC.ListenAddress
	c, err := client.NewHTTP(rpcAddr, "/websocket")
	if err != nil {
		panic(err)
	}

	// Create our two transactions
	k1 := []byte("firstName")
	v1 := []byte("satoshi")
	tx1 := append(k1, append([]byte("="), v1...)...)

	k2 := []byte("lastName")
	v2 := []byte("nakamoto")
	tx2 := append(k2, append([]byte("="), v2...)...)

	txs := [][]byte{tx1, tx2}

	// Create a new batch
	batch := c.NewBatch()

	// Queue up our transactions
	for _, tx := range txs {
		if _, err := batch.BroadcastTxCommit(tx); err != nil {
			panic(err)
		}
	}

	// Send the batch of 2 transactions
	if _, err := batch.Send(); err != nil {
		panic(err)
	}

	// Now let's query for the original results as a batch
	keys := [][]byte{k1, k2}
	for _, key := range keys {
		if _, err := batch.ABCIQuery("/key", key); err != nil {
			panic(err)
		}
	}

	// Send the 2 queries and keep the results
	results, err := batch.Send()
	if err != nil {
		panic(err)
	}

	// Each result in the returned list is the deserialized result of each
	// respective ABCIQuery response
	for _, result := range results {
		qr, ok := result.(*ctypes.ResultABCIQuery)
		if !ok {
			panic("invalid result type from ABCIQuery request")
		}
		fmt.Println(string(qr.Response.Key), "=", string(qr.Response.Value))
	}

	// Output:
	// firstName = satoshi
	// lastName = nakamoto
}
