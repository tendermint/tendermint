package rpctypes

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"

	amino "github.com/tendermint/go-amino"

	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
)

// a wrapper to emulate a sum type: jsonrpcid = string | int
// TODO: refactor when Go 2.0 arrives https://github.com/golang/go/issues/19412
type jsonrpcid interface {
	isJSONRPCID()
}

// JSONRPCStringID a wrapper for JSON-RPC string IDs
type JSONRPCStringID string

func (JSONRPCStringID) isJSONRPCID() {}

// JSONRPCIntID a wrapper for JSON-RPC integer IDs
type JSONRPCIntID int

func (JSONRPCIntID) isJSONRPCID() {}

//----------------------------------------
// REQUEST

type RPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      jsonrpcid       `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"` // must be map[string]interface{} or []interface{}
}

// UnmarshalJSON custom JSON unmarshalling due to jsonrpcid being string or int
func (request *RPCRequest) UnmarshalJSON(data []byte) error {
	unsafeResp := &struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      interface{}     `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"` // must be map[string]interface{} or []interface{}
	}{}
	err := json.Unmarshal(data, &unsafeResp)
	if err == nil {
		request.JSONRPC = unsafeResp.JSONRPC
		request.Method = unsafeResp.Method
		request.Params = unsafeResp.Params
		if unsafeResp.ID == nil {
			return nil
		}
		switch e := unsafeResp.ID.(type) {
		case string:
			request.ID = JSONRPCStringID(unsafeResp.ID.(string))
			return nil
		case int:
			request.ID = JSONRPCIntID(unsafeResp.ID.(int))
			return nil
		case float32:
			request.ID = JSONRPCIntID(int(unsafeResp.ID.(float32)))
			return nil
		case float64:
			request.ID = JSONRPCIntID(int(unsafeResp.ID.(float64)))
			return nil
		default:
			var r = reflect.TypeOf(e)
			return fmt.Errorf("Unknown JSON-RPC ID type: %v", r)
		}
	} else {
		return err
	}
}

func NewRPCRequest(id jsonrpcid, method string, params json.RawMessage) RPCRequest {
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

func MapToRequest(cdc *amino.Codec, id jsonrpcid, method string, params map[string]interface{}) (RPCRequest, error) {
	var params_ = make(map[string]json.RawMessage, len(params))
	for name, value := range params {
		valueJSON, err := cdc.MarshalJSON(value)
		if err != nil {
			return RPCRequest{}, err
		}
		params_[name] = valueJSON
	}
	payload, err := json.Marshal(params_) // NOTE: Amino doesn't handle maps yet.
	if err != nil {
		return RPCRequest{}, err
	}
	request := NewRPCRequest(id, method, payload)
	return request, nil
}

func ArrayToRequest(cdc *amino.Codec, id jsonrpcid, method string, params []interface{}) (RPCRequest, error) {
	var params_ = make([]json.RawMessage, len(params))
	for i, value := range params {
		valueJSON, err := cdc.MarshalJSON(value)
		if err != nil {
			return RPCRequest{}, err
		}
		params_[i] = valueJSON
	}
	payload, err := json.Marshal(params_) // NOTE: Amino doesn't handle maps yet.
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
	ID      jsonrpcid       `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// UnmarshalJSON custom JSON unmarshalling due to jsonrpcid being string or int
func (request *RPCResponse) UnmarshalJSON(data []byte) error {
	unsafeResp := &struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      interface{}     `json:"id"`
		Result  json.RawMessage `json:"result,omitempty"`
		Error   *RPCError       `json:"error,omitempty"`
	}{}
	err := json.Unmarshal(data, &unsafeResp)
	if err == nil {
		request.JSONRPC = unsafeResp.JSONRPC
		request.Error = unsafeResp.Error
		request.Result = unsafeResp.Result
		if unsafeResp.ID == nil {
			return nil
		}
		switch e := unsafeResp.ID.(type) {
		case string:
			request.ID = JSONRPCStringID(unsafeResp.ID.(string))
			return nil
		case int:
			request.ID = JSONRPCIntID(unsafeResp.ID.(int))
			return nil
		case float32:
			request.ID = JSONRPCIntID(int(unsafeResp.ID.(float32)))
			return nil
		case float64:
			request.ID = JSONRPCIntID(int(unsafeResp.ID.(float64)))
			return nil
		default:
			var r = reflect.TypeOf(e)
			return fmt.Errorf("Unknown JSON-RPC ID type: %v", r)
		}
	} else {
		return err
	}
}

func NewRPCSuccessResponse(cdc *amino.Codec, id jsonrpcid, res interface{}) RPCResponse {
	var rawMsg json.RawMessage

	if res != nil {
		var js []byte
		js, err := cdc.MarshalJSON(res)
		if err != nil {
			return RPCInternalError(id, errors.Wrap(err, "Error marshalling response"))
		}
		rawMsg = json.RawMessage(js)
	}

	return RPCResponse{JSONRPC: "2.0", ID: id, Result: rawMsg}
}

func NewRPCErrorResponse(id jsonrpcid, code int, msg string, data string) RPCResponse {
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

func RPCParseError(id jsonrpcid, err error) RPCResponse {
	return NewRPCErrorResponse(id, -32700, "Parse error. Invalid JSON", err.Error())
}

func RPCInvalidRequestError(id jsonrpcid, err error) RPCResponse {
	return NewRPCErrorResponse(id, -32600, "Invalid Request", err.Error())
}

func RPCMethodNotFoundError(id jsonrpcid) RPCResponse {
	return NewRPCErrorResponse(id, -32601, "Method not found", "")
}

func RPCInvalidParamsError(id jsonrpcid, err error) RPCResponse {
	return NewRPCErrorResponse(id, -32602, "Invalid params", err.Error())
}

func RPCInternalError(id jsonrpcid, err error) RPCResponse {
	return NewRPCErrorResponse(id, -32603, "Internal error", err.Error())
}

func RPCServerError(id jsonrpcid, err error) RPCResponse {
	return NewRPCErrorResponse(id, -32000, "Server error", err.Error())
}

//----------------------------------------

// *wsConnection implements this interface.
type WSRPCConnection interface {
	GetRemoteAddr() string
	WriteRPCResponse(resp RPCResponse)
	TryWriteRPCResponse(resp RPCResponse) bool
	GetEventSubscriber() EventSubscriber
	Codec() *amino.Codec
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
