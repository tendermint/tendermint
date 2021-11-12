package server

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"reflect"
	"regexp"
	"strings"

	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/rpc/coretypes"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

// HTTP + URI handler

var reInt = regexp.MustCompile(`^-?[0-9]+$`)

// convert from a function name to the http handler
func makeHTTPHandler(rpcFunc *RPCFunc, logger log.Logger) func(http.ResponseWriter, *http.Request) {
	// Always return -1 as there's no ID here.
	dummyID := rpctypes.JSONRPCIntID(-1) // URIClientRequestID

	// Exception for websocket endpoints
	if rpcFunc.ws {
		return func(w http.ResponseWriter, r *http.Request) {
			res := rpctypes.RPCMethodNotFoundError(dummyID)
			if wErr := WriteRPCResponseHTTPError(w, res); wErr != nil {
				logger.Error("failed to write response", "res", res, "err", wErr)
			}
		}
	}

	// All other endpoints
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("HTTP HANDLER", "req", dumpHTTPRequest(r))

		ctx := &rpctypes.Context{HTTPReq: r}
		args := []reflect.Value{reflect.ValueOf(ctx)}

		fnArgs, err := httpParamsToArgs(rpcFunc, r)
		if err != nil {
			res := rpctypes.RPCInvalidParamsError(dummyID,
				fmt.Errorf("error converting http params to arguments: %w", err),
			)
			if wErr := WriteRPCResponseHTTPError(w, res); wErr != nil {
				logger.Error("failed to write response", "res", res, "err", wErr)
			}
			return
		}
		args = append(args, fnArgs...)

		returns := rpcFunc.f.Call(args)

		logger.Debug("HTTPRestRPC", "method", r.URL.Path, "args", args, "returns", returns)
		result, err := unreflectResult(returns)
		switch e := err.(type) {
		// if no error then return a success response
		case nil:
			res := rpctypes.NewRPCSuccessResponse(dummyID, result)
			if wErr := WriteRPCResponseHTTP(w, rpcFunc.cache, res); wErr != nil {
				logger.Error("failed to write response", "res", res, "err", wErr)
			}

		// if this already of type RPC error then forward that error.
		case *rpctypes.RPCError:
			res := rpctypes.NewRPCErrorResponse(dummyID, e.Code, e.Message, e.Data)
			if wErr := WriteRPCResponseHTTPError(w, res); wErr != nil {
				logger.Error("failed to write response", "res", res, "err", wErr)
			}

		default: // we need to unwrap the error and parse it accordingly
			var res rpctypes.RPCResponse

			switch errors.Unwrap(err) {
			case coretypes.ErrZeroOrNegativeHeight,
				coretypes.ErrZeroOrNegativePerPage,
				coretypes.ErrPageOutOfRange,
				coretypes.ErrInvalidRequest:
				res = rpctypes.RPCInvalidRequestError(dummyID, err)
			default: // ctypes.ErrHeightNotAvailable, ctypes.ErrHeightExceedsChainHead:
				res = rpctypes.RPCInternalError(dummyID, err)
			}

			if wErr := WriteRPCResponseHTTPError(w, res); wErr != nil {
				logger.Error("failed to write response", "res", res, "err", wErr)
			}
		}

	}
}

// Covert an http query to a list of properly typed values.
// To be properly decoded the arg must be a concrete type from tendermint (if its an interface).
func httpParamsToArgs(rpcFunc *RPCFunc, r *http.Request) ([]reflect.Value, error) {
	// skip types.Context
	const argsOffset = 1

	values := make([]reflect.Value, len(rpcFunc.argNames))

	for i, name := range rpcFunc.argNames {
		argType := rpcFunc.args[i+argsOffset]

		values[i] = reflect.Zero(argType) // set default for that type

		arg := getParam(r, name)
		// log.Notice("param to arg", "argType", argType, "name", name, "arg", arg)

		if arg == "" {
			continue
		}

		v, ok, err := nonJSONStringToArg(argType, arg)
		if err != nil {
			return nil, err
		}
		if ok {
			values[i] = v
			continue
		}

		values[i], err = jsonStringToArg(argType, arg)
		if err != nil {
			return nil, err
		}
	}

	return values, nil
}

func jsonStringToArg(rt reflect.Type, arg string) (reflect.Value, error) {
	rv := reflect.New(rt)
	err := tmjson.Unmarshal([]byte(arg), rv.Interface())
	if err != nil {
		return rv, err
	}
	rv = rv.Elem()
	return rv, nil
}

func nonJSONStringToArg(rt reflect.Type, arg string) (reflect.Value, bool, error) {
	if rt.Kind() == reflect.Ptr {
		rv1, ok, err := nonJSONStringToArg(rt.Elem(), arg)
		switch {
		case err != nil:
			return reflect.Value{}, false, err
		case ok:
			rv := reflect.New(rt.Elem())
			rv.Elem().Set(rv1)
			return rv, true, nil
		default:
			return reflect.Value{}, false, nil
		}
	} else {
		return _nonJSONStringToArg(rt, arg)
	}
}

// NOTE: rt.Kind() isn't a pointer.
func _nonJSONStringToArg(rt reflect.Type, arg string) (reflect.Value, bool, error) {
	isIntString := reInt.Match([]byte(arg))
	isQuotedString := strings.HasPrefix(arg, `"`) && strings.HasSuffix(arg, `"`)
	isHexString := strings.HasPrefix(strings.ToLower(arg), "0x")

	var expectingString, expectingByteSlice, expectingInt bool
	switch rt.Kind() {
	case reflect.Int,
		reflect.Uint,
		reflect.Int8,
		reflect.Uint8,
		reflect.Int16,
		reflect.Uint16,
		reflect.Int32,
		reflect.Uint32,
		reflect.Int64,
		reflect.Uint64:
		expectingInt = true
	case reflect.String:
		expectingString = true
	case reflect.Slice:
		expectingByteSlice = rt.Elem().Kind() == reflect.Uint8
	}

	if isIntString && expectingInt {
		qarg := `"` + arg + `"`
		rv, err := jsonStringToArg(rt, qarg)
		if err != nil {
			return rv, false, err
		}

		return rv, true, nil
	}

	if isHexString {
		if !expectingString && !expectingByteSlice {
			err := fmt.Errorf("got a hex string arg, but expected '%s'",
				rt.Kind().String())
			return reflect.ValueOf(nil), false, err
		}

		var value []byte
		value, err := hex.DecodeString(arg[2:])
		if err != nil {
			return reflect.ValueOf(nil), false, err
		}
		if rt.Kind() == reflect.String {
			return reflect.ValueOf(string(value)), true, nil
		}
		return reflect.ValueOf(value), true, nil
	}

	if isQuotedString && expectingByteSlice {
		v := reflect.New(reflect.TypeOf(""))
		err := tmjson.Unmarshal([]byte(arg), v.Interface())
		if err != nil {
			return reflect.ValueOf(nil), false, err
		}
		v = v.Elem()
		return reflect.ValueOf([]byte(v.String())), true, nil
	}

	return reflect.ValueOf(nil), false, nil
}

func getParam(r *http.Request, param string) string {
	s := r.URL.Query().Get(param)
	if s == "" {
		s = r.FormValue(param)
	}
	return s
}

func dumpHTTPRequest(r *http.Request) string {
	d, e := httputil.DumpRequest(r, true)
	if e != nil {
		return e.Error()
	}

	return string(d)
}
