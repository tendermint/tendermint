package main

import (
	"encoding/hex"
	"fmt"
	"os"

	"golang.org/x/net/context"

	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/rpc/grpc"
)

var grpcAddr = "tcp://localhost:36656"

func main() {
	args := os.Args
	if len(args) == 1 {
		fmt.Println("Must enter a transaction to send (hex)")
		os.Exit(1)
	}
	tx := args[1]
	txBytes, err := hex.DecodeString(tx)
	if err != nil {
		fmt.Println("Invalid hex", err)
		os.Exit(1)
	}

	clientGRPC := core_grpc.StartGRPCClient(grpcAddr)
	res, err := clientGRPC.BroadcastTx(context.Background(), &core_grpc.RequestBroadcastTx{txBytes})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println(string(wire.JSONBytes(res)))
}
