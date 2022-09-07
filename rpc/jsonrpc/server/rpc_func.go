package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/tendermint/tendermint/libs/log"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

// DefaultRPCTimeout is the default context timeout for calls to any RPC method
// that does not override it with a more specific timeout.
const DefaultRPCTimeout = 60 * time.Second

// RegisterRPCFuncs adds a route to mux for each non-websocket function in the
// funcMap, and also a root JSON-RPC POST handler.
func RegisterRPCFuncs(mux *http.ServeMux, funcMap map[string]*RPCFunc, logger log.Logger) {
	for name, fn := range funcMap {
		if fn.ws {
			continue // skip websocket endpoints, not usable via GET calls
		}
		mux.HandleFunc("/"+name, ensureBodyClose(makeHTTPHandler(fn, logger)))
	}

	// Endpoints for POST.
	mux.HandleFunc("/", ensureBodyClose(handleInvalidJSONRPCPaths(makeJSONRPCHandler(funcMap, logger))))
}

// Function introspection

// RPCFunc contains the introspected type information for a function.
type RPCFunc struct {
	f       reflect.Value // underlying rpc function
	param   reflect.Type  // the parameter struct, or nil
	result  reflect.Type  // the non-error result type, or nil
	args    []argInfo     // names and type information (for URL decoding)
	timeout time.Duration // default request timeout, 0 means none
	ws      bool          // websocket only
}

// argInfo records the name of a field, along with a bit to tell whether the
// value of the field requires binary data, having underlying type []byte.  The
// flag is needed when decoding URL parameters, where we permit quoted strings
// to be passed for either argument type.
type argInfo struct {
	name     string
	isBinary bool // value wants binary data
}

// Call parses the given JSON parameters and calls the function wrapped by rf
// with the resulting argument value. It reports an error if parameter parsing
// fails, otherwise it returns the result from the wrapped function.
func (rf *RPCFunc) Call(ctx context.Context, params json.RawMessage) (interface{}, error) {
	// If ctx has its own deadline we will respect it; otherwise use rf.timeout.
	if _, ok := ctx.Deadline(); !ok && rf.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, rf.timeout)
		defer cancel()
	}
	args, err := rf.parseParams(ctx, params)
	if err != nil {
		return nil, err
	}
	returns := rf.f.Call(args)

	// Case 1: There is no non-error result type.
	if rf.result == nil {
		if oerr := returns[0].Interface(); oerr != nil {
			return nil, oerr.(error)
		}
		return nil, nil
	}

	// Case 2: There is a non-error result.
	if oerr := returns[1].Interface(); oerr != nil {
		// In case of error, report the error and ignore the result.
		return nil, oerr.(error)
	}
	return returns[0].Interface(), nil
}

// Timeout updates rf to include a default timeout for calls to rf. This
// timeout is used if one is not already provided on the request context.
// Setting d == 0 means there will be no timeout. Returns rf to allow chaining.
func (rf *RPCFunc) Timeout(d time.Duration) *RPCFunc { rf.timeout = d; return rf }

// parseParams parses the parameters of a JSON-RPC request and returns the
// corresponding argument values. On success, the first argument value will be
// the value of ctx.
func (rf *RPCFunc) parseParams(ctx context.Context, params json.RawMessage) ([]reflect.Value, error) {
	// If rf does not accept parameters, there is no decoding to do, but verify
	// that no parameters were passed.
	if rf.param == nil {
		if !isNullOrEmpty(params) {
			return nil, invalidParamsError("no parameters accepted for this method")
		}
		return []reflect.Value{reflect.ValueOf(ctx)}, nil
	}
	bits, err := rf.adjustParams(params)
	if err != nil {
		return nil, invalidParamsError(err.Error())
	}
	arg := reflect.New(rf.param)
	if err := json.Unmarshal(bits, arg.Interface()); err != nil {
		return nil, invalidParamsError(err.Error())
	}
	return []reflect.Value{reflect.ValueOf(ctx), arg}, nil
}

// adjustParams checks whether data is encoded as a JSON array, and if so
// adjusts the values to match the corresponding parameter names.
func (rf *RPCFunc) adjustParams(data []byte) (json.RawMessage, error) {
	base := bytes.TrimSpace(data)
	if bytes.HasPrefix(base, []byte("[")) {
		var args []json.RawMessage
		if err := json.Unmarshal(base, &args); err != nil {
			return nil, err
		} else if len(args) != len(rf.args) {
			return nil, fmt.Errorf("got %d arguments, want %d", len(args), len(rf.args))
		}
		m := make(map[string]json.RawMessage)
		for i, arg := range args {
			m[rf.args[i].name] = arg
		}
		return json.Marshal(m)
	} else if bytes.HasPrefix(base, []byte("{")) || bytes.Equal(base, []byte("null")) {
		return base, nil
	}
	return nil, errors.New("parameters must be an object or an array")

}

// NewRPCFunc constructs an RPCFunc for f, which must be a function whose type
// signature matches one of these schemes:
//
//     func(context.Context) error
//     func(context.Context) (R, error)
//     func(context.Context, *T) error
//     func(context.Context, *T) (R, error)
//
// for an arbitrary struct type T and type R. NewRPCFunc will panic if f does
// not have one of these forms.  A newly-constructed RPCFunc has a default
// timeout of DefaultRPCTimeout; use the Timeout method to adjust this as
// needed.
func NewRPCFunc(f interface{}) *RPCFunc {
	rf, err := newRPCFunc(f)
	if err != nil {
		panic("invalid RPC function: " + err.Error())
	}
	return rf
}

// NewWSRPCFunc behaves as NewRPCFunc, but marks the resulting function for use
// via websocket.
func NewWSRPCFunc(f interface{}) *RPCFunc {
	rf := NewRPCFunc(f)
	rf.ws = true
	return rf
}

var (
	ctxType = reflect.TypeOf((*context.Context)(nil)).Elem()
	errType = reflect.TypeOf((*error)(nil)).Elem()
)

// newRPCFunc constructs an RPCFunc for f. See the comment at NewRPCFunc.
func newRPCFunc(f interface{}) (*RPCFunc, error) {
	if f == nil {
		return nil, errors.New("nil function")
	}

	// Check the type and signature of f.
	fv := reflect.ValueOf(f)
	if fv.Kind() != reflect.Func {
		return nil, errors.New("not a function")
	}

	var ptype reflect.Type
	ft := fv.Type()
	if np := ft.NumIn(); np == 0 || np > 2 {
		return nil, errors.New("wrong number of parameters")
	} else if ft.In(0) != ctxType {
		return nil, errors.New("first parameter is not context.Context")
	} else if np == 2 {
		ptype = ft.In(1)
		if ptype.Kind() != reflect.Ptr {
			return nil, errors.New("parameter type is not a pointer")
		}
		ptype = ptype.Elem()
		if ptype.Kind() != reflect.Struct {
			return nil, errors.New("parameter type is not a struct")
		}
	}

	var rtype reflect.Type
	if no := ft.NumOut(); no < 1 || no > 2 {
		return nil, errors.New("wrong number of results")
	} else if ft.Out(no-1) != errType {
		return nil, errors.New("last result is not error")
	} else if no == 2 {
		rtype = ft.Out(0)
	}

	var args []argInfo
	if ptype != nil {
		for i := 0; i < ptype.NumField(); i++ {
			field := ptype.Field(i)
			if tag := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]; tag != "" && tag != "-" {
				args = append(args, argInfo{
					name:     tag,
					isBinary: isByteArray(field.Type),
				})
			} else if tag == "-" {
				// If the tag is "-" the field should explicitly be ignored, even
				// if it is otherwise eligible.
			} else if field.IsExported() && !field.Anonymous {
				// Examples: Name → name, MaxEffort → maxEffort.
				// Note that this is an aesthetic choice; the standard decoder will
				// match without regard to case anyway.
				name := strings.ToLower(field.Name[:1]) + field.Name[1:]
				args = append(args, argInfo{
					name:     name,
					isBinary: isByteArray(field.Type),
				})
			}
		}
	}

	return &RPCFunc{
		f:       fv,
		param:   ptype,
		result:  rtype,
		args:    args,
		timeout: DefaultRPCTimeout, // until overridden
	}, nil
}

// invalidParamsError returns an RPC invalid parameters error with the given
// detail message.
func invalidParamsError(msg string, args ...interface{}) error {
	return &rpctypes.RPCError{
		Code:    int(rpctypes.CodeInvalidParams),
		Message: rpctypes.CodeInvalidParams.String(),
		Data:    fmt.Sprintf(msg, args...),
	}
}

// isNullOrEmpty reports whether params is either itself empty or represents an
// empty parameter (null, empty object, or empty array).
func isNullOrEmpty(params json.RawMessage) bool {
	return len(params) == 0 ||
		bytes.Equal(params, []byte("null")) ||
		bytes.Equal(params, []byte("{}")) ||
		bytes.Equal(params, []byte("[]"))
}

// isByteArray reports whether t is (equivalent to) []byte.
func isByteArray(t reflect.Type) bool {
	return t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8
}
