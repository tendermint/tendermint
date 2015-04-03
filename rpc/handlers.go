package rpc

/*
TODO: support Call && GetStorage.
*/

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/tendermint/tendermint/binary"
	"github.com/tendermint/tendermint/events"
	"github.com/tendermint/tendermint/rpc/core"
	"golang.org/x/net/websocket"
	"io/ioutil"
	"net/http"
	"reflect"
	"runtime"
	"strings"
	"time"
)

// cache all type information about each function up front
// (func, responseStruct, argNames)
var funcMap = map[string]*FuncWrapper{
	"status":                  funcWrap(core.Status, []string{}),
	"net_info":                funcWrap(core.NetInfo, []string{}),
	"blockchain":              funcWrap(core.BlockchainInfo, []string{"min_height", "max_height"}),
	"get_block":               funcWrap(core.GetBlock, []string{"height"}),
	"get_account":             funcWrap(core.GetAccount, []string{"address"}),
	"get_storage":             funcWrap(core.GetStorage, []string{"address", "storage"}),
	"call":                    funcWrap(core.Call, []string{"address", "data"}),
	"list_validators":         funcWrap(core.ListValidators, []string{}),
	"dump_storage":            funcWrap(core.DumpStorage, []string{"address"}),
	"broadcast_tx":            funcWrap(core.BroadcastTx, []string{"tx"}),
	"list_accounts":           funcWrap(core.ListAccounts, []string{}),
	"unsafe/gen_priv_account": funcWrap(core.GenPrivAccount, []string{}),
	"unsafe/sign_tx":          funcWrap(core.SignTx, []string{"tx", "privAccounts"}),
}

// maps camel-case function names to lower case rpc version
// populated by calls to funcWrap
var reverseFuncMap = fillReverseFuncMap()

// fill the map from camelcase to lowercase
func fillReverseFuncMap() map[string]string {
	fMap := make(map[string]string)
	for name, f := range funcMap {
		camelName := runtime.FuncForPC(f.f.Pointer()).Name()
		spl := strings.Split(camelName, ".")
		if len(spl) > 1 {
			camelName = spl[len(spl)-1]
		}
		fMap[camelName] = name
	}
	return fMap
}

func initHandlers(ew *events.EventSwitch) {
	// HTTP endpoints
	for funcName, funcInfo := range funcMap {
		http.HandleFunc("/"+funcName, toHTTPHandler(funcInfo))
	}

	// JSONRPC endpoints
	http.HandleFunc("/", JSONRPCHandler)

	w := NewWebsocketManager(ew)
	// websocket endpoint
	http.Handle("/events", websocket.Handler(w.eventsHandler))
}

//-------------------------------------

// holds all type information for each function
type FuncWrapper struct {
	f        reflect.Value  // function from "rpc/core"
	args     []reflect.Type // type of each function arg
	returns  []reflect.Type // type of each return arg
	argNames []string       // name of each argument
}

func funcWrap(f interface{}, args []string) *FuncWrapper {
	return &FuncWrapper{
		f:        reflect.ValueOf(f),
		args:     funcArgTypes(f),
		returns:  funcReturnTypes(f),
		argNames: args,
	}
}

func funcArgTypes(f interface{}) []reflect.Type {
	t := reflect.TypeOf(f)
	n := t.NumIn()
	types := make([]reflect.Type, n)
	for i := 0; i < n; i++ {
		types[i] = t.In(i)
	}
	return types
}

func funcReturnTypes(f interface{}) []reflect.Type {
	t := reflect.TypeOf(f)
	n := t.NumOut()
	types := make([]reflect.Type, n)
	for i := 0; i < n; i++ {
		types[i] = t.Out(i)
	}
	return types
}

//-----------------------------------------------------------------------------
// rpc.json

// jsonrpc calls grab the given method's function info and runs reflect.Call
func JSONRPCHandler(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.Path) > 1 {
		WriteRPCResponse(w, NewRPCResponse(nil, fmt.Sprintf("Invalid JSONRPC endpoint %s", r.URL.Path)))
		return
	}
	b, _ := ioutil.ReadAll(r.Body)
	var request RPCRequest
	err := json.Unmarshal(b, &request)
	if err != nil {
		WriteRPCResponse(w, NewRPCResponse(nil, err.Error()))
		return
	}

	funcInfo := funcMap[request.Method]
	if funcInfo == nil {
		WriteRPCResponse(w, NewRPCResponse(nil, "RPC method unknown: "+request.Method))
		return
	}
	args, err := jsonParamsToArgs(funcInfo, request.Params)
	if err != nil {
		WriteRPCResponse(w, NewRPCResponse(nil, err.Error()))
		return
	}
	returns := funcInfo.f.Call(args)
	response, err := unreflectResponse(returns)
	if err != nil {
		WriteRPCResponse(w, NewRPCResponse(nil, err.Error()))
		return
	}
	WriteRPCResponse(w, NewRPCResponse(response, ""))
}

// covert a list of interfaces to properly typed values
func jsonParamsToArgs(funcInfo *FuncWrapper, params []interface{}) ([]reflect.Value, error) {
	values := make([]reflect.Value, len(params))
	for i, p := range params {
		ty := funcInfo.args[i]
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
	binary.ReadJSONFromObject(v.Interface(), object, &err)
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
func toHTTPHandler(funcInfo *FuncWrapper) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		args, err := httpParamsToArgs(funcInfo, r)
		if err != nil {
			WriteRPCResponse(w, NewRPCResponse(nil, err.Error()))
			return
		}
		returns := funcInfo.f.Call(args)
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
func httpParamsToArgs(funcInfo *FuncWrapper, r *http.Request) ([]reflect.Value, error) {
	argTypes := funcInfo.args
	argNames := funcInfo.argNames

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

// for requests coming in
type WsRequest struct {
	Type  string // subscribe or unsubscribe
	Event string
}

// for responses going out
type WsResponse struct {
	Event string
	Data  interface{}
	Error string
}

// a single websocket connection
// contains the listeners id
type Connection struct {
	id          string
	wsCon       *websocket.Conn
	writeChan   chan WsResponse
	quitChan    chan struct{}
	failedSends uint
}

// new websocket connection wrapper
func NewConnection(con *websocket.Conn) *Connection {
	return &Connection{
		id:        con.RemoteAddr().String(),
		wsCon:     con,
		writeChan: make(chan WsResponse, WriteChanBuffer), // buffered. we keep track when its full
	}
}

// close the connection
func (c *Connection) Close() {
	c.wsCon.Close()
	close(c.writeChan)
	close(c.quitChan)
}

// main manager for all websocket connections
// holds the event switch
type WebsocketManager struct {
	ew   *events.EventSwitch
	cons map[string]*Connection
}

func NewWebsocketManager(ew *events.EventSwitch) *WebsocketManager {
	return &WebsocketManager{
		ew:   ew,
		cons: make(map[string]*Connection),
	}
}

func (w *WebsocketManager) eventsHandler(con *websocket.Conn) {
	// register connection
	c := NewConnection(con)
	w.cons[con.RemoteAddr().String()] = c

	// read subscriptions/unsubscriptions to events
	go w.read(c)
	// write responses
	go w.write(c)
}

const (
	WsConnectionReaperSeconds = 5
	MaxFailedSendsSeconds     = 10
	WriteChanBuffer           = 10
)

// read from the socket and subscribe to or unsubscribe from events
func (w *WebsocketManager) read(con *Connection) {
	reaper := time.Tick(time.Second * WsConnectionReaperSeconds)
	for {
		select {
		case <-reaper:
			if con.failedSends > MaxFailedSendsSeconds {
				// sending has failed too many times.
				// kill the connection
				con.quitChan <- struct{}{}
			}
		default:
			var in []byte
			if err := websocket.Message.Receive(con.wsCon, &in); err != nil {
				// an error reading the connection,
				// so kill the connection
				con.quitChan <- struct{}{}
			}
			var req WsRequest
			err := json.Unmarshal(in, &req)
			if err != nil {
				errStr := fmt.Sprintf("Error unmarshaling data: %s", err.Error())
				con.writeChan <- WsResponse{Error: errStr}
			}
			switch req.Type {
			case "subscribe":
				w.ew.AddListenerForEvent(con.id, req.Event, func(msg interface{}) {
					resp := WsResponse{
						Event: req.Event,
						Data:  msg,
					}
					select {
					case con.writeChan <- resp:
						// yay
						con.failedSends = 0
					default:
						// channel is full
						// if this happens too many times,
						// close connection
						con.failedSends += 1
					}
				})
			case "unsubscribe":
				if req.Event != "" {
					w.ew.RemoveListenerForEvent(req.Event, con.id)
				} else {
					w.ew.RemoveListener(con.id)
				}
			default:
				con.writeChan <- WsResponse{Error: "Unknown request type: " + req.Type}
			}

		}
	}
}

// receives on a write channel and writes out to the socket
func (w *WebsocketManager) write(con *Connection) {
	n, err := new(int64), new(error)
	for {
		select {
		case msg := <-con.writeChan:
			buf := new(bytes.Buffer)
			binary.WriteJSON(msg, buf, n, err)
			if *err != nil {
				log.Error("Failed to write JSON WsResponse", "error", err)
			} else {
				websocket.Message.Send(con.wsCon, buf.Bytes())
			}
		case <-con.quitChan:
			w.closeConn(con)
			return
		}
	}
}

// close a connection and delete from manager
func (w *WebsocketManager) closeConn(con *Connection) {
	con.Close()
	delete(w.cons, con.id)
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
