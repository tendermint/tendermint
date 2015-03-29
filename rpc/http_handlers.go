package rpc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/tendermint/tendermint/binary"
	"github.com/tendermint/tendermint/rpc/core"
	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
)

// cache all type information about each function up front
// (func, responseStruct, argNames)
// XXX: response structs are allocated once and reused - will this cause an issue eg. if a field ever not overwritten?
var funcMap = map[string]*FuncWrapper{
	"status":                  funcWrap(core.Status, &ResponseStatus{}, []string{}),
	"net_info":                funcWrap(core.NetInfo, &ResponseNetInfo{}, []string{}),
	"blockchain":              funcWrap(core.BlockchainInfo, &ResponseBlockchainInfo{}, []string{"min_height", "max_height"}),
	"get_block":               funcWrap(core.GetBlock, &ResponseGetBlock{}, []string{"height"}),
	"get_account":             funcWrap(core.GetAccount, &ResponseGetAccount{}, []string{"address"}),
	"list_validators":         funcWrap(core.ListValidators, &ResponseListValidators{}, []string{}),
	"broadcast_tx":            funcWrap(core.BroadcastTx, &ResponseBroadcastTx{}, []string{"tx"}),
	"list_accounts":           funcWrap(core.ListAccounts, &ResponseListAccounts{}, []string{}),
	"unsafe/gen_priv_account": funcWrap(core.GenPrivAccount, &ResponseGenPrivAccount{}, []string{}),
	"unsafe/sign_tx":          funcWrap(core.SignTx, &ResponseSignTx{}, []string{"tx", "privAccounts"}),
}

// holds all type information for each function
type FuncWrapper struct {
	f        reflect.Value  // function from "rpc/core"
	args     []reflect.Type // type of each function arg
	returns  []reflect.Type // type of each return arg
	argNames []string       // name of each argument
	response reflect.Value  // response struct (to be filled with "returns")
}

func funcWrap(f interface{}, response interface{}, args []string) *FuncWrapper {
	return &FuncWrapper{
		f:        reflect.ValueOf(f),
		args:     funcArgTypes(f),
		returns:  funcReturnTypes(f),
		argNames: args,
		response: reflect.ValueOf(response),
	}
}

// convert from a function name to the http handler
func toHandler(funcName string) func(http.ResponseWriter, *http.Request) {
	funcInfo := funcMap[funcName]
	return func(w http.ResponseWriter, r *http.Request) {
		values, err := queryToValues(funcInfo, r)
		if err != nil {
			WriteAPIResponse(w, API_INVALID_PARAM, nil, err.Error())
			return
		}
		returns := funcInfo.f.Call(values)
		response, err := returnsToResponse(funcInfo, returns)
		if err != nil {
			WriteAPIResponse(w, API_ERROR, nil, err.Error())
			return
		}
		WriteAPIResponse(w, API_OK, response, "")
	}
}

// convert a (json) string to a given type
func jsonToArg(ty reflect.Type, arg string) (reflect.Value, error) {
	v := reflect.New(ty).Elem()
	kind := v.Kind()
	var err error
	switch kind {
	case reflect.Interface:
		v = reflect.New(ty)
		binary.ReadJSON(v.Interface(), []byte(arg), &err)
		if err != nil {
			return v, err
		}
		v = v.Elem()
	case reflect.Struct:
		binary.ReadJSON(v.Interface(), []byte(arg), &err)
		if err != nil {
			return v, err
		}
	case reflect.Slice:
		rt := ty.Elem()
		if rt.Kind() == reflect.Uint8 {
			// if hex, decode
			if len(arg) > 2 && arg[:2] == "0x" {
				arg = arg[2:]
				b, err := hex.DecodeString(arg)
				if err != nil {
					return v, err
				}
				v = reflect.ValueOf(b)
			} else {
				v = reflect.ValueOf([]byte(arg))
			}
		} else {
			v = reflect.New(ty)
			binary.ReadJSON(v.Interface(), []byte(arg), &err)
			if err != nil {
				return v, err
			}
			v = v.Elem()
		}
	case reflect.Int64:
		u, err := strconv.ParseInt(arg, 10, 64)
		if err != nil {
			return v, err
		}
		v = reflect.ValueOf(u)
	case reflect.Int32:
		u, err := strconv.ParseInt(arg, 10, 32)
		if err != nil {
			return v, err
		}
		v = reflect.ValueOf(u)
	case reflect.Uint64:
		u, err := strconv.ParseUint(arg, 10, 64)
		if err != nil {
			return v, err
		}
		v = reflect.ValueOf(u)
	case reflect.Uint:
		u, err := strconv.ParseUint(arg, 10, 32)
		if err != nil {
			return v, err
		}
		v = reflect.ValueOf(u)
	default:
		v = reflect.ValueOf(arg)
	}
	return v, nil
}

// covert an http query to a list of properly typed values.
// to be properly decoded the arg must be a concrete type from tendermint (if its an interface).
func queryToValues(funcInfo *FuncWrapper, r *http.Request) ([]reflect.Value, error) {
	argTypes := funcInfo.args
	argNames := funcInfo.argNames

	var err error
	values := make([]reflect.Value, len(argNames))
	for i, name := range argNames {
		ty := argTypes[i]
		arg := GetParam(r, name)
		values[i], err = jsonToArg(ty, arg)
		if err != nil {
			return nil, err
		}
	}
	return values, nil
}

// covert a list of interfaces to properly typed values
// TODO!
func paramsToValues(funcInfo *FuncWrapper, params []string) ([]reflect.Value, error) {
	values := make([]reflect.Value, len(params))
	for i, p := range params {
		ty := funcInfo.args[i]
		v, err := jsonToArg(ty, p)
		if err != nil {
			return nil, err
		}
		values[i] = v
	}
	return values, nil
}

// convert a list of values to a populated struct with the correct types
func returnsToResponse(funcInfo *FuncWrapper, returns []reflect.Value) (interface{}, error) {
	returnTypes := funcInfo.returns
	finalType := returnTypes[len(returnTypes)-1]
	if finalType.Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		errV := returns[len(returnTypes)-1]
		if errV.Interface() != nil {
			return nil, fmt.Errorf("%v", errV.Interface())
		}
	}

	// copy the response struct (New returns a pointer so we have to Elem() twice)
	v := reflect.New(funcInfo.response.Elem().Type()).Elem()
	nFields := v.NumField()
	for i := 0; i < nFields; i++ {
		field := v.FieldByIndex([]int{i})
		field.Set(returns[i])
	}

	return v.Interface(), nil
}

// jsonrpc calls grab the given method's function info and runs reflect.Call
func JsonRpcHandler(w http.ResponseWriter, r *http.Request) {
	b, _ := ioutil.ReadAll(r.Body)
	var jrpc JsonRpc
	err := json.Unmarshal(b, &jrpc)
	if err != nil {
		// TODO
	}

	funcInfo := funcMap[jrpc.Method]
	values, err := paramsToValues(funcInfo, jrpc.Params)
	if err != nil {
		WriteAPIResponse(w, API_INVALID_PARAM, nil, err.Error())
		return
	}
	returns := funcInfo.f.Call(values)
	response, err := returnsToResponse(funcInfo, returns)
	if err != nil {
		WriteAPIResponse(w, API_ERROR, nil, err.Error())
		return
	}
	WriteAPIResponse(w, API_OK, response, "")
}

func initHandlers() {
	// HTTP endpoints
	// toHandler runs once for each function and caches
	// all reflection data
	http.HandleFunc("/status", toHandler("status"))
	http.HandleFunc("/net_info", toHandler("net_info"))
	http.HandleFunc("/blockchain", toHandler("blockchain"))
	http.HandleFunc("/get_block", toHandler("get_block"))
	http.HandleFunc("/get_account", toHandler("get_account"))
	http.HandleFunc("/list_validators", toHandler("list_validators"))
	http.HandleFunc("/broadcast_tx", toHandler("broadcast_tx"))
	http.HandleFunc("/list_accounts", toHandler("list_accounts"))
	http.HandleFunc("/unsafe/gen_priv_account", toHandler("unsafe/gen_priv_account"))
	http.HandleFunc("/unsafe/sign_tx", toHandler("unsafe/sign_tx"))
	//http.HandleFunc("/call", CallHandler)
	//http.HandleFunc("/get_storage", GetStorageHandler)

	// JsonRPC endpoints
	http.HandleFunc("/", JsonRpcHandler)
	// unsafe JsonRPC endpoints
	//http.HandleFunc("/unsafe", UnsafeJsonRpcHandler)

}

type JsonRpc struct {
	JsonRpc string   `json:"jsonrpc"`
	Method  string   `json:"method"`
	Params  []string `json:"params"`
	Id      int      `json:"id"`
}

// this will panic if not passed a function
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
