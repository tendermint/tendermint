# go-rpc

[![CircleCI](https://circleci.com/gh/tendermint/go-rpc.svg?style=svg)](https://circleci.com/gh/tendermint/go-rpc)

HTTP RPC server supporting calls via uri params, jsonrpc, and jsonrpc over websockets

# Client Requests

Suppose we want to expose the rpc function `HelloWorld(name string, num int)`.

## GET (URI)

As a GET request, it would have URI encoded parameters, and look like:

```
curl 'http://localhost:8008/hello_world?name="my_world"&num=5'
```

Note the `'` around the url, which is just so bash doesn't ignore the quotes in `"my_world"`.
This should also work:

```
curl http://localhost:8008/hello_world?name=\"my_world\"&num=5
```

A GET request to `/` returns a list of available endpoints.
For those which take arguments, the arguments will be listed in order, with `_` where the actual value should be.

## POST (JSONRPC)

As a POST request, we use JSONRPC. For instance, the same request would have this as the body:

```
{
  "jsonrpc": "2.0",
  "id": "anything",
  "method": "hello_world",
  "params": {
    "name": "my_world",
    "num": 5
  }
}
```

With the above saved in file `data.json`, we can make the request with

```
curl --data @data.json http://localhost:8008
```

## WebSocket (JSONRPC)

All requests are exposed over websocket in the same form as the POST JSONRPC.
Websocket connections are available at their own endpoint, typically `/websocket`,
though this is configurable when starting the server.

# Server Definition

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
	_, err := rpcserver.StartHTTPServer("0.0.0.0:8008", mux)
	if err != nil {
		panic(err)
	}
}()

```

Note that unix sockets are supported as well (eg. `/path/to/socket` instead of `0.0.0.0:8008`)

Now see all available endpoints by sending a GET request to `0.0.0.0:8008`.
Each route is available as a GET request, as a JSONRPCv2 POST request, and via JSONRPCv2 over websockets.


# Examples

* [Tendermint](https://github.com/tendermint/tendermint/blob/master/rpc/core/routes.go)
* [tm-monitor](https://github.com/tendermint/tools/blob/master/tm-monitor/rpc.go)

## CHANGELOG

### 0.7.0

BREAKING CHANGES:

- removed `Client` empty interface
- `ClientJSONRPC#Call` `params` argument became a map
- rename `ClientURI` -> `URIClient`, `ClientJSONRPC` -> `JSONRPCClient`

IMPROVEMENTS:

- added `HTTPClient` interface, which can be used for both `ClientURI`
and `ClientJSONRPC`
- all params are now optional (Golang's default will be used if some param is missing)
- added `Call` method to `WSClient` (see method's doc for details)
