package client_test

import (
	"bytes"
	"context"
	"fmt"
	"log"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	"github.com/tendermint/tendermint/rpc/coretypes"
	rpctest "github.com/tendermint/tendermint/rpc/test"
)

func ExampleHTTP_simple() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start a tendermint node (and kvstore) in the background to test against
	app := kvstore.NewApplication()
	conf := rpctest.CreateConfig("ExampleHTTP_simple")

	_, closer, err := rpctest.StartTendermint(ctx, conf, app, rpctest.SuppressStdout)
	if err != nil {
		log.Fatal(err) //nolint:gocritic
	}
	defer func() { _ = closer(ctx) }()

	// Create our RPC client
	rpcAddr := conf.RPC.ListenAddress
	c, err := rpchttp.New(rpcAddr)
	if err != nil {
		log.Fatal(err)
	}

	// Create a transaction
	k := []byte("name")
	v := []byte("satoshi")
	tx := append(k, append([]byte("="), v...)...)

	// Broadcast the transaction and wait for it to commit (rather use
	// c.BroadcastTxSync though in production).
	bres, err := c.BroadcastTxCommit(context.Background(), tx)
	if err != nil {
		log.Fatal(err)
	}
	if bres.CheckTx.IsErr() || bres.DeliverTx.IsErr() {
		log.Fatal("BroadcastTxCommit transaction failed")
	}

	// Now try to fetch the value for the key
	qres, err := c.ABCIQuery(context.Background(), "/key", k)
	if err != nil {
		log.Fatal(err)
	}
	if qres.Response.IsErr() {
		log.Fatal("ABCIQuery failed")
	}
	if !bytes.Equal(qres.Response.Key, k) {
		log.Fatal("returned key does not match queried key")
	}
	if !bytes.Equal(qres.Response.Value, v) {
		log.Fatal("returned value does not match sent value")
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start a tendermint node (and kvstore) in the background to test against
	app := kvstore.NewApplication()
	conf := rpctest.CreateConfig("ExampleHTTP_batching")

	_, closer, err := rpctest.StartTendermint(ctx, conf, app, rpctest.SuppressStdout)
	if err != nil {
		log.Fatal(err) //nolint:gocritic
	}
	defer func() { _ = closer(ctx) }()

	rpcAddr := conf.RPC.ListenAddress
	c, err := rpchttp.New(rpcAddr)
	if err != nil {
		log.Fatal(err)
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
		// Broadcast the transaction and wait for it to commit (rather use
		// c.BroadcastTxSync though in production).
		if _, err := batch.BroadcastTxCommit(context.Background(), tx); err != nil {
			log.Fatal(err)
		}
	}

	// Send the batch of 2 transactions
	if _, err := batch.Send(context.Background()); err != nil {
		log.Fatal(err)
	}

	// Now let's query for the original results as a batch
	keys := [][]byte{k1, k2}
	for _, key := range keys {
		if _, err := batch.ABCIQuery(context.Background(), "/key", key); err != nil {
			log.Fatal(err)
		}
	}

	// Send the 2 queries and keep the results
	results, err := batch.Send(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// Each result in the returned list is the deserialized result of each
	// respective ABCIQuery response
	for _, result := range results {
		qr, ok := result.(*coretypes.ResultABCIQuery)
		if !ok {
			log.Fatal("invalid result type from ABCIQuery request")
		}
		fmt.Println(string(qr.Response.Key), "=", string(qr.Response.Value))
	}

	// Output:
	// firstName = satoshi
	// lastName = nakamoto
}
