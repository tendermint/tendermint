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

	"github.com/gorilla/websocket"
	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/events"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	. "github.com/tendermint/tendermint/rpc/types"
	"github.com/tendermint/tendermint/types"
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
			WriteRPCResponse(w, NewRPCResponse(request.ID, nil, fmt.Sprintf("Invalid JSONRPC endpoint %s", r.URL.Path)))
			return
		}
		rpcFunc := funcMap[request.Method]
		if rpcFunc == nil {
			WriteRPCResponse(w, NewRPCResponse(request.ID, nil, "RPC method unknown: "+request.Method))
			return
		}
		args, err := jsonParamsToArgs(rpcFunc, request.Params)
		if err != nil {
			WriteRPCResponse(w, NewRPCResponse(request.ID, nil, err.Error()))
			return
		}
		returns := rpcFunc.f.Call(args)
		log.Info("HTTPJSONRPC", "method", request.Method, "args", args, "returns", returns)
		result, err := unreflectResult(returns)
		if err != nil {
			WriteRPCResponse(w, NewRPCResponse(request.ID, nil, err.Error()))
			return
		}
		WriteRPCResponse(w, NewRPCResponse(request.ID, result, ""))
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
	wire.ReadJSONObjectPtr(v.Interface(), object, &err)
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
	wire.ReadJSONPtr(v.Interface(), []byte(arg), &err)
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
	writeChanCapacity     = 1000
	wsWriteTimeoutSeconds = 30 // each write times out after this
	wsReadTimeoutSeconds  = 30 // connection times out if we haven't received *anything* in this long, not even pings.
	wsPingTickerSeconds   = 10 // send a ping every PingTickerSeconds.
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
	pingTicker  *time.Ticker

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
func (wsc *WSConnection) OnStart() error {
	wsc.QuitService.OnStart()

	// Read subscriptions/unsubscriptions to events
	go wsc.readRoutine()

	// Custom Ping handler to touch readTimeout
	wsc.readTimeout = time.NewTimer(time.Second * wsReadTimeoutSeconds)
	wsc.pingTicker = time.NewTicker(time.Second * wsPingTickerSeconds)
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

func (wsc *WSConnection) OnStop() {
	wsc.QuitService.OnStop()
	wsc.evsw.RemoveListener(wsc.id)
	wsc.readTimeout.Stop()
	wsc.pingTicker.Stop()
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

// Blocking write to writeChan until service stops.
func (wsc *WSConnection) writeRPCResponse(resp RPCResponse) {
	select {
	case <-wsc.Quit:
		return
	case wsc.writeChan <- resp:
	}
}

// Nonblocking write.
func (wsc *WSConnection) tryWriteRPCResponse(resp RPCResponse) bool {
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
			// wsc.baseConn.SetReadDeadline(time.Now().Add(time.Second * wsReadTimeoutSeconds))
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
				wsc.writeRPCResponse(NewRPCResponse(request.ID, nil, errStr))
				continue
			}
			switch request.Method {
			case "subscribe":
				if len(request.Params) != 1 {
					wsc.writeRPCResponse(NewRPCResponse(request.ID, nil, "subscribe takes 1 event parameter string"))
					continue
				}
				if event, ok := request.Params[0].(string); !ok {
					wsc.writeRPCResponse(NewRPCResponse(request.ID, nil, "subscribe takes 1 event parameter string"))
					continue
				} else {
					log.Notice("Subscribe to event", "id", wsc.id, "event", event)
					wsc.evsw.AddListenerForEvent(wsc.id, event, func(msg types.EventData) {
						// NOTE: EventSwitch callbacks must be nonblocking
						// NOTE: RPCResponses of subscribed events have id suffix "#event"
						wsc.tryWriteRPCResponse(NewRPCResponse(request.ID+"#event", ctypes.ResultEvent{event, msg}, ""))
					})
					continue
				}
			case "unsubscribe":
				if len(request.Params) == 0 {
					log.Notice("Unsubscribe from all events", "id", wsc.id)
					wsc.evsw.RemoveListener(wsc.id)
					wsc.writeRPCResponse(NewRPCResponse(request.ID, nil, ""))
					continue
				} else if len(request.Params) == 1 {
					if event, ok := request.Params[0].(string); !ok {
						wsc.writeRPCResponse(NewRPCResponse(request.ID, nil, "unsubscribe takes 0 or 1 event parameter strings"))
						continue
					} else {
						log.Notice("Unsubscribe from event", "id", wsc.id, "event", event)
						wsc.evsw.RemoveListenerForEvent(event, wsc.id)
						wsc.writeRPCResponse(NewRPCResponse(request.ID, nil, ""))
						continue
					}
				} else {
					wsc.writeRPCResponse(NewRPCResponse(request.ID, nil, "unsubscribe takes 0 or 1 event parameter strings"))
					continue
				}
			default:
				rpcFunc := wsc.funcMap[request.Method]
				if rpcFunc == nil {
					wsc.writeRPCResponse(NewRPCResponse(request.ID, nil, "RPC method unknown: "+request.Method))
					continue
				}
				args, err := jsonParamsToArgs(rpcFunc, request.Params)
				if err != nil {
					wsc.writeRPCResponse(NewRPCResponse(request.ID, nil, err.Error()))
					continue
				}
				returns := rpcFunc.f.Call(args)
				log.Info("WSJSONRPC", "method", request.Method, "args", args, "returns", returns)
				result, err := unreflectResult(returns)
				if err != nil {
					wsc.writeRPCResponse(NewRPCResponse(request.ID, nil, err.Error()))
					continue
				} else {
					wsc.writeRPCResponse(NewRPCResponse(request.ID, result, ""))
					continue
				}
			}
		}
	}
}

// receives on a write channel and writes out on the socket
func (wsc *WSConnection) writeRoutine() {
	defer wsc.baseConn.Close()
	var n, err = int(0), error(nil)
	for {
		select {
		case <-wsc.Quit:
			return
		case <-wsc.pingTicker.C:
			err := wsc.baseConn.WriteMessage(websocket.PingMessage, []byte{})
			if err != nil {
				log.Error("Failed to write ping message on websocket", "error", err)
				wsc.Stop()
				return
			}
		case msg := <-wsc.writeChan:
			buf := new(bytes.Buffer)
			wire.WriteJSON(msg, buf, &n, &err)
			if err != nil {
				log.Error("Failed to marshal RPCResponse to JSON", "error", err)
			} else {
				wsc.baseConn.SetWriteDeadline(time.Now().Add(time.Second * wsWriteTimeoutSeconds))
				bufBytes := buf.Bytes()
				if err = wsc.baseConn.WriteMessage(websocket.TextMessage, bufBytes); err != nil {
					log.Warn("Failed to write response on websocket", "error", err)
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
