package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/tendermint/tendermint/libs/log"
	rpcserver "github.com/tendermint/tendermint/rpc/jsonrpc/server"
)

var routes = map[string]*rpcserver.RPCFunc{
	"hello_world": rpcserver.NewRPCFunc(HelloWorld, "name", "num"),
}

func HelloWorld(ctx context.Context, name string, num int) (Result, error) {
	return Result{fmt.Sprintf("hi %s %d", name, num)}, nil
}

type Result struct {
	Result string
}

func main() {
	var (
		mux    = http.NewServeMux()
		logger = log.MustNewDefaultLogger(log.LogFormatPlain, log.LogLevelInfo)
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

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
