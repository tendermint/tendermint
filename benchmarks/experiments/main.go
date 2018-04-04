package main

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"context"

	"github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tmlibs/log"
)

var logger = log.NewNopLogger()
var finishedTasks = 0
var mutex = &sync.Mutex{}

func main() {

	var endpoint = "tcp://0.0.0.0:46657"

	var httpClient = getHTTPClient(endpoint)

	var res, err = httpClient.Status()
	if err != nil {
		logger.Info("something wrong happens", err)
	}
	logger.Info("received status", res)

	go monitorTask(endpoint)

	txCount := 10
	var clientNumber = 10
	for i := 0; i < clientNumber; i++ {
		go clientTask(i, txCount, endpoint)
	}
	for finishedTasks < clientNumber+1 {
	}
	fmt.Printf("Done: %d\n", finishedTasks)
}

func clientTask(id, txCount int, endpoint string) {
	var httpClient = getHTTPClient(endpoint)
	for i := 0; i < txCount; i++ {
		var _, err = httpClient.BroadcastTxSync(generateTx(id, rand.Int()))
		if err != nil {
			fmt.Printf("Something wrong happened: %s\n", err)
		}
	}
	fmt.Printf("Finished client task: %d\n", id)

	mutex.Lock()
	finishedTasks++
	mutex.Unlock()
}

func getHTTPClient(rpcAddr string) *client.HTTP {
	return client.NewHTTP(rpcAddr, "/websocket")
}

func generateTx(i, valI int) []byte {
	// a tx encodes the validator index, the tx number, and some random junk
	tx := make([]byte, 250)
	binary.PutUvarint(tx[:32], uint64(valI))
	binary.PutUvarint(tx[32:64], uint64(i))
	if _, err := rand.Read(tx[65:]); err != nil {
		fmt.Println("err reading from crypto/rand", err)
		os.Exit(1)
	}
	return tx
}

func monitorTask(endpoint string) {
	fmt.Println("Monitor task started...")
	var duration = 5 * time.Second

	const subscriber = "monitor"
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	evts := make(chan interface{})

	var httpClient = getHTTPClient(endpoint)
	httpClient.Start()

	evtTyp := types.EventNewBlockHeader

	// register for the next event of this type
	query := types.QueryForEvent(evtTyp)
	err := httpClient.Subscribe(ctx, subscriber, query, evts)
	if err != nil {
		fmt.Println("error when subscribing", err)
	}

	// make sure to unregister after the test is over
	defer httpClient.UnsubscribeAll(ctx, subscriber)

	totalNumOfCommittedTxs := int64(0)

	for {
		fmt.Println("Starting main loop", err)
		select {
		case evt := <-evts:
			event := evt.(types.TMEventData)
			header, ok := event.Unwrap().(types.EventDataNewBlockHeader)
			if ok {
				fmt.Println("received header\n", header.Header.StringIndented(""))
			} else {
				fmt.Println("not able to unwrap header")
			}
			// Do some metric computation with header
			totalNumOfCommittedTxs += header.Header.NumTxs

		case <-ctx.Done():
			fmt.Printf("Finished monitor task. Received %d transactions \n", totalNumOfCommittedTxs)

			mutex.Lock()
			finishedTasks++
			mutex.Unlock()
			return
		}
	}
}
