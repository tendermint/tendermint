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
	"sync/atomic"
	"time"

	"github.com/tendermint/tendermint/Godeps/_workspace/src/github.com/gorilla/websocket"
	"github.com/tendermint/tendermint/binary"
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

func RegisterEventsHandler(mux *http.ServeMux, evsw *events.EventSwitch) {
	// websocket endpoint
	wm := NewWebsocketManager(evsw)
	mux.HandleFunc("/events", wm.websocketHandler) // 	websocket.Handler(w.eventsHandler))
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
		if len(r.URL.Path) > 1 {
			WriteRPCResponse(w, NewRPCResponse(nil, fmt.Sprintf("Invalid JSONRPC endpoint %s", r.URL.Path)))
			return
		}
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
			WriteRPCResponse(w, NewRPCResponse(nil, err.Error()))
			return
		}
		rpcFunc := funcMap[request.Method]
		if rpcFunc == nil {
			WriteRPCResponse(w, NewRPCResponse(nil, "RPC method unknown: "+request.Method))
			return
		}
		args, err := jsonParamsToArgs(rpcFunc, request.Params)
		if err != nil {
			WriteRPCResponse(w, NewRPCResponse(nil, err.Error()))
			return
		}
		returns := rpcFunc.f.Call(args)
		log.Debug("HTTPJSONRPC", "method", request.Method, "args", args, "returns", returns)
		response, err := unreflectResponse(returns)
		if err != nil {
			WriteRPCResponse(w, NewRPCResponse(nil, err.Error()))
			return
		}
		WriteRPCResponse(w, NewRPCResponse(response, ""))
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
	binary.ReadJSONObject(v.Interface(), object, &err)
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
			WriteRPCResponse(w, NewRPCResponse(nil, err.Error()))
			return
		}
		returns := rpcFunc.f.Call(args)
		log.Debug("HTTPRestRPC", "method", r.URL.Path, "args", args, "returns", returns)
		response, err := unreflectResponse(returns)
		if err != nil {
			WriteRPCResponse(w, NewRPCResponse(nil, err.Error()))
			return
		}
		WriteRPCResponse(w, NewRPCResponse(response, ""))
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
	binary.ReadJSON(v.Interface(), []byte(arg), &err)
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
	WSConnectionReaperSeconds = 5
	MaxFailedSends            = 10
	WriteChanBufferSize       = 10
)

// a single websocket connection
// contains listener id, underlying ws connection,
// and the event switch for subscribing to events
type WSConnection struct {
	id          string
	wsConn      *websocket.Conn
	writeChan   chan WSResponse
	quitChan    chan struct{}
	failedSends uint
	started     uint32
	stopped     uint32

	evsw *events.EventSwitch
}

// new websocket connection wrapper
func NewWSConnection(wsConn *websocket.Conn) *WSConnection {
	return &WSConnection{
		id:        wsConn.RemoteAddr().String(),
		wsConn:    wsConn,
		writeChan: make(chan WSResponse, WriteChanBufferSize), // buffered. we keep track when its full
		quitChan:  make(chan struct{}),
	}
}

// start the connection and hand her the event switch
func (con *WSConnection) Start(evsw *events.EventSwitch) {
	if atomic.CompareAndSwapUint32(&con.started, 0, 1) {
		con.evsw = evsw

		// read subscriptions/unsubscriptions to events
		go con.read()
		// write responses
		con.write()
	}
}

// close the connection
func (con *WSConnection) Stop() {
	if atomic.CompareAndSwapUint32(&con.stopped, 0, 1) {
		con.evsw.RemoveListener(con.id)
		close(con.quitChan)
		// the write loop closes the websocket connection
		// when it exits its loop, and the read loop
		// closes the writeChan
	}
}

// attempt to write response to writeChan and record failures
func (con *WSConnection) safeWrite(resp WSResponse) {
	select {
	case con.writeChan <- resp:
		// yay
		con.failedSends = 0
	default:
		// channel is full
		// if this happens too many times in a row,
		// close connection
		con.failedSends += 1
	}
}

// read from the socket and subscribe to or unsubscribe from events
func (con *WSConnection) read() {
	defer close(con.writeChan)
	reaper := time.Tick(time.Second * WSConnectionReaperSeconds)
	for {
		select {
		// TODO: this actually doesn't work
		// since ReadMessage blocks. Really it needs its own
		// go routine
		case <-reaper:
			if con.failedSends > MaxFailedSends {
				// sending has failed too many times.
				// kill the connection
				con.Stop()
				return
			}
		default:
			var in []byte
			_, in, err := con.wsConn.ReadMessage()
			if err != nil {
				// an error reading the connection,
				// kill the connection
				con.Stop()
				return
			}
			var req WSRequest
			err = json.Unmarshal(in, &req)
			if err != nil {
				errStr := fmt.Sprintf("Error unmarshaling data: %s", err.Error())
				con.safeWrite(WSResponse{Error: errStr})
				continue
			}
			switch req.Type {
			case "subscribe":
				log.Info("New event subscription", "con id", con.id, "event", req.Event)
				con.evsw.AddListenerForEvent(con.id, req.Event, func(msg interface{}) {
					resp := WSResponse{
						Event: req.Event,
						Data:  msg,
					}
					con.safeWrite(resp)
				})
			case "unsubscribe":
				if req.Event != "" {
					con.evsw.RemoveListenerForEvent(req.Event, con.id)
				} else {
					con.evsw.RemoveListener(con.id)
				}
			default:
				con.safeWrite(WSResponse{Error: "Unknown request type: " + req.Type})
			}
		}
	}
}

// receives on a write channel and writes out on the socket
func (con *WSConnection) write() {
	defer con.wsConn.Close()
	n, err := new(int64), new(error)
	for {
		select {
		case msg := <-con.writeChan:
			buf := new(bytes.Buffer)
			binary.WriteJSON(msg, buf, n, err)
			if *err != nil {
				log.Error("Failed to marshal WSResponse to JSON", "error", err)
			} else {
				if err := con.wsConn.WriteMessage(websocket.TextMessage, buf.Bytes()); err != nil {
					log.Error("Failed to write response on websocket", "error", err)
					con.Stop()
					return
				}
			}
		case <-con.quitChan:
			return
		}
	}
}

// main manager for all websocket connections
// holds the event switch
type WebsocketManager struct {
	websocket.Upgrader
	evsw *events.EventSwitch
}

func NewWebsocketManager(evsw *events.EventSwitch) *WebsocketManager {
	return &WebsocketManager{
		evsw: evsw,
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

func (wm *WebsocketManager) websocketHandler(w http.ResponseWriter, r *http.Request) {
	wsConn, err := wm.Upgrade(w, r, nil)
	if err != nil {
		// TODO - return http error
		log.Error("Failed to upgrade to websocket connection", "error", err)
		return
	}

	// register connection
	con := NewWSConnection(wsConn)
	log.Info("New websocket connection", "origin", con.id)
	con.Start(wm.evsw)
}

// rpc.websocket
//-----------------------------------------------------------------------------

// returns is Response struct and error. If error is not nil, return it
func unreflectResponse(returns []reflect.Value) (interface{}, error) {
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
