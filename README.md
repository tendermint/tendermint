# go-rpc

[![CircleCI](https://circleci.com/gh/tendermint/go-rpc.svg?style=svg)](https://circleci.com/gh/tendermint/go-rpc)

HTTP RPC server supporting calls via uri params, jsonrpc, and jsonrpc over websockets

# How To

Define some types and routes:

```
// Define a type for results and register concrete versions with go-wire
type Result interface{}

type ResultStatus struct {
	Value string
}

var _ = wire.RegisterInterface(
	struct{ Result }{},
	wire.ConcreteType{&ResultStatus{}, 0x1},
)

// Define some routes
var Routes = map[string]*rpcserver.RPCFunc{
	"status": rpcserver.NewRPCFunc(StatusResult, "arg"),
}

// an rpc function
func StatusResult(v string) (Result, error) {
	return &ResultStatus{v}, nil
}

```

Now start the server:

```
mux := http.NewServeMux()
rpcserver.RegisterRPCFuncs(mux, Routes)
wm := rpcserver.NewWebsocketManager(Routes, nil)
mux.HandleFunc("/websocket", wm.WebsocketHandler)
go func() {
	_, err := rpcserver.StartHTTPServer("0.0.0.0:46657", mux)
	if err != nil {
		panic(err)
	}
}()

```

Note that unix sockets are supported as well (eg. `/path/to/socket` instead of `0.0.0.0:46657`)

Now see all available endpoints by sending a GET request to `0.0.0.0:46657`.
Each route is available as a GET request, as a JSONRPCv2 POST request, and via JSONRPCv2 over websockets


# Examples

* [Tendermint](https://github.com/tendermint/tendermint/blob/master/rpc/core/routes.go)
* [Network Monitor](https://github.com/tendermint/netmon/blob/master/handlers/routes.go)
