package rpcserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"sort"
	"time"

	"github.com/tendermint/tendermint/Godeps/_workspace/src/github.com/gorilla/websocket"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/events"
	. "github.com/tendermint/tendermint/rpc/types"
)

func RegisterRPCFuncs(mux *http.ServeMux, funcMap map[string]*RPCFunc) {
	// HTTP endpoints
	for funcName, rpcFunc := range funcMap {
		mux.HandleFunc("/"+funcName, makeHTTPHandler(rpcFunc))
	}

	// JSONRPC endpoints
	mux.HandleFunc("/", makeJSONRPCHandler(funcMap))
}

//-------------------------------------
// function introspection

// holds all type information for each function
type RPCFunc struct {
	f        reflect.Value  // underlying rpc function
	args     []reflect.Type // type of each function arg
	returns  []reflect.Type // type of each return arg
	argNames []string       // name of each argument
}

// wraps a function for quicker introspection
func NewRPCFunc(f interface{}, args []string) *RPCFunc {
	return &RPCFunc{
		f:        reflect.ValueOf(f),
		args:     funcArgTypes(f),
		returns:  funcReturnTypes(f),
		argNames: args,
	}
}

// return a function's argument types
func funcArgTypes(f interface{}) []reflect.Type {
	t := reflect.TypeOf(f)
	n := t.NumIn()
	types := make([]reflect.Type, n)
	for i := 0; i < n; i++ {
		types[i] = t.In(i)
	}
	return types
}

// return a function's return types
func funcReturnTypes(f interface{}) []reflect.Type {
	t := reflect.TypeOf(f)
	n := t.NumOut()
	types := make([]reflect.Type, n)
	for i := 0; i < n; i++ {
		types[i] = t.Out(i)
	}
	return types
}

// function introspection
//-----------------------------------------------------------------------------
// rpc.json

// jsonrpc calls grab the given method's function info and runs reflect.Call
func makeJSONRPCHandler(funcMap map[string]*RPCFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, _ := ioutil.ReadAll(r.Body)
		// if its an empty request (like from a browser),
		// just display a list of functions
		if len(b) == 0 {
			writeListOfEndpoints(w, r, funcMap)
			return
		}

		var request RPCRequest
		err := json.Unmarshal(b, &request)
		if err != nil {
			WriteRPCResponse(w, NewRPCResponse("", nil, err.Error()))
			return
		}
		if len(r.URL.Path) > 1 {
			WriteRPCResponse(w, NewRPCResponse(request.Id, nil, fmt.Sprintf("Invalid JSONRPC endpoint %s", r.URL.Path)))
			return
		}
		rpcFunc := funcMap[request.Method]
		if rpcFunc == nil {
			WriteRPCResponse(w, NewRPCResponse(request.Id, nil, "RPC method unknown: "+request.Method))
			return
		}
		args, err := jsonParamsToArgs(rpcFunc, request.Params)
		if err != nil {
			WriteRPCResponse(w, NewRPCResponse(request.Id, nil, err.Error()))
			return
		}
		returns := rpcFunc.f.Call(args)
		log.Info("HTTPJSONRPC", "method", request.Method, "args", args, "returns", returns)
		result, err := unreflectResult(returns)
		if err != nil {
			WriteRPCResponse(w, NewRPCResponse(request.Id, nil, err.Error()))
			return
		}
		WriteRPCResponse(w, NewRPCResponse(request.Id, result, ""))
	}
}

// covert a list of interfaces to properly typed values
func jsonParamsToArgs(rpcFunc *RPCFunc, params []interface{}) ([]reflect.Value, error) {
	if len(rpcFunc.argNames) != len(params) {
		return nil, errors.New(fmt.Sprintf("Expected %v parameters (%v), got %v (%v)",
			len(rpcFunc.argNames), rpcFunc.argNames, len(params), params))
	}
	values := make([]reflect.Value, len(params))
	for i, p := range params {
		ty := rpcFunc.args[i]
		v, err := _jsonObjectToArg(ty, p)
		if err != nil {
			return nil, err
		}
		values[i] = v
	}
	return values, nil
}

func _jsonObjectToArg(ty reflect.Type, object interface{}) (reflect.Value, error) {
	var err error
	v := reflect.New(ty)
	binary.ReadJSONObjectPtr(v.Interface(), object, &err)
	if err != nil {
		return v, err
	}
	v = v.Elem()
	return v, nil
}

// rpc.json
//-----------------------------------------------------------------------------
// rpc.http

// convert from a function name to the http handler
func makeHTTPHandler(rpcFunc *RPCFunc) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		args, err := httpParamsToArgs(rpcFunc, r)
		if err != nil {
			WriteRPCResponse(w, NewRPCResponse("", nil, err.Error()))
			return
		}
		returns := rpcFunc.f.Call(args)
		log.Info("HTTPRestRPC", "method", r.URL.Path, "args", args, "returns", returns)
		result, err := unreflectResult(returns)
		if err != nil {
			WriteRPCResponse(w, NewRPCResponse("", nil, err.Error()))
			return
		}
		WriteRPCResponse(w, NewRPCResponse("", result, ""))
	}
}

// Covert an http query to a list of properly typed values.
// To be properly decoded the arg must be a concrete type from tendermint (if its an interface).
func httpParamsToArgs(rpcFunc *RPCFunc, r *http.Request) ([]reflect.Value, error) {
	argTypes := rpcFunc.args
	argNames := rpcFunc.argNames

	var err error
	values := make([]reflect.Value, len(argNames))
	for i, name := range argNames {
		ty := argTypes[i]
		arg := GetParam(r, name)
		values[i], err = _jsonStringToArg(ty, arg)
		if err != nil {
			return nil, err
		}
	}
	return values, nil
}

func _jsonStringToArg(ty reflect.Type, arg string) (reflect.Value, error) {
	var err error
	v := reflect.New(ty)
	binary.ReadJSONPtr(v.Interface(), []byte(arg), &err)
	if err != nil {
		return v, err
	}
	v = v.Elem()
	return v, nil
}

// rpc.http
//-----------------------------------------------------------------------------
// rpc.websocket

const (
	writeChanCapacity     = 20
	WSWriteTimeoutSeconds = 10 // exposed for tests
	WSReadTimeoutSeconds  = 10 // exposed for tests
)

// a single websocket connection
// contains listener id, underlying ws connection,
// and the event switch for subscribing to events
type WSConnection struct {
	QuitService

	id          string
	baseConn    *websocket.Conn
	writeChan   chan RPCResponse
	readTimeout *time.Timer

	funcMap map[string]*RPCFunc
	evsw    *events.EventSwitch
}

// new websocket connection wrapper
func NewWSConnection(baseConn *websocket.Conn, funcMap map[string]*RPCFunc, evsw *events.EventSwitch) *WSConnection {
	wsc := &WSConnection{
		id:        baseConn.RemoteAddr().String(),
		baseConn:  baseConn,
		writeChan: make(chan RPCResponse, writeChanCapacity), // error when full.
		funcMap:   funcMap,
		evsw:      evsw,
	}
	wsc.QuitService = *NewQuitService(log, "WSConnection", wsc)
	return wsc
}

// wsc.Start() blocks until the connection closes.
func (wsc *WSConnection) OnStart() {
	wsc.QuitService.OnStart()

	// Read subscriptions/unsubscriptions to events
	go wsc.readRoutine()

	// Custom Ping handler to touch readTimeout
	wsc.readTimeout = time.NewTimer(time.Second * WSReadTimeoutSeconds)
	wsc.baseConn.SetPingHandler(func(m string) error {
		wsc.baseConn.WriteControl(websocket.PongMessage, []byte(m), time.Now().Add(time.Second*WSWriteTimeoutSeconds))
		wsc.readTimeout.Reset(time.Second * WSReadTimeoutSeconds)
		return nil
	})
	wsc.baseConn.SetPongHandler(func(m string) error {
		wsc.readTimeout.Reset(time.Second * WSReadTimeoutSeconds)
		return nil
	})
	go wsc.readTimeoutRoutine()

	// Write responses, BLOCKING.
	wsc.writeRoutine()
}

func (wsc *WSConnection) OnStop() {
	wsc.QuitService.OnStop()
	wsc.evsw.RemoveListener(wsc.id)
	wsc.readTimeout.Stop()
	// The write loop closes the websocket connection
	// when it exits its loop, and the read loop
	// closes the writeChan
}

func (wsc *WSConnection) readTimeoutRoutine() {
	select {
	case <-wsc.readTimeout.C:
		log.Notice("Stopping connection due to read timeout")
		wsc.Stop()
	case <-wsc.Quit:
		return
	}
}

// Attempt to write response to writeChan and record failures
func (wsc *WSConnection) writeRPCResponse(resp RPCResponse) {
	select {
	case wsc.writeChan <- resp:
	default:
		log.Notice("Stopping connection due to writeChan overflow", "id", wsc.id)
		wsc.Stop() // writeChan capacity exceeded, error.
	}
}

// Read from the socket and subscribe to or unsubscribe from events
func (wsc *WSConnection) readRoutine() {
	// Do not close writeChan, to allow writeRPCResponse() to fail.
	// defer close(wsc.writeChan)

	for {
		select {
		case <-wsc.Quit:
			return
		default:
			var in []byte
			// Do not set a deadline here like below:
			// wsc.baseConn.SetReadDeadline(time.Now().Add(time.Second * WSReadTimeoutSeconds))
			// The client may not send anything for a while.
			// We use `readTimeout` to handle read timeouts.
			_, in, err := wsc.baseConn.ReadMessage()
			if err != nil {
				log.Notice("Failed to read from connection", "id", wsc.id)
				// an error reading the connection,
				// kill the connection
				wsc.Stop()
				return
			}
			var request RPCRequest
			err = json.Unmarshal(in, &request)
			if err != nil {
				errStr := fmt.Sprintf("Error unmarshaling data: %s", err.Error())
				wsc.writeRPCResponse(NewRPCResponse(request.Id, nil, errStr))
				continue
			}
			switch request.Method {
			case "subscribe":
				if len(request.Params) != 1 {
					wsc.writeRPCResponse(NewRPCResponse(request.Id, nil, "subscribe takes 1 event parameter string"))
					continue
				}
				if event, ok := request.Params[0].(string); !ok {
					wsc.writeRPCResponse(NewRPCResponse(request.Id, nil, "subscribe takes 1 event parameter string"))
					continue
				} else {
					log.Notice("Subscribe to event", "id", wsc.id, "event", event)
					wsc.evsw.AddListenerForEvent(wsc.id, event, func(msg interface{}) {
						wsc.writeRPCResponse(NewRPCResponse(request.Id, RPCEventResult{event, msg}, ""))
					})
					continue
				}
			case "unsubscribe":
				if len(request.Params) == 0 {
					log.Notice("Unsubscribe from all events", "id", wsc.id)
					wsc.evsw.RemoveListener(wsc.id)
					continue
				} else if len(request.Params) == 1 {
					if event, ok := request.Params[0].(string); !ok {
						wsc.writeRPCResponse(NewRPCResponse(request.Id, nil, "unsubscribe takes 0 or 1 event parameter strings"))
						continue
					} else {
						log.Notice("Unsubscribe from event", "id", wsc.id, "event", event)
						wsc.evsw.RemoveListenerForEvent(event, wsc.id)
						continue
					}
				} else {
					wsc.writeRPCResponse(NewRPCResponse(request.Id, nil, "unsubscribe takes 0 or 1 event parameter strings"))
					continue
				}
			default:
				rpcFunc := wsc.funcMap[request.Method]
				if rpcFunc == nil {
					wsc.writeRPCResponse(NewRPCResponse(request.Id, nil, "RPC method unknown: "+request.Method))
					continue
				}
				args, err := jsonParamsToArgs(rpcFunc, request.Params)
				if err != nil {
					wsc.writeRPCResponse(NewRPCResponse(request.Id, nil, err.Error()))
					continue
				}
				returns := rpcFunc.f.Call(args)
				log.Info("WSJSONRPC", "method", request.Method, "args", args, "returns", returns)
				result, err := unreflectResult(returns)
				if err != nil {
					wsc.writeRPCResponse(NewRPCResponse(request.Id, nil, err.Error()))
					continue
				} else {
					wsc.writeRPCResponse(NewRPCResponse(request.Id, result, ""))
					continue
				}
			}
		}
	}
}

// receives on a write channel and writes out on the socket
func (wsc *WSConnection) writeRoutine() {
	defer wsc.baseConn.Close()
	n, err := new(int64), new(error)
	for {
		select {
		case <-wsc.Quit:
			return
		case msg := <-wsc.writeChan:
			buf := new(bytes.Buffer)
			binary.WriteJSON(msg, buf, n, err)
			if *err != nil {
				log.Error("Failed to marshal RPCResponse to JSON", "error", err)
			} else {
				wsc.baseConn.SetWriteDeadline(time.Now().Add(time.Second * WSWriteTimeoutSeconds))
				if err := wsc.baseConn.WriteMessage(websocket.TextMessage, buf.Bytes()); err != nil {
					log.Warn("Failed to write response on websocket", "error", err)
					wsc.Stop()
					return
				}
			}
		}
	}
}

//----------------------------------------

// main manager for all websocket connections
// holds the event switch
type WebsocketManager struct {
	websocket.Upgrader
	funcMap map[string]*RPCFunc
	evsw    *events.EventSwitch
}

func NewWebsocketManager(funcMap map[string]*RPCFunc, evsw *events.EventSwitch) *WebsocketManager {
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
	}
}

// Upgrade the request/response (via http.Hijack) and starts the WSConnection.
func (wm *WebsocketManager) WebsocketHandler(w http.ResponseWriter, r *http.Request) {
	wsConn, err := wm.Upgrade(w, r, nil)
	if err != nil {
		// TODO - return http error
		log.Error("Failed to upgrade to websocket connection", "error", err)
		return
	}

	// register connection
	con := NewWSConnection(wsConn, wm.funcMap, wm.evsw)
	log.Notice("New websocket connection", "origin", con.id)
	con.Start() // Blocking
}

// rpc.websocket
//-----------------------------------------------------------------------------

// returns is result struct and error. If error is not nil, return it
func unreflectResult(returns []reflect.Value) (interface{}, error) {
	errV := returns[1]
	if errV.Interface() != nil {
		return nil, fmt.Errorf("%v", errV.Interface())
	}
	return returns[0].Interface(), nil
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
