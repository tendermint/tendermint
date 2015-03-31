package rpc

/*
TODO: support Call && GetStorage.
*/

import (
	"encoding/json"
	"fmt"
	"github.com/tendermint/tendermint2/binary"
	"github.com/tendermint/tendermint2/rpc/core"
	"io/ioutil"
	"net/http"
	"reflect"
)

// cache all type information about each function up front
// (func, responseStruct, argNames)
var funcMap = map[string]*FuncWrapper{
	"status":                  funcWrap(core.Status, []string{}),
	"net_info":                funcWrap(core.NetInfo, []string{}),
	"blockchain":              funcWrap(core.BlockchainInfo, []string{"min_height", "max_height"}),
	"get_block":               funcWrap(core.GetBlock, []string{"height"}),
	"get_account":             funcWrap(core.GetAccount, []string{"address"}),
	"list_validators":         funcWrap(core.ListValidators, []string{}),
	"dump_storage":            funcWrap(core.DumpStorage, []string{"address"}),
	"broadcast_tx":            funcWrap(core.BroadcastTx, []string{"tx"}),
	"list_accounts":           funcWrap(core.ListAccounts, []string{}),
	"unsafe/gen_priv_account": funcWrap(core.GenPrivAccount, []string{}),
	"unsafe/sign_tx":          funcWrap(core.SignTx, []string{"tx", "privAccounts"}),
}

func initHandlers() {
	// HTTP endpoints
	for funcName, funcInfo := range funcMap {
		http.HandleFunc("/"+funcName, toHttpHandler(funcInfo))
	}

	// JSONRPC endpoints
	http.HandleFunc("/", JSONRPCHandler)
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

type JSONRPC struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	Id      int           `json:"id"`
}

// jsonrpc calls grab the given method's function info and runs reflect.Call
func JSONRPCHandler(w http.ResponseWriter, r *http.Request) {
	b, _ := ioutil.ReadAll(r.Body)
	var jrpc JSONRPC
	err := json.Unmarshal(b, &jrpc)
	if err != nil {
		// TODO
	}

	funcInfo := funcMap[jrpc.Method]
	args, err := jsonParamsToArgs(funcInfo, jrpc.Params)
	if err != nil {
		WriteAPIResponse(w, API_INVALID_PARAM, nil, err.Error())
		return
	}
	returns := funcInfo.f.Call(args)
	response, err := returnsToResponse(returns)
	if err != nil {
		WriteAPIResponse(w, API_ERROR, nil, err.Error())
		return
	}
	WriteAPIResponse(w, API_OK, response, "")
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
func toHttpHandler(funcInfo *FuncWrapper) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		args, err := httpParamsToArgs(funcInfo, r)
		if err != nil {
			WriteAPIResponse(w, API_INVALID_PARAM, nil, err.Error())
			return
		}
		returns := funcInfo.f.Call(args)
		response, err := returnsToResponse(returns)
		if err != nil {
			WriteAPIResponse(w, API_ERROR, nil, err.Error())
			return
		}
		WriteAPIResponse(w, API_OK, response, "")
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

// returns is Response struct and error. If error is not nil, return it
func returnsToResponse(returns []reflect.Value) (interface{}, error) {
	errV := returns[1]
	if errV.Interface() != nil {
		return nil, fmt.Errorf("%v", errV.Interface())
	}
	return returns[0].Interface(), nil
}
