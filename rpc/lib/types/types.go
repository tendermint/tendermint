package rpctypes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	tmpubsub "github.com/tendermint/tmlibs/pubsub"
)

//----------------------------------------
// REQUEST

type RPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"` // must be map[string]interface{} or []interface{}
}

func NewRPCRequest(id string, method string, params json.RawMessage) RPCRequest {
	return RPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
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
// RESPONSE

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

func (err RPCError) Error() string {
	const baseFormat = "RPC error %v - %s"
	if err.Data != "" {
		return fmt.Sprintf(baseFormat+": %s", err.Code, err.Message, err.Data)
	}
	return fmt.Sprintf(baseFormat, err.Code, err.Message)
}

type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

func NewRPCSuccessResponse(id string, res interface{}) RPCResponse {
	var rawMsg json.RawMessage

	if res != nil {
		var js []byte
		js, err := json.Marshal(res)
		if err != nil {
			return RPCInternalError(id, errors.Wrap(err, "Error marshalling response"))
		}
		rawMsg = json.RawMessage(js)
	}

	return RPCResponse{JSONRPC: "2.0", ID: id, Result: rawMsg}
}

func NewRPCErrorResponse(id string, code int, msg string, data string) RPCResponse {
	return RPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: msg, Data: data},
	}
}

func (resp RPCResponse) String() string {
	if resp.Error == nil {
		return fmt.Sprintf("[%s %v]", resp.ID, resp.Result)
	}
	return fmt.Sprintf("[%s %s]", resp.ID, resp.Error)
}

func RPCParseError(id string, err error) RPCResponse {
	return NewRPCErrorResponse(id, -32700, "Parse error. Invalid JSON", err.Error())
}

func RPCInvalidRequestError(id string, err error) RPCResponse {
	return NewRPCErrorResponse(id, -32600, "Invalid Request", err.Error())
}

func RPCMethodNotFoundError(id string) RPCResponse {
	return NewRPCErrorResponse(id, -32601, "Method not found", "")
}

func RPCInvalidParamsError(id string, err error) RPCResponse {
	return NewRPCErrorResponse(id, -32602, "Invalid params", err.Error())
}

func RPCInternalError(id string, err error) RPCResponse {
	return NewRPCErrorResponse(id, -32603, "Internal error", err.Error())
}

func RPCServerError(id string, err error) RPCResponse {
	return NewRPCErrorResponse(id, -32000, "Server error", err.Error())
}

//----------------------------------------

// *wsConnection implements this interface.
type WSRPCConnection interface {
	GetRemoteAddr() string
	WriteRPCResponse(resp RPCResponse)
	TryWriteRPCResponse(resp RPCResponse) bool
	GetEventSubscriber() EventSubscriber
}

// EventSubscriber mirros tendermint/tendermint/types.EventBusSubscriber
type EventSubscriber interface {
	Subscribe(ctx context.Context, subscriber string, query tmpubsub.Query, out chan<- interface{}) error
	Unsubscribe(ctx context.Context, subscriber string, query tmpubsub.Query) error
	UnsubscribeAll(ctx context.Context, subscriber string) error
}

// websocket-only RPCFuncs take this as the first parameter.
type WSRPCContext struct {
	Request RPCRequest
	WSRPCConnection
}

//----------------------------------------
// SOCKETS
//
// Determine if its a unix or tcp socket.
// If tcp, must specify the port; `0.0.0.0` will return incorrectly as "unix" since there's no port
// TODO: deprecate
func SocketType(listenAddr string) string {
	socketType := "unix"
	if len(strings.Split(listenAddr, ":")) >= 2 {
		socketType = "tcp"
	}
	return socketType
}
