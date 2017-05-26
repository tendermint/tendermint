package rpctypes

import (
	"encoding/json"
	"fmt"
	"strings"

	events "github.com/tendermint/tmlibs/events"
)

type RpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type RPCRequest struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      string           `json:"id"`
	Method  string           `json:"method"`
	Params  *json.RawMessage `json:"params"` // must be map[string]interface{} or []interface{}
}

func NewRPCRequest(id string, method string, params json.RawMessage) RPCRequest {
	return RPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  &params,
	}
}

func (req RPCRequest) String() string {
	return fmt.Sprintf("[%s %s]", req.ID, req.Method)
}

func MapToRequest(id string, method string, params map[string]interface{}) (RPCRequest, error) {
	payload, err := json.Marshal(params)
	if err != nil {
		return RPCRequest{}, err
	}
	request := NewRPCRequest(id, method, payload)
	return request, nil
}

func ArrayToRequest(id string, method string, params []interface{}) (RPCRequest, error) {
	payload, err := json.Marshal(params)
	if err != nil {
		return RPCRequest{}, err
	}
	request := NewRPCRequest(id, method, payload)
	return request, nil
}

//----------------------------------------

type RPCResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      string           `json:"id,omitempty"`
	Result  *json.RawMessage `json:"result,omitempty"`
	Error   *RpcError        `json:"error,omitempty"`
}

func NewRPCSuccessResponse(id string, res interface{}) RPCResponse {
	var raw *json.RawMessage

	if res != nil {
		var js []byte
		js, err := json.Marshal(res)
		if err != nil {
			return RPCInternalError(id)
		}
		rawMsg := json.RawMessage(js)
		raw = &rawMsg
	}

	return RPCResponse{JSONRPC: "2.0", ID: id, Result: raw}
}

func NewRPCErrorResponse(id string, code int, msg string) RPCResponse {
	return RPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RpcError{Code: code, Message: msg},
	}
}

func (resp RPCResponse) String() string {
	if resp.Error == "" {
		return fmt.Sprintf("[%s %v]", resp.ID, resp.Result)
	} else {
		return fmt.Sprintf("[%s %s]", resp.ID, resp.Error)
	}
}

func RPCParseError(id string) RPCResponse {
	return NewRPCErrorResponse(id, -32700, "Parse error. Invalid JSON")
}

func RPCInvalidRequestError(id string) RPCResponse {
	return NewRPCErrorResponse(id, -32600, "Invalid Request")
}

func RPCMethodNotFoundError(id string) RPCResponse {
	return NewRPCErrorResponse(id, -32601, "Method not found")
}

func RPCInvalidParamsError(id string) RPCResponse {
	return NewRPCErrorResponse(id, -32602, "Invalid params")
}

func RPCInternalError(id string) RPCResponse {
	return NewRPCErrorResponse(id, -32603, "Internal error")
}

func RPCServerError(id string) RPCResponse {
	return NewRPCErrorResponse(id, -32000, "Server error")
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
