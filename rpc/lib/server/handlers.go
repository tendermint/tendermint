package rpcserver

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"

	types "github.com/tendermint/tendermint/rpc/lib/types"
	cmn "github.com/tendermint/tmlibs/common"
	events "github.com/tendermint/tmlibs/events"
	"github.com/tendermint/tmlibs/log"
)

// Adds a route for each function in the funcMap, as well as general jsonrpc and websocket handlers for all functions.
// "result" is the interface on which the result objects are registered, and is popualted with every RPCResponse
func RegisterRPCFuncs(mux *http.ServeMux, funcMap map[string]*RPCFunc, logger log.Logger) {
	// HTTP endpoints
	for funcName, rpcFunc := range funcMap {
		mux.HandleFunc("/"+funcName, makeHTTPHandler(rpcFunc, logger))
	}

	// JSONRPC endpoints
	mux.HandleFunc("/", makeJSONRPCHandler(funcMap, logger))
}

//-------------------------------------
// function introspection

// holds all type information for each function
type RPCFunc struct {
	f        reflect.Value  // underlying rpc function
	args     []reflect.Type // type of each function arg
	returns  []reflect.Type // type of each return arg
	argNames []string       // name of each argument
	ws       bool           // websocket only
}

// wraps a function for quicker introspection
// f is the function, args are comma separated argument names
func NewRPCFunc(f interface{}, args string) *RPCFunc {
	return newRPCFunc(f, args, false)
}

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
func makeJSONRPCHandler(funcMap map[string]*RPCFunc, logger log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, _ := ioutil.ReadAll(r.Body)
		// if its an empty request (like from a browser),
		// just display a list of functions
		if len(b) == 0 {
			writeListOfEndpoints(w, r, funcMap)
			return
		}

		var request types.RPCRequest
		err := json.Unmarshal(b, &request)
		if err != nil {
			WriteRPCResponseHTTPError(w, http.StatusBadRequest, types.NewRPCResponse("", nil, fmt.Sprintf("Error unmarshalling request: %v", err.Error())))
			return
		}
		if len(r.URL.Path) > 1 {
			WriteRPCResponseHTTPError(w, http.StatusNotFound, types.NewRPCResponse(request.ID, nil, fmt.Sprintf("Invalid JSONRPC endpoint %s", r.URL.Path)))
			return
		}
		rpcFunc := funcMap[request.Method]
		if rpcFunc == nil {
			WriteRPCResponseHTTPError(w, http.StatusNotFound, types.NewRPCResponse(request.ID, nil, "RPC method unknown: "+request.Method))
			return
		}
		if rpcFunc.ws {
			WriteRPCResponseHTTPError(w, http.StatusMethodNotAllowed, types.NewRPCResponse(request.ID, nil, "RPC method is only for websockets: "+request.Method))
			return
		}
		args, err := jsonParamsToArgsRPC(rpcFunc, request.Params)
		if err != nil {
			WriteRPCResponseHTTPError(w, http.StatusBadRequest, types.NewRPCResponse(request.ID, nil, fmt.Sprintf("Error converting json params to arguments: %v", err.Error())))
			return
		}
		returns := rpcFunc.f.Call(args)
		logger.Info("HTTPJSONRPC", "method", request.Method, "args", args, "returns", returns)
		result, err := unreflectResult(returns)
		if err != nil {
			WriteRPCResponseHTTPError(w, http.StatusInternalServerError, types.NewRPCResponse(request.ID, result, err.Error()))
			return
		}
		WriteRPCResponseHTTP(w, types.NewRPCResponse(request.ID, result, ""))
	}
}

func mapParamsToArgs(rpcFunc *RPCFunc, params map[string]*json.RawMessage, argsOffset int) ([]reflect.Value, error) {
	values := make([]reflect.Value, len(rpcFunc.argNames))
	for i, argName := range rpcFunc.argNames {
		argType := rpcFunc.args[i+argsOffset]

		if p, ok := params[argName]; ok && p != nil && len(*p) > 0 {
			val := reflect.New(argType)
			err := json.Unmarshal(*p, val.Interface())
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

func arrayParamsToArgs(rpcFunc *RPCFunc, params []*json.RawMessage, argsOffset int) ([]reflect.Value, error) {
	if len(rpcFunc.argNames) != len(params) {
		return nil, errors.Errorf("Expected %v parameters (%v), got %v (%v)",
			len(rpcFunc.argNames), rpcFunc.argNames, len(params), params)
	}

	values := make([]reflect.Value, len(params))
	for i, p := range params {
		argType := rpcFunc.args[i+argsOffset]
		val := reflect.New(argType)
		err := json.Unmarshal(*p, val.Interface())
		if err != nil {
			return nil, err
		}
		values[i] = val.Elem()
	}
	return values, nil
}

// raw is unparsed json (from json.RawMessage) encoding either a map or an array.
//
// argsOffset should be 0 for RPC calls, and 1 for WS requests, where len(rpcFunc.args) != len(rpcFunc.argNames).
// Example:
//   rpcFunc.args = [rpctypes.WSRPCContext string]
//   rpcFunc.argNames = ["arg"]
func jsonParamsToArgs(rpcFunc *RPCFunc, raw []byte, argsOffset int) ([]reflect.Value, error) {
	// first, try to get the map..
	var m map[string]*json.RawMessage
	err := json.Unmarshal(raw, &m)
	if err == nil {
		return mapParamsToArgs(rpcFunc, m, argsOffset)
	}

	// otherwise, try an array
	var a []*json.RawMessage
	err = json.Unmarshal(raw, &a)
	if err == nil {
		return arrayParamsToArgs(rpcFunc, a, argsOffset)
	}

	// otherwise, bad format, we cannot parse
	return nil, errors.Errorf("Unknown type for JSON params: %v. Expected map or array", err)
}

// Convert a []interface{} OR a map[string]interface{} to properly typed values
func jsonParamsToArgsRPC(rpcFunc *RPCFunc, params *json.RawMessage) ([]reflect.Value, error) {
	return jsonParamsToArgs(rpcFunc, *params, 0)
}

// Same as above, but with the first param the websocket connection
func jsonParamsToArgsWS(rpcFunc *RPCFunc, params *json.RawMessage, wsCtx types.WSRPCContext) ([]reflect.Value, error) {
	values, err := jsonParamsToArgs(rpcFunc, *params, 1)
	if err != nil {
		return nil, err
	}
	return append([]reflect.Value{reflect.ValueOf(wsCtx)}, values...), nil
}

// rpc.json
//-----------------------------------------------------------------------------
// rpc.http

// convert from a function name to the http handler
func makeHTTPHandler(rpcFunc *RPCFunc, logger log.Logger) func(http.ResponseWriter, *http.Request) {
	// Exception for websocket endpoints
	if rpcFunc.ws {
		return func(w http.ResponseWriter, r *http.Request) {
			WriteRPCResponseHTTPError(w, http.StatusMethodNotAllowed, types.NewRPCResponse("", nil, "This RPC method is only for websockets"))
		}
	}
	// All other endpoints
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("HTTP HANDLER", "req", r)
		args, err := httpParamsToArgs(rpcFunc, r)
		if err != nil {
			WriteRPCResponseHTTPError(w, http.StatusBadRequest, types.NewRPCResponse("", nil, fmt.Sprintf("Error converting http params to args: %v", err.Error())))
			return
		}
		returns := rpcFunc.f.Call(args)
		logger.Info("HTTPRestRPC", "method", r.URL.Path, "args", args, "returns", returns)
		result, err := unreflectResult(returns)
		if err != nil {
			WriteRPCResponseHTTPError(w, http.StatusInternalServerError, types.NewRPCResponse("", nil, err.Error()))
			return
		}
		WriteRPCResponseHTTP(w, types.NewRPCResponse("", result, ""))
	}
}

// Covert an http query to a list of properly typed values.
// To be properly decoded the arg must be a concrete type from tendermint (if its an interface).
func httpParamsToArgs(rpcFunc *RPCFunc, r *http.Request) ([]reflect.Value, error) {
	values := make([]reflect.Value, len(rpcFunc.args))

	for i, name := range rpcFunc.argNames {
		argType := rpcFunc.args[i]

		values[i] = reflect.Zero(argType) // set default for that type

		arg := GetParam(r, name)
		// log.Notice("param to arg", "argType", argType, "name", name, "arg", arg)

		if "" == arg {
			continue
		}

		v, err, ok := nonJsonToArg(argType, arg)
		if err != nil {
			return nil, err
		}
		if ok {
			values[i] = v
			continue
		}

		values[i], err = _jsonStringToArg(argType, arg)
		if err != nil {
			return nil, err
		}
	}

	return values, nil
}

func _jsonStringToArg(ty reflect.Type, arg string) (reflect.Value, error) {
	v := reflect.New(ty)
	err := json.Unmarshal([]byte(arg), v.Interface())
	if err != nil {
		return v, err
	}
	v = v.Elem()
	return v, nil
}

func nonJsonToArg(ty reflect.Type, arg string) (reflect.Value, error, bool) {
	isQuotedString := strings.HasPrefix(arg, `"`) && strings.HasSuffix(arg, `"`)
	isHexString := strings.HasPrefix(strings.ToLower(arg), "0x")
	expectingString := ty.Kind() == reflect.String
	expectingByteSlice := ty.Kind() == reflect.Slice && ty.Elem().Kind() == reflect.Uint8

	if isHexString {
		if !expectingString && !expectingByteSlice {
			err := errors.Errorf("Got a hex string arg, but expected '%s'",
				ty.Kind().String())
			return reflect.ValueOf(nil), err, false
		}

		var value []byte
		value, err := hex.DecodeString(arg[2:])
		if err != nil {
			return reflect.ValueOf(nil), err, false
		}
		if ty.Kind() == reflect.String {
			return reflect.ValueOf(string(value)), nil, true
		}
		return reflect.ValueOf([]byte(value)), nil, true
	}

	if isQuotedString && expectingByteSlice {
		v := reflect.New(reflect.TypeOf(""))
		err := json.Unmarshal([]byte(arg), v.Interface())
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
	writeChanCapacity     = 1000
	wsWriteTimeoutSeconds = 30 // each write times out after this
	wsReadTimeoutSeconds  = 30 // connection times out if we haven't received *anything* in this long, not even pings.
	wsPingTickerSeconds   = 10 // send a ping every PingTickerSeconds.
)

// a single websocket connection
// contains listener id, underlying ws connection,
// and the event switch for subscribing to events
type wsConnection struct {
	cmn.BaseService

	remoteAddr  string
	baseConn    *websocket.Conn
	writeChan   chan types.RPCResponse
	readTimeout *time.Timer
	pingTicker  *time.Ticker

	funcMap map[string]*RPCFunc
	evsw    events.EventSwitch
}

// new websocket connection wrapper
func NewWSConnection(baseConn *websocket.Conn, funcMap map[string]*RPCFunc, evsw events.EventSwitch) *wsConnection {
	wsc := &wsConnection{
		remoteAddr: baseConn.RemoteAddr().String(),
		baseConn:   baseConn,
		writeChan:  make(chan types.RPCResponse, writeChanCapacity), // error when full.
		funcMap:    funcMap,
		evsw:       evsw,
	}
	wsc.BaseService = *cmn.NewBaseService(nil, "wsConnection", wsc)
	return wsc
}

// wsc.Start() blocks until the connection closes.
func (wsc *wsConnection) OnStart() error {
	wsc.BaseService.OnStart()

	// these must be set before the readRoutine is created, as it may
	// call wsc.Stop(), which accesses these timers
	wsc.readTimeout = time.NewTimer(time.Second * wsReadTimeoutSeconds)
	wsc.pingTicker = time.NewTicker(time.Second * wsPingTickerSeconds)

	// Read subscriptions/unsubscriptions to events
	go wsc.readRoutine()

	// Custom Ping handler to touch readTimeout
	wsc.baseConn.SetPingHandler(func(m string) error {
		// NOTE: https://github.com/gorilla/websocket/issues/97
		go wsc.baseConn.WriteControl(websocket.PongMessage, []byte(m), time.Now().Add(time.Second*wsWriteTimeoutSeconds))
		wsc.readTimeout.Reset(time.Second * wsReadTimeoutSeconds)
		return nil
	})
	wsc.baseConn.SetPongHandler(func(m string) error {
		// NOTE: https://github.com/gorilla/websocket/issues/97
		wsc.readTimeout.Reset(time.Second * wsReadTimeoutSeconds)
		return nil
	})
	go wsc.readTimeoutRoutine()

	// Write responses, BLOCKING.
	wsc.writeRoutine()
	return nil
}

func (wsc *wsConnection) OnStop() {
	wsc.BaseService.OnStop()
	if wsc.evsw != nil {
		wsc.evsw.RemoveListener(wsc.remoteAddr)
	}
	wsc.readTimeout.Stop()
	wsc.pingTicker.Stop()
	// The write loop closes the websocket connection
	// when it exits its loop, and the read loop
	// closes the writeChan
}

func (wsc *wsConnection) readTimeoutRoutine() {
	select {
	case <-wsc.readTimeout.C:
		wsc.Logger.Info("Stopping connection due to read timeout")
		wsc.Stop()
	case <-wsc.Quit:
		return
	}
}

// Implements WSRPCConnection
func (wsc *wsConnection) GetRemoteAddr() string {
	return wsc.remoteAddr
}

// Implements WSRPCConnection
func (wsc *wsConnection) GetEventSwitch() events.EventSwitch {
	return wsc.evsw
}

// Implements WSRPCConnection
// Blocking write to writeChan until service stops.
// Goroutine-safe
func (wsc *wsConnection) WriteRPCResponse(resp types.RPCResponse) {
	select {
	case <-wsc.Quit:
		return
	case wsc.writeChan <- resp:
	}
}

// Implements WSRPCConnection
// Nonblocking write.
// Goroutine-safe
func (wsc *wsConnection) TryWriteRPCResponse(resp types.RPCResponse) bool {
	select {
	case <-wsc.Quit:
		return false
	case wsc.writeChan <- resp:
		return true
	default:
		return false
	}
}

// Read from the socket and subscribe to or unsubscribe from events
func (wsc *wsConnection) readRoutine() {
	// Do not close writeChan, to allow WriteRPCResponse() to fail.
	// defer close(wsc.writeChan)

	for {
		select {
		case <-wsc.Quit:
			return
		default:
			var in []byte
			// Do not set a deadline here like below:
			// wsc.baseConn.SetReadDeadline(time.Now().Add(time.Second * wsReadTimeoutSeconds))
			// The client may not send anything for a while.
			// We use `readTimeout` to handle read timeouts.
			_, in, err := wsc.baseConn.ReadMessage()
			if err != nil {
				wsc.Logger.Info("Failed to read from connection", "remote", wsc.remoteAddr, "err", err.Error())
				// an error reading the connection,
				// kill the connection
				wsc.Stop()
				return
			}
			var request types.RPCRequest
			err = json.Unmarshal(in, &request)
			if err != nil {
				errStr := fmt.Sprintf("Error unmarshaling data: %s", err.Error())
				wsc.WriteRPCResponse(types.NewRPCResponse(request.ID, nil, errStr))
				continue
			}

			// Now, fetch the RPCFunc and execute it.

			rpcFunc := wsc.funcMap[request.Method]
			if rpcFunc == nil {
				wsc.WriteRPCResponse(types.NewRPCResponse(request.ID, nil, "RPC method unknown: "+request.Method))
				continue
			}
			var args []reflect.Value
			if rpcFunc.ws {
				wsCtx := types.WSRPCContext{Request: request, WSRPCConnection: wsc}
				args, err = jsonParamsToArgsWS(rpcFunc, request.Params, wsCtx)
			} else {
				args, err = jsonParamsToArgsRPC(rpcFunc, request.Params)
			}
			if err != nil {
				wsc.WriteRPCResponse(types.NewRPCResponse(request.ID, nil, err.Error()))
				continue
			}
			returns := rpcFunc.f.Call(args)
			wsc.Logger.Info("WSJSONRPC", "method", request.Method, "args", args, "returns", returns)
			result, err := unreflectResult(returns)
			if err != nil {
				wsc.WriteRPCResponse(types.NewRPCResponse(request.ID, nil, err.Error()))
				continue
			} else {
				wsc.WriteRPCResponse(types.NewRPCResponse(request.ID, result, ""))
				continue
			}

		}
	}
}

// receives on a write channel and writes out on the socket
func (wsc *wsConnection) writeRoutine() {
	defer wsc.baseConn.Close()
	for {
		select {
		case <-wsc.Quit:
			return
		case <-wsc.pingTicker.C:
			err := wsc.baseConn.WriteMessage(websocket.PingMessage, []byte{})
			if err != nil {
				wsc.Logger.Error("Failed to write ping message on websocket", "error", err)
				wsc.Stop()
				return
			}
		case msg := <-wsc.writeChan:
			jsonBytes, err := json.MarshalIndent(msg, "", "  ")
			if err != nil {
				wsc.Logger.Error("Failed to marshal RPCResponse to JSON", "error", err)
			} else {
				wsc.baseConn.SetWriteDeadline(time.Now().Add(time.Second * wsWriteTimeoutSeconds))
				if err = wsc.baseConn.WriteMessage(websocket.TextMessage, jsonBytes); err != nil {
					wsc.Logger.Error("Failed to write response on websocket", "error", err)
					wsc.Stop()
					return
				}
			}
		}
	}
}

//----------------------------------------

// Main manager for all websocket connections
// Holds the event switch
// NOTE: The websocket path is defined externally, e.g. in node/node.go
type WebsocketManager struct {
	websocket.Upgrader
	funcMap map[string]*RPCFunc
	evsw    events.EventSwitch
	logger  log.Logger
}

func NewWebsocketManager(funcMap map[string]*RPCFunc, evsw events.EventSwitch) *WebsocketManager {
	return &WebsocketManager{
		funcMap: funcMap,
		evsw:    evsw,
		Upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// TODO
				return true
			},
		},
		logger: log.NewNopLogger(),
	}
}

func (wm *WebsocketManager) SetLogger(l log.Logger) {
	wm.logger = l
}

// Upgrade the request/response (via http.Hijack) and starts the wsConnection.
func (wm *WebsocketManager) WebsocketHandler(w http.ResponseWriter, r *http.Request) {
	wsConn, err := wm.Upgrade(w, r, nil)
	if err != nil {
		// TODO - return http error
		wm.logger.Error("Failed to upgrade to websocket connection", "error", err)
		return
	}

	// register connection
	con := NewWSConnection(wsConn, wm.funcMap, wm.evsw)
	wm.logger.Info("New websocket connection", "remote", con.remoteAddr)
	con.Start() // Blocking
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
		link := fmt.Sprintf("http://%s/%s", r.Host, name)
		buf.WriteString(fmt.Sprintf("<a href=\"%s\">%s</a></br>", link, link))
	}

	buf.WriteString("<br>Endpoints that require arguments:<br>")
	for _, name := range argNames {
		link := fmt.Sprintf("http://%s/%s?", r.Host, name)
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
	w.Write(buf.Bytes())
}
