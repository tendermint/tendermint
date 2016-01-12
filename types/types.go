package rpctypes

import (
	"github.com/tendermint/go-events"
)

type RPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      string        `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

func NewRPCRequest(id string, method string, params []interface{}) RPCRequest {
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
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Result  Result `json:"result"`
	Error   string `json:"error"`
}

func NewRPCResponse(id string, res Result, err string) RPCResponse {
	return RPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  res,
		Error:   err,
	}
}

//----------------------------------------

// *wsConnection implements this interface.
type WSRPCConnection interface {
	GetRemoteAddr() string
	GetEventSwitch() *events.EventSwitch
	WriteRPCResponse(resp RPCResponse)
	TryWriteRPCResponse(resp RPCResponse) bool
}

// websocket-only RPCFuncs take this as the first parameter.
type WSRPCContext struct {
	Request RPCRequest
	WSRPCConnection
}
