package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	rpcserver "github.com/tendermint/tendermint/rpc/jsonrpc/server"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

var routes = map[string]*rpcserver.RPCFunc{
	"hello_world": rpcserver.NewRPCFunc(HelloWorld, "name,num", false),
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
		logger = log.MustNewDefaultLogger(log.LogFormatPlain, log.LogLevelInfo, false)
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Stop upon receiving SIGTERM or CTRL-C.
	tmos.TrapSignal(ctx, logger, func() {})

	rpcserver.RegisterRPCFuncs(mux, routes, logger)
	config := rpcserver.DefaultConfig()
	listener, err := rpcserver.Listen("tcp://127.0.0.1:8008", config.MaxOpenConnections)
	if err != nil {
		logger.Error("rpc listening", "err", err)
		os.Exit(1)
	}

	if err = rpcserver.Serve(ctx, listener, mux, logger, config); err != nil {
		logger.Error("rpc serve", "err", err)
		os.Exit(1)
	}
}
