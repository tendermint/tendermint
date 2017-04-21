package rpctypes

import (
	"encoding/json"
	"strings"

	events "github.com/tendermint/go-events"
	wire "github.com/tendermint/go-wire"
)

type RPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"` // must be map[string]interface{} or []interface{}
}

func NewRPCRequest(id string, method string, params map[string]interface{}) RPCRequest {
	return RPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
}

//----------------------------------------

/*
Result is a generic interface.
Applications should register type-bytes like so:

var _ = wire.RegisterInterface(
	struct{ Result }{},
	wire.ConcreteType{&ResultGenesis{}, ResultTypeGenesis},
	wire.ConcreteType{&ResultBlockchainInfo{}, ResultTypeBlockchainInfo},
	...
)
*/
type Result interface {
}

//----------------------------------------

type RPCResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      string           `json:"id"`
	Result  *json.RawMessage `json:"result"`
	Error   string           `json:"error"`
}

func NewRPCResponse(id string, res interface{}, err string) RPCResponse {
	var raw *json.RawMessage
	if res != nil {
		rawMsg := json.RawMessage(wire.JSONBytes(res))
		raw = &rawMsg
	}
	return RPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  raw,
		Error:   err,
	}
}

//----------------------------------------

// *wsConnection implements this interface.
type WSRPCConnection interface {
	GetRemoteAddr() string
	GetEventSwitch() events.EventSwitch
	WriteRPCResponse(resp RPCResponse)
	TryWriteRPCResponse(resp RPCResponse) bool
}

// websocket-only RPCFuncs take this as the first parameter.
type WSRPCContext struct {
	Request RPCRequest
	WSRPCConnection
}

//----------------------------------------
// sockets
//
// Determine if its a unix or tcp socket.
// If tcp, must specify the port; `0.0.0.0` will return incorrectly as "unix" since there's no port
func SocketType(listenAddr string) string {
	socketType := "unix"
	if len(strings.Split(listenAddr, ":")) >= 2 {
		socketType = "tcp"
	}
	return socketType
}
