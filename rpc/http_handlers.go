package rpc

import (
	"encoding/json"
	"fmt"
	"github.com/tendermint/tendermint/binary"
	"github.com/tendermint/tendermint/rpc/core"
	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
)

// map each function to the argument names
var argsMap = map[string][]string{
	"status":           []string{},
	"net_info":         []string{},
	"blockchain":       []string{"min_height", "max_height"},
	"get_block":        []string{"height"},
	"get_account":      []string{"address"},
	"list_validators":  []string{},
	"broadcast_tx":     []string{"tx"},
	"list_accounts":    []string{},
	"gen_priv_account": []string{},
	"sign_tx":          []string{"tx", "privAccounts"},
}

// cache all type information about each function up front
var funcMap = map[string]*FuncWrapper{
	"status":           funcWrap("status", core.Status),
	"net_info":         funcWrap("net_info", core.NetInfo),
	"blockchain":       funcWrap("blockchain", core.BlockchainInfo),
	"get_block":        funcWrap("get_block", core.GetBlock),
	"get_account":      funcWrap("get_account", core.GetAccount),
	"list_validators":  funcWrap("list_validators", core.ListValidators),
	"broadcast_tx":     funcWrap("broadcast_tx", core.BroadcastTx),
	"list_accounts":    funcWrap("list_accounts", core.ListAccounts),
	"gen_priv_account": funcWrap("gen_priv_account", core.GenPrivAccount),
	"sign_tx":          funcWrap("sign_tx", core.SignTx),
}

// map each function to an empty struct which can hold its return values
var responseMap = map[string]reflect.Value{
	"status":           reflect.ValueOf(&ResponseStatus{}),
	"net_info":         reflect.ValueOf(&ResponseNetInfo{}),
	"blockchain":       reflect.ValueOf(&ResponseBlockchainInfo{}),
	"get_block":        reflect.ValueOf(&ResponseGetBlock{}),
	"get_account":      reflect.ValueOf(&ResponseGetAccount{}),
	"list_validators":  reflect.ValueOf(&ResponseListValidators{}),
	"broadcast_tx":     reflect.ValueOf(&ResponseBroadcastTx{}),
	"list_accounts":    reflect.ValueOf(&ResponseListAccounts{}),
	"gen_priv_account": reflect.ValueOf(&ResponseGenPrivAccount{}),
	"sign_tx":          reflect.ValueOf(&ResponseSignTx{}),
}

// holds all type information for each function
type FuncWrapper struct {
	f        reflect.Value  // function from "rpc/core"
	args     []reflect.Type // type of each function arg
	returns  []reflect.Type // type of each return arg
	argNames []string       // name of each argument
	response reflect.Value  // response struct (to be filled with "returns")
}

func funcWrap(name string, f interface{}) *FuncWrapper {
	return &FuncWrapper{
		f:        reflect.ValueOf(f),
		args:     funcArgTypes(f),
		returns:  funcReturnTypes(f),
		argNames: argsMap[name],
		response: responseMap[name],
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

// covert an http query to a list of properly typed values
// to be properly decoded the arg must be a concrete type from tendermint
func queryToValues(funcInfo *FuncWrapper, r *http.Request) ([]reflect.Value, error) {
	argTypes := funcInfo.args
	argNames := funcInfo.argNames

	var err error
	values := make([]reflect.Value, len(argNames))
	for i, name := range argNames {
		ty := argTypes[i]
		v := reflect.New(ty).Elem()
		kind := v.Kind()
		arg := GetParam(r, name)
		switch kind {
		case reflect.Interface:
			v = reflect.New(ty)
			binary.ReadJSON(v.Interface(), []byte(arg), &err)
			if err != nil {
				return nil, err
			}
			v = v.Elem()
		case reflect.Struct:
			binary.ReadJSON(v.Interface(), []byte(arg), &err)
			if err != nil {
				return nil, err
			}
		case reflect.Slice:
			rt := ty.Elem()
			if rt.Kind() == reflect.Uint8 {
				v = reflect.ValueOf([]byte(arg))
			} else {
				v = reflect.New(ty)
				binary.ReadJSON(v.Interface(), []byte(arg), &err)
				if err != nil {
					return nil, err
				}
				v = v.Elem()
			}
		case reflect.Int64:
			u, err := strconv.ParseInt(arg, 10, 64)
			if err != nil {
				return nil, err
			}
			v = reflect.ValueOf(u)
		case reflect.Int32:
			u, err := strconv.ParseInt(arg, 10, 32)
			if err != nil {
				return nil, err
			}
			v = reflect.ValueOf(u)
		case reflect.Uint64:
			u, err := strconv.ParseUint(arg, 10, 64)
			if err != nil {
				return nil, err
			}
			v = reflect.ValueOf(u)
		case reflect.Uint:
			u, err := strconv.ParseUint(arg, 10, 32)
			if err != nil {
				return nil, err
			}
			v = reflect.ValueOf(u)
		default:
			v = reflect.ValueOf(arg)
		}
		values[i] = v
	}
	return values, nil
}

// covert a list of interfaces to properly typed values
// TODO!
func paramsToValues(funcInfo *FuncWrapper, params []interface{}) ([]reflect.Value, error) {
	values := make([]reflect.Value, len(params))
	for i, p := range params {
		values[i] = reflect.ValueOf(p)
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

	v := funcInfo.response.Elem()
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
	http.HandleFunc("/unsafe/gen_priv_account", toHandler("gen_priv_account"))
	http.HandleFunc("/unsafe/sign_tx", toHandler("sign_tx"))
	//http.HandleFunc("/call", CallHandler)
	//http.HandleFunc("/get_storage", GetStorageHandler)

	// JsonRPC endpoints
	http.HandleFunc("/", JsonRpcHandler)
	// unsafe JsonRPC endpoints
	//http.HandleFunc("/unsafe", UnsafeJsonRpcHandler)

}

type JsonRpc struct {
	JsonRpc string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	Id      int           `json:"id"`
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
