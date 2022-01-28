package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"

	"github.com/tendermint/tendermint/libs/log"
)

// RegisterRPCFuncs adds a route to mux for each non-websocket function in the
// funcMap, and also a root JSON-RPC POST handler.
func RegisterRPCFuncs(mux *http.ServeMux, funcMap map[string]*RPCFunc, logger log.Logger) {
	for name, fn := range funcMap {
		if fn.ws {
			continue // skip websocket endpoints, not usable via GET calls
		}
		mux.HandleFunc("/"+name, makeHTTPHandler(fn, logger))
	}

	// Endpoints for POST.
	mux.HandleFunc("/", handleInvalidJSONRPCPaths(makeJSONRPCHandler(funcMap, logger)))
}

// Function introspection

// RPCFunc contains the introspected type information for a function.
type RPCFunc struct {
	f        reflect.Value  // underlying rpc function
	args     []reflect.Type // type of each function arg
	returns  []reflect.Type // type of each return arg
	argNames []string       // name of each argument
	ws       bool           // websocket only
}

// NewRPCFunc constructs an RPCFunc for f, which must be a function whose type
// signature matches one of these schemes:
//
//     func(context.Context, T1, T2, ...) error
//     func(context.Context, T1, T2, ...) (R, error)
//
// for arbitrary types T_i and R. The number of argNames must exactly match the
// number of non-context arguments to f. Otherwise, NewRPCFunc panics.
//
// The parameter names given are used to map JSON object keys to the
// corresonding parameter of the function. The names do not need to match the
// declared names, but must match what the client sends in a request.
func NewRPCFunc(f interface{}, argNames ...string) *RPCFunc {
	rf, err := newRPCFunc(f, argNames)
	if err != nil {
		panic("invalid RPC function: " + err.Error())
	}
	return rf
}

// NewWSRPCFunc behaves as NewRPCFunc, but marks the resulting function for use
// via websocket.
func NewWSRPCFunc(f interface{}, argNames ...string) *RPCFunc {
	rf := NewRPCFunc(f, argNames...)
	rf.ws = true
	return rf
}

var (
	ctxType = reflect.TypeOf((*context.Context)(nil)).Elem()
	errType = reflect.TypeOf((*error)(nil)).Elem()
)

// newRPCFunc constructs an RPCFunc for f. See the comment at NewRPCFunc.
func newRPCFunc(f interface{}, argNames []string) (*RPCFunc, error) {
	if f == nil {
		return nil, errors.New("nil function")
	}

	// Check the type and signature of f.
	fv := reflect.ValueOf(f)
	if fv.Kind() != reflect.Func {
		return nil, errors.New("not a function")
	}

	ft := fv.Type()
	if np := ft.NumIn(); np == 0 {
		return nil, errors.New("wrong number of parameters")
	} else if ft.In(0) != ctxType {
		return nil, errors.New("first parameter is not context.Context")
	} else if np-1 != len(argNames) {
		return nil, fmt.Errorf("have %d names for %d parameters", len(argNames), np-1)
	}

	if no := ft.NumOut(); no < 1 || no > 2 {
		return nil, errors.New("wrong number of results")
	} else if ft.Out(no-1) != errType {
		return nil, errors.New("last result is not error")
	}

	args := make([]reflect.Type, ft.NumIn())
	for i := 0; i < ft.NumIn(); i++ {
		args[i] = ft.In(i)
	}
	outs := make([]reflect.Type, ft.NumOut())
	for i := 0; i < ft.NumOut(); i++ {
		outs[i] = ft.Out(i)
	}
	return &RPCFunc{
		f:        fv,
		args:     args,
		returns:  outs,
		argNames: argNames,
	}, nil
}

//-------------------------------------------------------------

// NOTE: assume returns is result struct and error. If error is not nil, return it
func unreflectResult(returns []reflect.Value) (interface{}, error) {
	errV := returns[1]
	if err, ok := errV.Interface().(error); ok && err != nil {
		return nil, err
	}
	rv := returns[0]
	// the result is a registered interface,
	// we need a pointer to it so we can marshal with type byte
	rvp := reflect.New(rv.Type())
	rvp.Elem().Set(rv)
	return rvp.Interface(), nil
}
