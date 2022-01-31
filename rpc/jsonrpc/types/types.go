package types

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/tendermint/tendermint/rpc/coretypes"
)

// a wrapper to emulate a sum type: jsonrpcid = string | int
// TODO: refactor when Go 2.0 arrives https://github.com/golang/go/issues/19412
type jsonrpcid interface {
	isJSONRPCID()
}

// JSONRPCStringID a wrapper for JSON-RPC string IDs
type JSONRPCStringID string

func (JSONRPCStringID) isJSONRPCID()      {}
func (id JSONRPCStringID) String() string { return string(id) }

// JSONRPCIntID a wrapper for JSON-RPC integer IDs
type JSONRPCIntID int

func (JSONRPCIntID) isJSONRPCID()      {}
func (id JSONRPCIntID) String() string { return fmt.Sprintf("%d", id) }

func idFromInterface(idInterface interface{}) (jsonrpcid, error) {
	switch id := idInterface.(type) {
	case string:
		return JSONRPCStringID(id), nil
	case float64:
		// json.Unmarshal uses float64 for all numbers
		// (https://golang.org/pkg/encoding/json/#Unmarshal),
		// but the JSONRPC2.0 spec says the id SHOULD NOT contain
		// decimals - so we truncate the decimals here.
		return JSONRPCIntID(int(id)), nil
	default:
		typ := reflect.TypeOf(id)
		return nil, fmt.Errorf("json-rpc ID (%v) is of unknown type (%v)", id, typ)
	}
}

// ErrorCode is the type of JSON-RPC error codes.
type ErrorCode int

func (e ErrorCode) String() string {
	if s, ok := errorCodeString[e]; ok {
		return s
	}
	return fmt.Sprintf("server error: code %d", e)
}

// Constants defining the standard JSON-RPC error codes.
const (
	CodeParseError     ErrorCode = -32700 // Invalid JSON received by the server
	CodeInvalidRequest ErrorCode = -32600 // The JSON sent is not a valid request object
	CodeMethodNotFound ErrorCode = -32601 // The method does not exist or is unavailable
	CodeInvalidParams  ErrorCode = -32602 // Invalid method parameters
	CodeInternalError  ErrorCode = -32603 // Internal JSON-RPC error
)

var errorCodeString = map[ErrorCode]string{
	CodeParseError:     "Parse error",
	CodeInvalidRequest: "Invalid request",
	CodeMethodNotFound: "Method not found",
	CodeInvalidParams:  "Invalid params",
	CodeInternalError:  "Internal error",
}

//----------------------------------------
// REQUEST

type RPCRequest struct {
	ID     jsonrpcid
	Method string
	Params json.RawMessage
}

type rpcRequestJSON struct {
	V  string          `json:"jsonrpc"` // must be "2.0"
	ID interface{}     `json:"id,omitempty"`
	M  string          `json:"method"`
	P  json.RawMessage `json:"params"`
}

// UnmarshalJSON decodes a request from a JSON-RPC 2.0 request object.
func (req *RPCRequest) UnmarshalJSON(data []byte) error {
	var wrapper rpcRequestJSON
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return err
	} else if wrapper.V != "" && wrapper.V != "2.0" {
		return fmt.Errorf("invalid version: %q", wrapper.V)
	}

	if wrapper.ID != nil {
		id, err := idFromInterface(wrapper.ID)
		if err != nil {
			return fmt.Errorf("invalid request ID: %w", err)
		}
		req.ID = id
	}
	req.Method = wrapper.M
	req.Params = wrapper.P
	return nil
}

// MarshalJSON marshals a request with the appropriate version tag.
func (req RPCRequest) MarshalJSON() ([]byte, error) {
	return json.Marshal(rpcRequestJSON{
		V:  "2.0",
		ID: req.ID,
		M:  req.Method,
		P:  req.Params,
	})
}

func (req RPCRequest) String() string {
	return fmt.Sprintf("RPCRequest{%s %s/%X}", req.ID, req.Method, req.Params)
}

// MakeResponse constructs a success response to req with the given result.  If
// there is an error marshaling result to JSON, it returns an error response.
func (req RPCRequest) MakeResponse(result interface{}) RPCResponse {
	data, err := json.Marshal(result)
	if err != nil {
		return req.MakeErrorf(CodeInternalError, "marshaling result: %v", err)
	}
	return RPCResponse{ID: req.ID, Result: data}
}

// MakeErrorf constructs an error response to req with the given code and a
// message constructed by formatting msg with args.
func (req RPCRequest) MakeErrorf(code ErrorCode, msg string, args ...interface{}) RPCResponse {
	return RPCResponse{
		ID: req.ID,
		Error: &RPCError{
			Code:    int(code),
			Message: code.String(),
			Data:    fmt.Sprintf(msg, args...),
		},
	}
}

// MakeError constructs an error response to req from the given error value.
// This function will panic if err == nil.
func (req RPCRequest) MakeError(err error) RPCResponse {
	if err == nil {
		panic("cannot construct an error response for nil")
	}
	if e, ok := err.(*RPCError); ok {
		return RPCResponse{ID: req.ID, Error: e}
	}
	if errors.Is(err, coretypes.ErrZeroOrNegativeHeight) ||
		errors.Is(err, coretypes.ErrZeroOrNegativePerPage) ||
		errors.Is(err, coretypes.ErrPageOutOfRange) ||
		errors.Is(err, coretypes.ErrInvalidRequest) {
		return RPCResponse{ID: req.ID, Error: &RPCError{
			Code:    int(CodeInvalidRequest),
			Message: CodeInvalidRequest.String(),
			Data:    err.Error(),
		}}
	}
	return RPCResponse{ID: req.ID, Error: &RPCError{
		Code:    int(CodeInternalError),
		Message: CodeInternalError.String(),
		Data:    err.Error(),
	}}
}

// ParamsToRequest constructs a new RPCRequest with the given ID, method, and parameters.
func ParamsToRequest(id jsonrpcid, method string, params interface{}) (RPCRequest, error) {
	payload, err := json.Marshal(params)
	if err != nil {
		return RPCRequest{}, err
	}
	return RPCRequest{
		ID:     id,
		Method: method,
		Params: payload,
	}, nil
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
	ID     jsonrpcid
	Result json.RawMessage
	Error  *RPCError
}

type rpcResponseJSON struct {
	V  string          `json:"jsonrpc"` // must be "2.0"
	ID interface{}     `json:"id,omitempty"`
	R  json.RawMessage `json:"result,omitempty"`
	E  *RPCError       `json:"error,omitempty"`
}

// UnmarshalJSON decodes a response from a JSON-RPC 2.0 response object.
func (resp *RPCResponse) UnmarshalJSON(data []byte) error {
	var wrapper rpcResponseJSON
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return err
	} else if wrapper.V != "" && wrapper.V != "2.0" {
		return fmt.Errorf("invalid version: %q", wrapper.V)
	}

	if wrapper.ID != nil {
		id, err := idFromInterface(wrapper.ID)
		if err != nil {
			return fmt.Errorf("invalid response ID: %w", err)
		}
		resp.ID = id
	}

	resp.Error = wrapper.E
	resp.Result = wrapper.R
	return nil
}

// MarshalJSON marshals a response with the appropriate version tag.
func (resp RPCResponse) MarshalJSON() ([]byte, error) {
	return json.Marshal(rpcResponseJSON{
		V:  "2.0",
		ID: resp.ID,
		R:  resp.Result,
		E:  resp.Error,
	})
}

func (resp RPCResponse) String() string {
	if resp.Error == nil {
		return fmt.Sprintf("RPCResponse{%s %X}", resp.ID, resp.Result)
	}
	return fmt.Sprintf("RPCResponse{%s %v}", resp.ID, resp.Error)
}

//----------------------------------------

// WSRPCConnection represents a websocket connection.
type WSRPCConnection interface {
	// GetRemoteAddr returns a remote address of the connection.
	GetRemoteAddr() string
	// WriteRPCResponse writes the response onto connection (BLOCKING).
	WriteRPCResponse(context.Context, RPCResponse) error
	// TryWriteRPCResponse tries to write the response onto connection (NON-BLOCKING).
	TryWriteRPCResponse(context.Context, RPCResponse) bool
	// Context returns the connection's context.
	Context() context.Context
}

// CallInfo carries JSON-RPC request metadata for RPC functions invoked via
// JSON-RPC. It can be recovered from the context with GetCallInfo.
type CallInfo struct {
	RPCRequest  *RPCRequest     // non-nil for requests via HTTP or websocket
	HTTPRequest *http.Request   // non-nil for requests via HTTP
	WSConn      WSRPCConnection // non-nil for requests via websocket
}

type callInfoKey struct{}

// WithCallInfo returns a child context of ctx with the ci attached.
func WithCallInfo(ctx context.Context, ci *CallInfo) context.Context {
	return context.WithValue(ctx, callInfoKey{}, ci)
}

// GetCallInfo returns the CallInfo record attached to ctx, or nil if ctx does
// not contain a call record.
func GetCallInfo(ctx context.Context) *CallInfo {
	if v := ctx.Value(callInfoKey{}); v != nil {
		return v.(*CallInfo)
	}
	return nil
}

// RemoteAddr returns the remote address (usually a string "IP:port").  If
// neither HTTPRequest nor WSConn is set, an empty string is returned.
//
// For HTTP requests, this reports the request's RemoteAddr.
// For websocket requests, this reports the connection's GetRemoteAddr.
func (ci *CallInfo) RemoteAddr() string {
	if ci == nil {
		return ""
	} else if ci.HTTPRequest != nil {
		return ci.HTTPRequest.RemoteAddr
	} else if ci.WSConn != nil {
		return ci.WSConn.GetRemoteAddr()
	}
	return ""
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
