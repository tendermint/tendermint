package main

import (
	"fmt"

	"github.com/gorilla/websocket"
	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/rpc/client"
	// ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/rpc/types"
)

func main() {
	ws := rpcclient.NewWSClient("ws://127.0.0.1:46657/websocket")
	// ws := rpcclient.NewWSClient("ws://104.236.69.128:46657/websocket")
	_, err := ws.Start()
	if err != nil {
		Exit(err.Error())
	}

	// Read a bunch of responses
	go func() {
		for {
			res, ok := <-ws.ResultsCh
			if !ok {
				break
			}
			fmt.Println("Received response", res)
		}
	}()

	// Make a bunch of requests
	request := rpctypes.NewRPCRequest("fakeid", "status", nil)
	for i := 0; ; i++ {
		reqBytes := wire.JSONBytes(request)
		err := ws.WriteMessage(websocket.TextMessage, reqBytes)
		if err != nil {
			Exit(err.Error())
		}
	}

	ws.Stop()
}
