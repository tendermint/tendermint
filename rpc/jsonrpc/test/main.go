package main

import (
	"fmt"
	"net/http"
	"os"

	amino "github.com/tendermint/go-amino"

	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	rpcserver "github.com/tendermint/tendermint/rpc/jsonrpc/server"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

var routes = map[string]*rpcserver.RPCFunc{
	"hello_world": rpcserver.NewRPCFunc(HelloWorld, "name,num"),
}

func HelloWorld(ctx *rpctypes.Context, name string, num int) (Result, error) {
	return Result{fmt.Sprintf("hi %s %d", name, num)}, nil
}

type Result struct {
	Result string
}

func main() {
	var (
		mux    = http.NewServeMux()
		cdc    = amino.NewCodec()
		logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout))
	)

	// Stop upon receiving SIGTERM or CTRL-C.
	tmos.TrapSignal(logger, func() {})

	rpcserver.RegisterRPCFuncs(mux, routes, cdc, logger)
	config := rpcserver.DefaultConfig()
	listener, err := rpcserver.Listen("tcp://127.0.0.1:8008", config)
	if err != nil {
		tmos.Exit(err.Error())
	}
	rpcserver.Serve(listener, mux, logger, config)
}
