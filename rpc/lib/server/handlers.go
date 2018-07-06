package rpcserver

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"

	amino "github.com/tendermint/go-amino"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	types "github.com/tendermint/tendermint/rpc/lib/types"
)

// RegisterRPCFuncs adds a route for each function in the funcMap, as well as general jsonrpc and websocket handlers for all functions.
// "result" is the interface on which the result objects are registered, and is popualted with every RPCResponse
func RegisterRPCFuncs(mux *http.ServeMux, funcMap map[string]*RPCFunc, cdc *amino.Codec, logger log.Logger) {
	// HTTP endpoints
	for funcName, rpcFunc := range funcMap {
		mux.HandleFunc("/"+funcName, makeHTTPHandler(rpcFunc, cdc, logger))
	}

	// JSONRPC endpoints
	mux.HandleFunc("/", handleInvalidJSONRPCPaths(makeJSONRPCHandler(funcMap, cdc, logger)))
}

//-------------------------------------
// function introspection

// RPCFunc contains the introspected type information for a function
type RPCFunc struct {
	f        reflect.Value  // underlying rpc function
	args     []reflect.Type // type of each function arg
	returns  []reflect.Type // type of each return arg
	argNames []string       // name of each argument
	ws       bool           // websocket only
}

// NewRPCFunc wraps a function for introspection.
// f is the function, args are comma separated argument names
func NewRPCFunc(f interface{}, args string) *RPCFunc {
	return newRPCFunc(f, args, false)
}

// NewWSRPCFunc wraps a function for introspection and use in the websockets.
func NewWSRPCFunc(f interface{}, args string) *RPCFunc {
	return newRPCFunc(f, args, true)
}

func newRPCFunc(f interface{}, args string, ws bool) *RPCFunc {
	var argNames []string
	if args != "" {
		argNames = strings.Split(args, ",")
	}
	return &RPCFunc{
		f:        reflect.ValueOf(f),
		args:     funcArgTypes(f),
		returns:  funcReturnTypes(f),
		argNames: argNames,
		ws:       ws,
	}
}

// return a function's argument types
func funcArgTypes(f interface{}) []reflect.Type {
	t := reflect.TypeOf(f)
	n := t.NumIn()
	typez := make([]reflect.Type, n)
	for i := 0; i < n; i++ {
		typez[i] = t.In(i)
	}
	return typez
}

// return a function's return types
func funcReturnTypes(f interface{}) []reflect.Type {
	t := reflect.TypeOf(f)
	n := t.NumOut()
	typez := make([]reflect.Type, n)
	for i := 0; i < n; i++ {
		typez[i] = t.Out(i)
	}
	return typez
}

// function introspection
//-----------------------------------------------------------------------------
// rpc.json

// jsonrpc calls grab the given method's function info and runs reflect.Call
func makeJSONRPCHandler(funcMap map[string]*RPCFunc, cdc *amino.Codec, logger log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			WriteRPCResponseHTTP(w, types.RPCInvalidRequestError("", errors.Wrap(err, "Error reading request body")))
			return
		}
		// if its an empty request (like from a browser),
		// just display a list of functions
		if len(b) == 0 {
			writeListOfEndpoints(w, r, funcMap)
			return
		}

		var request types.RPCRequest
		err = json.Unmarshal(b, &request)
		if err != nil {
			WriteRPCResponseHTTP(w, types.RPCParseError("", errors.Wrap(err, "Error unmarshalling request")))
			return
		}
		// A Notification is a Request object without an "id" member.
		// The Server MUST NOT reply to a Notification, including those that are within a batch request.
		if request.ID == "" {
			logger.Debug("HTTPJSONRPC received a notification, skipping... (please send a non-empty ID if you want to call a method)")
			return
		}
		if len(r.URL.Path) > 1 {
			WriteRPCResponseHTTP(w, types.RPCInvalidRequestError(request.ID, errors.Errorf("Path %s is invalid", r.URL.Path)))
			return
		}
		rpcFunc := funcMap[request.Method]
		if rpcFunc == nil || rpcFunc.ws {
			WriteRPCResponseHTTP(w, types.RPCMethodNotFoundError(request.ID))
			return
		}
		var args []reflect.Value
		if len(request.Params) > 0 {
			args, err = jsonParamsToArgsRPC(rpcFunc, cdc, request.Params)
			if err != nil {
				WriteRPCResponseHTTP(w, types.RPCInvalidParamsError(request.ID, errors.Wrap(err, "Error converting json params to arguments")))
				return
			}
		}
		returns := rpcFunc.f.Call(args)
		logger.Info("HTTPJSONRPC", "method", request.Method, "args", args, "returns", returns)
		result, err := unreflectResult(returns)
		if err != nil {
			WriteRPCResponseHTTP(w, types.RPCInternalError(request.ID, err))
			return
		}
		WriteRPCResponseHTTP(w, types.NewRPCSuccessResponse(cdc, request.ID, result))
	}
}

func handleInvalidJSONRPCPaths(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Since the pattern "/" matches all paths not matched by other registered patterns we check whether the path is indeed
		// "/", otherwise return a 404 error
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		next(w, r)
	}
}

func mapParamsToArgs(rpcFunc *RPCFunc, cdc *amino.Codec, params map[string]json.RawMessage, argsOffset int) ([]reflect.Value, error) {
	values := make([]reflect.Value, len(rpcFunc.argNames))
	for i, argName := range rpcFunc.argNames {
		argType := rpcFunc.args[i+argsOffset]

		if p, ok := params[argName]; ok && p != nil && len(p) > 0 {
			val := reflect.New(argType)
			err := cdc.UnmarshalJSON(p, val.Interface())
			if err != nil {
				return nil, err
			}
			values[i] = val.Elem()
		} else { // use default for that type
			values[i] = reflect.Zero(argType)
		}
	}

	return values, nil
}

func arrayParamsToArgs(rpcFunc *RPCFunc, cdc *amino.Codec, params []json.RawMessage, argsOffset int) ([]reflect.Value, error) {
	if len(rpcFunc.argNames) != len(params) {
		return nil, errors.Errorf("Expected %v parameters (%v), got %v (%v)",
			len(rpcFunc.argNames), rpcFunc.argNames, len(params), params)
	}

	values := make([]reflect.Value, len(params))
	for i, p := range params {
		argType := rpcFunc.args[i+argsOffset]
		val := reflect.New(argType)
		err := cdc.UnmarshalJSON(p, val.Interface())
		if err != nil {
			return nil, err
		}
		values[i] = val.Elem()
	}
	return values, nil
}

// `raw` is unparsed json (from json.RawMessage) encoding either a map or an array.
// `argsOffset` should be 0 for RPC calls, and 1 for WS requests, where len(rpcFunc.args) != len(rpcFunc.argNames).
//
// Example:
//   rpcFunc.args = [rpctypes.WSRPCContext string]
//   rpcFunc.argNames = ["arg"]
func jsonParamsToArgs(rpcFunc *RPCFunc, cdc *amino.Codec, raw []byte, argsOffset int) ([]reflect.Value, error) {

	// TODO: Make more efficient, perhaps by checking the first character for '{' or '['?
	// First, try to get the map.
	var m map[string]json.RawMessage
	err := json.Unmarshal(raw, &m)
	if err == nil {
		return mapParamsToArgs(rpcFunc, cdc, m, argsOffset)
	}

	// Otherwise, try an array.
	var a []json.RawMessage
	err = json.Unmarshal(raw, &a)
	if err == nil {
		return arrayParamsToArgs(rpcFunc, cdc, a, argsOffset)
	}

	// Otherwise, bad format, we cannot parse
	return nil, errors.Errorf("Unknown type for JSON params: %v. Expected map or array", err)
}

// Convert a []interface{} OR a map[string]interface{} to properly typed values
func jsonParamsToArgsRPC(rpcFunc *RPCFunc, cdc *amino.Codec, params json.RawMessage) ([]reflect.Value, error) {
	return jsonParamsToArgs(rpcFunc, cdc, params, 0)
}

// Same as above, but with the first param the websocket connection
func jsonParamsToArgsWS(rpcFunc *RPCFunc, cdc *amino.Codec, params json.RawMessage, wsCtx types.WSRPCContext) ([]reflect.Value, error) {
	values, err := jsonParamsToArgs(rpcFunc, cdc, params, 1)
	if err != nil {
		return nil, err
	}
	return append([]reflect.Value{reflect.ValueOf(wsCtx)}, values...), nil
}

// rpc.json
//-----------------------------------------------------------------------------
// rpc.http

// convert from a function name to the http handler
func makeHTTPHandler(rpcFunc *RPCFunc, cdc *amino.Codec, logger log.Logger) func(http.ResponseWriter, *http.Request) {
	// Exception for websocket endpoints
	if rpcFunc.ws {
		return func(w http.ResponseWriter, r *http.Request) {
			WriteRPCResponseHTTP(w, types.RPCMethodNotFoundError(""))
		}
	}
	// All other endpoints
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("HTTP HANDLER", "req", r)
		args, err := httpParamsToArgs(rpcFunc, cdc, r)
		if err != nil {
			WriteRPCResponseHTTP(w, types.RPCInvalidParamsError("", errors.Wrap(err, "Error converting http params to arguments")))
			return
		}
		returns := rpcFunc.f.Call(args)
		logger.Info("HTTPRestRPC", "method", r.URL.Path, "args", args, "returns", returns)
		result, err := unreflectResult(returns)
		if err != nil {
			WriteRPCResponseHTTP(w, types.RPCInternalError("", err))
			return
		}
		WriteRPCResponseHTTP(w, types.NewRPCSuccessResponse(cdc, "", result))
	}
}

// Covert an http query to a list of properly typed values.
// To be properly decoded the arg must be a concrete type from tendermint (if its an interface).
func httpParamsToArgs(rpcFunc *RPCFunc, cdc *amino.Codec, r *http.Request) ([]reflect.Value, error) {
	values := make([]reflect.Value, len(rpcFunc.args))

	for i, name := range rpcFunc.argNames {
		argType := rpcFunc.args[i]

		values[i] = reflect.Zero(argType) // set default for that type

		arg := GetParam(r, name)
		// log.Notice("param to arg", "argType", argType, "name", name, "arg", arg)

		if "" == arg {
			continue
		}

		v, err, ok := nonJSONStringToArg(cdc, argType, arg)
		if err != nil {
			return nil, err
		}
		if ok {
			values[i] = v
			continue
		}

		values[i], err = jsonStringToArg(cdc, argType, arg)
		if err != nil {
			return nil, err
		}
	}

	return values, nil
}

func jsonStringToArg(cdc *amino.Codec, rt reflect.Type, arg string) (reflect.Value, error) {
	rv := reflect.New(rt)
	err := cdc.UnmarshalJSON([]byte(arg), rv.Interface())
	if err != nil {
		return rv, err
	}
	rv = rv.Elem()
	return rv, nil
}

func nonJSONStringToArg(cdc *amino.Codec, rt reflect.Type, arg string) (reflect.Value, error, bool) {
	if rt.Kind() == reflect.Ptr {
		rv_, err, ok := nonJSONStringToArg(cdc, rt.Elem(), arg)
		if err != nil {
			return reflect.Value{}, err, false
		} else if ok {
			rv := reflect.New(rt.Elem())
			rv.Elem().Set(rv_)
			return rv, nil, true
		} else {
			return reflect.Value{}, nil, false
		}
	} else {
		return _nonJSONStringToArg(cdc, rt, arg)
	}
}

// NOTE: rt.Kind() isn't a pointer.
func _nonJSONStringToArg(cdc *amino.Codec, rt reflect.Type, arg string) (reflect.Value, error, bool) {
	isIntString := RE_INT.Match([]byte(arg))
	isQuotedString := strings.HasPrefix(arg, `"`) && strings.HasSuffix(arg, `"`)
	isHexString := strings.HasPrefix(strings.ToLower(arg), "0x")

	var expectingString, expectingByteSlice, expectingInt bool
	switch rt.Kind() {
	case reflect.Int, reflect.Uint, reflect.Int8, reflect.Uint8, reflect.Int16, reflect.Uint16, reflect.Int32, reflect.Uint32, reflect.Int64, reflect.Uint64:
		expectingInt = true
	case reflect.String:
		expectingString = true
	case reflect.Slice:
		expectingByteSlice = rt.Elem().Kind() == reflect.Uint8
	}

	if isIntString && expectingInt {
		qarg := `"` + arg + `"`
		// jsonStringToArg
		rv, err := jsonStringToArg(cdc, rt, qarg)
		if err != nil {
			return rv, err, false
		} else {
			return rv, nil, true
		}
	}

	if isHexString {
		if !expectingString && !expectingByteSlice {
			err := errors.Errorf("Got a hex string arg, but expected '%s'",
				rt.Kind().String())
			return reflect.ValueOf(nil), err, false
		}

		var value []byte
		value, err := hex.DecodeString(arg[2:])
		if err != nil {
			return reflect.ValueOf(nil), err, false
		}
		if rt.Kind() == reflect.String {
			return reflect.ValueOf(string(value)), nil, true
		}
		return reflect.ValueOf([]byte(value)), nil, true
	}

	if isQuotedString && expectingByteSlice {
		v := reflect.New(reflect.TypeOf(""))
		err := cdc.UnmarshalJSON([]byte(arg), v.Interface())
		if err != nil {
			return reflect.ValueOf(nil), err, false
		}
		v = v.Elem()
		return reflect.ValueOf([]byte(v.String())), nil, true
	}

	return reflect.ValueOf(nil), nil, false
}

// rpc.http
//-----------------------------------------------------------------------------
// rpc.websocket

const (
	defaultWSWriteChanCapacity = 1000
	defaultWSWriteWait         = 10 * time.Second
	defaultWSReadWait          = 30 * time.Second
	defaultWSPingPeriod        = (defaultWSReadWait * 9) / 10
)

// A single websocket connection contains listener id, underlying ws
// connection, and the event switch for subscribing to events.
//
// In case of an error, the connection is stopped.
type wsConnection struct {
	cmn.BaseService

	remoteAddr string
	baseConn   *websocket.Conn
	writeChan  chan types.RPCResponse

	funcMap map[string]*RPCFunc
	cdc     *amino.Codec

	// write channel capacity
	writeChanCapacity int

	// each write times out after this.
	writeWait time.Duration

	// Connection times out if we haven't received *anything* in this long, not even pings.
	readWait time.Duration

	// Send pings to server with this period. Must be less than readWait, but greater than zero.
	pingPeriod time.Duration

	// object that is used to subscribe / unsubscribe from events
	eventSub types.EventSubscriber
}

// NewWSConnection wraps websocket.Conn.
//
// See the commentary on the func(*wsConnection) functions for a detailed
// description of how to configure ping period and pong wait time. NOTE: if the
// write buffer is full, pongs may be dropped, which may cause clients to
// disconnect. see https://github.com/gorilla/websocket/issues/97
func NewWSConnection(
	baseConn *websocket.Conn,
	funcMap map[string]*RPCFunc,
	cdc *amino.Codec,
	options ...func(*wsConnection),
) *wsConnection {
	baseConn.SetReadLimit(maxBodyBytes)
	wsc := &wsConnection{
		remoteAddr:        baseConn.RemoteAddr().String(),
		baseConn:          baseConn,
		funcMap:           funcMap,
		cdc:               cdc,
		writeWait:         defaultWSWriteWait,
		writeChanCapacity: defaultWSWriteChanCapacity,
		readWait:          defaultWSReadWait,
		pingPeriod:        defaultWSPingPeriod,
	}
	for _, option := range options {
		option(wsc)
	}
	wsc.BaseService = *cmn.NewBaseService(nil, "wsConnection", wsc)
	return wsc
}

// EventSubscriber sets object that is used to subscribe / unsubscribe from
// events - not Goroutine-safe. If none given, default node's eventBus will be
// used.
func EventSubscriber(eventSub types.EventSubscriber) func(*wsConnection) {
	return func(wsc *wsConnection) {
		wsc.eventSub = eventSub
	}
}

// WriteWait sets the amount of time to wait before a websocket write times out.
// It should only be used in the constructor - not Goroutine-safe.
func WriteWait(writeWait time.Duration) func(*wsConnection) {
	return func(wsc *wsConnection) {
		wsc.writeWait = writeWait
	}
}

// WriteChanCapacity sets the capacity of the websocket write channel.
// It should only be used in the constructor - not Goroutine-safe.
func WriteChanCapacity(cap int) func(*wsConnection) {
	return func(wsc *wsConnection) {
		wsc.writeChanCapacity = cap
	}
}

// ReadWait sets the amount of time to wait before a websocket read times out.
// It should only be used in the constructor - not Goroutine-safe.
func ReadWait(readWait time.Duration) func(*wsConnection) {
	return func(wsc *wsConnection) {
		wsc.readWait = readWait
	}
}

// PingPeriod sets the duration for sending websocket pings.
// It should only be used in the constructor - not Goroutine-safe.
func PingPeriod(pingPeriod time.Duration) func(*wsConnection) {
	return func(wsc *wsConnection) {
		wsc.pingPeriod = pingPeriod
	}
}

// OnStart implements cmn.Service by starting the read and write routines. It
// blocks until the connection closes.
func (wsc *wsConnection) OnStart() error {
	wsc.writeChan = make(chan types.RPCResponse, wsc.writeChanCapacity)

	// Read subscriptions/unsubscriptions to events
	go wsc.readRoutine()
	// Write responses, BLOCKING.
	wsc.writeRoutine()

	return nil
}

// OnStop implements cmn.Service by unsubscribing remoteAddr from all subscriptions.
func (wsc *wsConnection) OnStop() {
	// Both read and write loops close the websocket connection when they exit their loops.
	// The writeChan is never closed, to allow WriteRPCResponse() to fail.
	if wsc.eventSub != nil {
		wsc.eventSub.UnsubscribeAll(context.TODO(), wsc.remoteAddr)
	}
}

// GetRemoteAddr returns the remote address of the underlying connection.
// It implements WSRPCConnection
func (wsc *wsConnection) GetRemoteAddr() string {
	return wsc.remoteAddr
}

// GetEventSubscriber implements WSRPCConnection by returning event subscriber.
func (wsc *wsConnection) GetEventSubscriber() types.EventSubscriber {
	return wsc.eventSub
}

// WriteRPCResponse pushes a response to the writeChan, and blocks until it is accepted.
// It implements WSRPCConnection. It is Goroutine-safe.
func (wsc *wsConnection) WriteRPCResponse(resp types.RPCResponse) {
	select {
	case <-wsc.Quit():
		return
	case wsc.writeChan <- resp:
	}
}

// TryWriteRPCResponse attempts to push a response to the writeChan, but does not block.
// It implements WSRPCConnection. It is Goroutine-safe
func (wsc *wsConnection) TryWriteRPCResponse(resp types.RPCResponse) bool {
	select {
	case <-wsc.Quit():
		return false
	case wsc.writeChan <- resp:
		return true
	default:
		return false
	}
}

// Codec returns an amino codec used to decode parameters and encode results.
// It implements WSRPCConnection.
func (wsc *wsConnection) Codec() *amino.Codec {
	return wsc.cdc
}

// Read from the socket and subscribe to or unsubscribe from events
func (wsc *wsConnection) readRoutine() {
	defer func() {
		if r := recover(); r != nil {
			err, ok := r.(error)
			if !ok {
				err = fmt.Errorf("WSJSONRPC: %v", r)
			}
			wsc.Logger.Error("Panic in WSJSONRPC handler", "err", err, "stack", string(debug.Stack()))
			wsc.WriteRPCResponse(types.RPCInternalError("unknown", err))
			go wsc.readRoutine()
		} else {
			wsc.baseConn.Close() // nolint: errcheck
		}
	}()

	wsc.baseConn.SetPongHandler(func(m string) error {
		return wsc.baseConn.SetReadDeadline(time.Now().Add(wsc.readWait))
	})

	for {
		select {
		case <-wsc.Quit():
			return
		default:
			// reset deadline for every type of message (control or data)
			if err := wsc.baseConn.SetReadDeadline(time.Now().Add(wsc.readWait)); err != nil {
				wsc.Logger.Error("failed to set read deadline", "err", err)
			}
			var in []byte
			_, in, err := wsc.baseConn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					wsc.Logger.Info("Client closed the connection")
				} else {
					wsc.Logger.Error("Failed to read request", "err", err)
				}
				wsc.Stop()
				return
			}

			var request types.RPCRequest
			err = json.Unmarshal(in, &request)
			if err != nil {
				wsc.WriteRPCResponse(types.RPCParseError("", errors.Wrap(err, "Error unmarshaling request")))
				continue
			}

			// A Notification is a Request object without an "id" member.
			// The Server MUST NOT reply to a Notification, including those that are within a batch request.
			if request.ID == "" {
				wsc.Logger.Debug("WSJSONRPC received a notification, skipping... (please send a non-empty ID if you want to call a method)")
				continue
			}

			// Now, fetch the RPCFunc and execute it.

			rpcFunc := wsc.funcMap[request.Method]
			if rpcFunc == nil {
				wsc.WriteRPCResponse(types.RPCMethodNotFoundError(request.ID))
				continue
			}
			var args []reflect.Value
			if rpcFunc.ws {
				wsCtx := types.WSRPCContext{Request: request, WSRPCConnection: wsc}
				if len(request.Params) > 0 {
					args, err = jsonParamsToArgsWS(rpcFunc, wsc.cdc, request.Params, wsCtx)
				}
			} else {
				if len(request.Params) > 0 {
					args, err = jsonParamsToArgsRPC(rpcFunc, wsc.cdc, request.Params)
				}
			}
			if err != nil {
				wsc.WriteRPCResponse(types.RPCInternalError(request.ID, errors.Wrap(err, "Error converting json params to arguments")))
				continue
			}
			returns := rpcFunc.f.Call(args)

			// TODO: Need to encode args/returns to string if we want to log them
			wsc.Logger.Info("WSJSONRPC", "method", request.Method)

			result, err := unreflectResult(returns)
			if err != nil {
				wsc.WriteRPCResponse(types.RPCInternalError(request.ID, err))
				continue
			}

			wsc.WriteRPCResponse(types.NewRPCSuccessResponse(wsc.cdc, request.ID, result))
		}
	}
}

// receives on a write channel and writes out on the socket
func (wsc *wsConnection) writeRoutine() {
	pingTicker := time.NewTicker(wsc.pingPeriod)
	defer func() {
		pingTicker.Stop()
		if err := wsc.baseConn.Close(); err != nil {
			wsc.Logger.Error("Error closing connection", "err", err)
		}
	}()

	// https://github.com/gorilla/websocket/issues/97
	pongs := make(chan string, 1)
	wsc.baseConn.SetPingHandler(func(m string) error {
		select {
		case pongs <- m:
		default:
		}
		return nil
	})

	for {
		select {
		case m := <-pongs:
			err := wsc.writeMessageWithDeadline(websocket.PongMessage, []byte(m))
			if err != nil {
				wsc.Logger.Info("Failed to write pong (client may disconnect)", "err", err)
			}
		case <-pingTicker.C:
			err := wsc.writeMessageWithDeadline(websocket.PingMessage, []byte{})
			if err != nil {
				wsc.Logger.Error("Failed to write ping", "err", err)
				wsc.Stop()
				return
			}
		case msg := <-wsc.writeChan:
			jsonBytes, err := json.MarshalIndent(msg, "", "  ")
			if err != nil {
				wsc.Logger.Error("Failed to marshal RPCResponse to JSON", "err", err)
			} else {
				if err = wsc.writeMessageWithDeadline(websocket.TextMessage, jsonBytes); err != nil {
					wsc.Logger.Error("Failed to write response", "err", err)
					wsc.Stop()
					return
				}
			}
		case <-wsc.Quit():
			return
		}
	}
}

// All writes to the websocket must (re)set the write deadline.
// If some writes don't set it while others do, they may timeout incorrectly (https://github.com/tendermint/tendermint/issues/553)
func (wsc *wsConnection) writeMessageWithDeadline(msgType int, msg []byte) error {
	if err := wsc.baseConn.SetWriteDeadline(time.Now().Add(wsc.writeWait)); err != nil {
		return err
	}
	return wsc.baseConn.WriteMessage(msgType, msg)
}

//----------------------------------------

// WebsocketManager provides a WS handler for incoming connections and passes a
// map of functions along with any additional params to new connections.
// NOTE: The websocket path is defined externally, e.g. in node/node.go
type WebsocketManager struct {
	websocket.Upgrader

	funcMap       map[string]*RPCFunc
	cdc           *amino.Codec
	logger        log.Logger
	wsConnOptions []func(*wsConnection)
}

// NewWebsocketManager returns a new WebsocketManager that passes a map of
// functions, connection options and logger to new WS connections.
func NewWebsocketManager(funcMap map[string]*RPCFunc, cdc *amino.Codec, wsConnOptions ...func(*wsConnection)) *WebsocketManager {
	return &WebsocketManager{
		funcMap: funcMap,
		cdc:     cdc,
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// TODO ???
				return true
			},
		},
		logger:        log.NewNopLogger(),
		wsConnOptions: wsConnOptions,
	}
}

// SetLogger sets the logger.
func (wm *WebsocketManager) SetLogger(l log.Logger) {
	wm.logger = l
}

// WebsocketHandler upgrades the request/response (via http.Hijack) and starts
// the wsConnection.
func (wm *WebsocketManager) WebsocketHandler(w http.ResponseWriter, r *http.Request) {
	wsConn, err := wm.Upgrade(w, r, nil)
	if err != nil {
		// TODO - return http error
		wm.logger.Error("Failed to upgrade to websocket connection", "err", err)
		return
	}

	// register connection
	con := NewWSConnection(wsConn, wm.funcMap, wm.cdc, wm.wsConnOptions...)
	con.SetLogger(wm.logger.With("remote", wsConn.RemoteAddr()))
	wm.logger.Info("New websocket connection", "remote", con.remoteAddr)
	err = con.Start() // Blocking
	if err != nil {
		wm.logger.Error("Error starting connection", "err", err)
	}
}

// rpc.websocket
//-----------------------------------------------------------------------------

// NOTE: assume returns is result struct and error. If error is not nil, return it
func unreflectResult(returns []reflect.Value) (interface{}, error) {
	errV := returns[1]
	if errV.Interface() != nil {
		return nil, errors.Errorf("%v", errV.Interface())
	}
	rv := returns[0]
	// the result is a registered interface,
	// we need a pointer to it so we can marshal with type byte
	rvp := reflect.New(rv.Type())
	rvp.Elem().Set(rv)
	return rvp.Interface(), nil
}

// writes a list of available rpc endpoints as an html page
func writeListOfEndpoints(w http.ResponseWriter, r *http.Request, funcMap map[string]*RPCFunc) {
	noArgNames := []string{}
	argNames := []string{}
	for name, funcData := range funcMap {
		if len(funcData.args) == 0 {
			noArgNames = append(noArgNames, name)
		} else {
			argNames = append(argNames, name)
		}
	}
	sort.Strings(noArgNames)
	sort.Strings(argNames)
	buf := new(bytes.Buffer)
	buf.WriteString("<html><body>")
	buf.WriteString("<br>Available endpoints:<br>")

	for _, name := range noArgNames {
		link := fmt.Sprintf("//%s/%s", r.Host, name)
		buf.WriteString(fmt.Sprintf("<a href=\"%s\">%s</a></br>", link, link))
	}

	buf.WriteString("<br>Endpoints that require arguments:<br>")
	for _, name := range argNames {
		link := fmt.Sprintf("//%s/%s?", r.Host, name)
		funcData := funcMap[name]
		for i, argName := range funcData.argNames {
			link += argName + "=_"
			if i < len(funcData.argNames)-1 {
				link += "&"
			}
		}
		buf.WriteString(fmt.Sprintf("<a href=\"%s\">%s</a></br>", link, link))
	}
	buf.WriteString("</body></html>")
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(200)
	w.Write(buf.Bytes()) // nolint: errcheck
}
