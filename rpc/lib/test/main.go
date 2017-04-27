package main

import (
	"fmt"
	"net/http"

	cmn "github.com/tendermint/tmlibs/common"
	rpcserver "github.com/tendermint/tendermint/rpc/lib/server"
)

var routes = map[string]*rpcserver.RPCFunc{
	"hello_world": rpcserver.NewRPCFunc(HelloWorld, "name,num"),
}

func HelloWorld(name string, num int) (Result, error) {
	return Result{fmt.Sprintf("hi %s %d", name, num)}, nil
}

type Result struct {
	Result string
}

func main() {
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, routes)
	_, err := rpcserver.StartHTTPServer("0.0.0.0:8008", mux)
	if err != nil {
		cmn.Exit(err.Error())
	}

	// Wait forever
	cmn.TrapSignal(func() {
	})

}
