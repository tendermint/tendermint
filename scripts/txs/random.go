package main

import (
	"context"
	"crypto/rand"
	"fmt"
	mrand "math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	rpclib "github.com/tendermint/tendermint/rpc/lib/client"
)

func main() {
	args := os.Args[1:]

	ports, err := stringToInts(args[0])
	if err != nil {
		panic(err)
	}
	N, err := strconv.Atoi(args[1])
	if err != nil {
		panic(err)
	}
	txSize, err := strconv.Atoi(args[2])
	if err != nil {
		panic(err)
	}

	clients := []*rpclib.WSClient{} //.Client{}
	for _, port := range ports {
		url := fmt.Sprintf("localhost:%d", port)
		websocket := "/websocket"

		//client := rpcclient.NewHTTP(url, websocket)
		client := rpclib.NewWSClient(url, websocket, rpclib.PingPeriod(1*time.Second))
		//op, err := log.AllowLevel("debug")
		if err != nil {
			panic(err)
		}
		/*		logger := log.NewFilter(
					log.NewTMLogger(log.NewSyncWriter(os.Stdout)),
					op,
				).With("port", port)

				client.SetLogger(logger)*/
		if err := client.Start(); err != nil {
			panic(err)
		}

		clients = append(clients, client)
	}

	for i := 0; i < N; i++ {
		b := make([]byte, txSize)
		if _, err := rand.Read(b); err != nil {
			panic(err)
		}
		time.Sleep(time.Millisecond * time.Duration(mrand.Intn(10)))
		client := randClient(clients)
		err := client.Call(context.TODO(), "broadcast_tx_async", map[string]interface{}{
			"tx": b,
		})
		// r, err := client.BroadcastTxAsync(b)
		//resp := <-client.ResponsesCh
		fmt.Printf("%d %v; ", i, err)
	}
	fmt.Println("--")

	done := make(chan struct{}, N)
	for _, client := range clients {
		go func(c *rpclib.WSClient) {
			for {
				<-c.ResponsesCh
				done <- struct{}{}
			}
		}(client)
	}

	for i := 0; i < N; i++ {
		<-done
		fmt.Printf("%d ", i)
	}
	fmt.Println("")

	for _, client := range clients {
		client.Stop()
	}
}

func randClient(clients []*rpclib.WSClient) *rpclib.WSClient {
	i := mrand.Intn(len(clients))
	return clients[i]
}

func stringToInts(listOfPorts string) ([]int, error) {
	spl := strings.Split(listOfPorts, ",")

	ports := []int{}

	for _, sp := range spl {
		port, err := strconv.Atoi(sp)
		if err != nil {
			return nil, err
		}
		ports = append(ports, port)
	}

	return ports, nil

}
