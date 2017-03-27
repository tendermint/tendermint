package main

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
	cmn "github.com/tendermint/go-common"
	rpcclient "github.com/tendermint/go-rpc/client"
	rpctypes "github.com/tendermint/go-rpc/types"
	wire "github.com/tendermint/go-wire"
	_ "github.com/tendermint/tendermint/rpc/core/types" // Register RPCResponse > Result types
)

func main() {
	ws := rpcclient.NewWSClient("127.0.0.1:46657", "/websocket")
	_, err := ws.Start()
	if err != nil {
		cmn.Exit(err.Error())
	}

	// Read a bunch of responses
	go func() {
		for {
			_, ok := <-ws.ResultsCh
			if !ok {
				break
			}
			//fmt.Println("Received response", string(wire.JSONBytes(res)))
		}
	}()

	// Make a bunch of requests
	buf := make([]byte, 32)
	for i := 0; ; i++ {
		binary.BigEndian.PutUint64(buf, uint64(i))
		//txBytes := hex.EncodeToString(buf[:n])
		request := rpctypes.NewRPCRequest("fakeid", "broadcast_tx", map[string]interface{}{"tx": cmn.Arr(buf[:8])})
		reqBytes := wire.JSONBytes(request)
		//fmt.Println("!!", string(reqBytes))
		fmt.Print(".")
		err := ws.WriteMessage(websocket.TextMessage, reqBytes)
		if err != nil {
			cmn.Exit(err.Error())
		}
		if i%1000 == 0 {
			fmt.Println(i)
		}
		time.Sleep(time.Microsecond * 1000)
	}

	ws.Stop()
}
