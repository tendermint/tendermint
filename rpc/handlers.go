package rpc

import (
	"encoding/json"
	"fmt"
	"github.com/tendermint/tendermint/binary"
	"io/ioutil"
	"net/http"
	"reflect"
)

func RegisterRPCFuncs(mux *http.ServeMux, funcMap map[string]*RPCFunc) {
	// HTTP endpoints
	for funcName, rpcFunc := range funcMap {
		mux.HandleFunc("/"+funcName, makeHTTPHandler(rpcFunc))
	}

	// JSONRPC endpoints
	mux.HandleFunc("/", makeJSONRPCHandler(funcMap))
}

// holds all type information for each function
type RPCFunc struct {
	f        reflect.Value  // underlying rpc function
	args     []reflect.Type // type of each function arg
	returns  []reflect.Type // type of each return arg
	argNames []string       // name of each argument
}

func NewRPCFunc(f interface{}, args []string) *RPCFunc {
	return &RPCFunc{
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
func makeJSONRPCHandler(funcMap map[string]*RPCFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, _ := ioutil.ReadAll(r.Body)
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
func makeHTTPHandler(rpcFunc *RPCFunc) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		args, err := httpParamsToArgs(rpcFunc, r)
		if err != nil {
			WriteRPCResponse(w, NewRPCResponse(nil, err.Error()))
			return
		}
		returns := rpcFunc.f.Call(args)
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

// returns is Response struct and error. If error is not nil, return it
func unreflectResponse(returns []reflect.Value) (interface{}, error) {
	errV := returns[1]
	if errV.Interface() != nil {
		return nil, fmt.Errorf("%v", errV.Interface())
	}
	return returns[0].Interface(), nil
}
