package main

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tendermint/go-rpc/client"
	rpctypes "github.com/tendermint/go-rpc/types"
)

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) < 2 {
		fmt.Println("transact.go expects at least two arguments (ntxs, hosts)")
		os.Exit(1)
	}

	nTxS, hostS := args[0], args[1]
	nTxs, err := strconv.Atoi(nTxS)
	if err != nil {
		fmt.Println("ntxs must be an integer:", err)
		os.Exit(1)
	}

	hosts := strings.Split(hostS, ",")

	errCh := make(chan error, 1000)

	wg := new(sync.WaitGroup)
	wg.Add(len(hosts))
	start := time.Now()
	fmt.Printf("Sending %d txs on every host %v\n", nTxs, hosts)
	for i, host := range hosts {
		go broadcastTxsToHost(wg, errCh, i, host, nTxs, 0)
	}
	wg.Wait()
	fmt.Println("Done broadcasting txs. Took", time.Since(start))

}

func broadcastTxsToHost(wg *sync.WaitGroup, errCh chan error, valI int, valHost string, nTxs int, txCount int) {
	reconnectSleepSeconds := time.Second * 1

	// thisStart := time.Now()
	// cli := rpcclient.NewClientURI(valHost + ":46657")
	fmt.Println("Connecting to host to broadcast txs", valI, valHost)
	cli := rpcclient.NewWSClient(valHost, "/websocket")
	if _, err := cli.Start(); err != nil {
		if nTxs == 0 {
			time.Sleep(reconnectSleepSeconds)
			broadcastTxsToHost(wg, errCh, valI, valHost, nTxs, txCount)
			return
		}
		fmt.Printf("Error starting websocket connection to val%d (%s): %v\n", valI, valHost, err)
		os.Exit(1)
	}

	reconnect := make(chan struct{})
	go func(count int) {
	LOOP:
		for {
			ticker := time.NewTicker(reconnectSleepSeconds)
			select {
			case <-cli.ResultsCh:
				count += 1
				// nTxs == 0 means just loop forever
				if nTxs > 0 && count == nTxs {
					break LOOP
				}
			case err := <-cli.ErrorsCh:
				fmt.Println("err: val", valI, valHost, err)
			case <-cli.Quit:
				broadcastTxsToHost(wg, errCh, valI, valHost, nTxs, count)
				return
			case <-reconnect:
				broadcastTxsToHost(wg, errCh, valI, valHost, nTxs, count)
				return
			case <-ticker.C:
				if nTxs == 0 {
					cli.Stop()
					broadcastTxsToHost(wg, errCh, valI, valHost, nTxs, count)
					return
				}
			}
		}
		fmt.Printf("Received all responses from node %d (%s)\n", valI, valHost)
		wg.Done()
	}(txCount)
	var i = 0
	for {
		/*		if i%(nTxs/4) == 0 {
				fmt.Printf("Have sent %d txs to node %d. Total time so far: %v\n", i, valI, time.Since(thisStart))
			}*/

		if !cli.IsRunning() {
			return
		}

		tx := generateTx(i, valI)
		if err := cli.WriteJSON(rpctypes.RPCRequest{
			JSONRPC: "2.0",
			ID:      "",
			Method:  "broadcast_tx_async",
			Params:  []interface{}{hex.EncodeToString(tx)},
		}); err != nil {
			fmt.Printf("Error sending tx %d to validator %d: %v. Attempt reconnect\n", i, valI, err)
			reconnect <- struct{}{}
			return
		}
		i += 1
		if nTxs > 0 && i >= nTxs {
			break
		} else if nTxs == 0 {
			time.Sleep(time.Millisecond * 1)
		}
	}
	fmt.Printf("Done sending %d txs to node s%d (%s)\n", nTxs, valI, valHost)
}

func generateTx(i, valI int) []byte {
	// a tx encodes the validator index, the tx number, and some random junk
	// TODO: read random bytes into more of the tx
	tx := make([]byte, 250)
	binary.PutUvarint(tx[:32], uint64(valI))
	binary.PutUvarint(tx[32:64], uint64(i))
	if _, err := rand.Read(tx[234:]); err != nil {
		fmt.Println("err reading from crypto/rand", err)
		os.Exit(1)
	}
	return tx
}
