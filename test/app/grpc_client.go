package main

import (
	"encoding/hex"
	"fmt"
	"os"

	"context"

	amino "github.com/tendermint/go-amino"
	core_grpc "github.com/tendermint/tendermint/rpc/grpc"
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
	res, err := clientGRPC.BroadcastTx(context.Background(), &core_grpc.RequestBroadcastTx{Tx: txBytes})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	bz, err := amino.NewCodec().MarshalJSON(res)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println(string(bz))
}
