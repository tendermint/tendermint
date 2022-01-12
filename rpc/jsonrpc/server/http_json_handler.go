package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sort"

	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/rpc/coretypes"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

// HTTP + JSON handler

// jsonrpc calls grab the given method's function info and runs reflect.Call
func makeJSONRPCHandler(funcMap map[string]*RPCFunc, logger log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, hreq *http.Request) {
		fail := func(res rpctypes.RPCResponse) {
			if err := WriteRPCResponseHTTPError(w, res); err != nil {
				logger.Error("Failed writing error response", "res", res, "err", err)
			}
		}

		// For POST requests, reject a non-root URL path. This should not happen
		// in the standard configuration, since the wrapper checks the path.
		if hreq.URL.Path != "/" {
			fail(rpctypes.RPCInvalidRequestError(nil, fmt.Errorf("invalid path: %q", hreq.URL.Path)))
			return
		}

		b, err := io.ReadAll(hreq.Body)
		if err != nil {
			fail(rpctypes.RPCInvalidRequestError(nil, fmt.Errorf("reading request body: %w", err)))
			return
		}

		// if its an empty request (like from a browser), just display a list of
		// functions
		if len(b) == 0 {
			writeListOfEndpoints(w, hreq, funcMap)
			return
		}

		requests, err := parseRequests(b)
		if err != nil {
			fail(rpctypes.RPCParseError(fmt.Errorf("decoding request: %w", err)))
			return
		}

		var responses []rpctypes.RPCResponse
		for _, req := range requests {
			// Ignore notifications, which this service does not support.
			if req.ID == nil {
				logger.Debug("Ignoring notification", "req", req)
				continue
			}

			rpcFunc, ok := funcMap[req.Method]
			if !ok || rpcFunc.ws {
				responses = append(responses, rpctypes.RPCMethodNotFoundError(req.ID))
				continue
			}

			args, err := parseParams(rpcFunc, hreq, req)
			if err != nil {
				responses = append(responses, rpctypes.RPCInvalidParamsError(
					req.ID, fmt.Errorf("converting JSON parameters: %w", err)))
				continue
			}

			returns := rpcFunc.f.Call(args)
			logger.Debug("HTTPJSONRPC", "method", req.Method, "args", args, "returns", returns)
			result, err := unreflectResult(returns)
			switch e := err.(type) {
			// if no error then return a success response
			case nil:
				responses = append(responses, rpctypes.NewRPCSuccessResponse(req.ID, result))

			// if this already of type RPC error then forward that error
			case *rpctypes.RPCError:
				responses = append(responses, rpctypes.NewRPCErrorResponse(req.ID, e.Code, e.Message, e.Data))
			default: // we need to unwrap the error and parse it accordingly
				switch errors.Unwrap(err) {
				// check if the error was due to an invald request
				case coretypes.ErrZeroOrNegativeHeight, coretypes.ErrZeroOrNegativePerPage,
					coretypes.ErrPageOutOfRange, coretypes.ErrInvalidRequest:
					responses = append(responses, rpctypes.RPCInvalidRequestError(req.ID, err))
				// lastly default all remaining errors as internal errors
				default: // includes ctypes.ErrHeightNotAvailable and ctypes.ErrHeightExceedsChainHead
					responses = append(responses, rpctypes.RPCInternalError(req.ID, err))
				}
			}
		}

		if len(responses) > 0 {
			if wErr := WriteRPCResponseHTTP(w, responses...); wErr != nil {
				logger.Error("failed to write responses", "err", wErr)
			}
		}
	}
}

func handleInvalidJSONRPCPaths(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Since the pattern "/" matches all paths not matched by other registered patterns,
		//  we check whether the path is indeed "/", otherwise return a 404 error
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		next(w, r)
	}
}

// parseRequests parses a JSON-RPC request or request batch from data.
func parseRequests(data []byte) ([]rpctypes.RPCRequest, error) {
	var reqs []rpctypes.RPCRequest
	var err error

	isArray := bytes.HasPrefix(bytes.TrimSpace(data), []byte("["))
	if isArray {
		err = json.Unmarshal(data, &reqs)
	} else {
		reqs = append(reqs, rpctypes.RPCRequest{})
		err = json.Unmarshal(data, &reqs[0])
	}
	if err != nil {
		return nil, err
	}
	return reqs, nil
}

func mapParamsToArgs(
	rpcFunc *RPCFunc,
	params map[string]json.RawMessage,
	argsOffset int,
) ([]reflect.Value, error) {

	values := make([]reflect.Value, len(rpcFunc.argNames))
	for i, argName := range rpcFunc.argNames {
		argType := rpcFunc.args[i+argsOffset]

		if p, ok := params[argName]; ok && p != nil && len(p) > 0 {
			val := reflect.New(argType)
			err := tmjson.Unmarshal(p, val.Interface())
			if err != nil {
				return nil, err
			}
			values[i] = val.Elem()
		} else { // use default for that type
			values[i] = reflect.Zero(argType)
		}
	}

	return values, nil
}

func arrayParamsToArgs(
	rpcFunc *RPCFunc,
	params []json.RawMessage,
	argsOffset int,
) ([]reflect.Value, error) {

	if len(rpcFunc.argNames) != len(params) {
		return nil, fmt.Errorf("expected %v parameters (%v), got %v (%v)",
			len(rpcFunc.argNames), rpcFunc.argNames, len(params), params)
	}

	values := make([]reflect.Value, len(params))
	for i, p := range params {
		argType := rpcFunc.args[i+argsOffset]
		val := reflect.New(argType)
		err := tmjson.Unmarshal(p, val.Interface())
		if err != nil {
			return nil, err
		}
		values[i] = val.Elem()
	}
	return values, nil
}

// parseParams parses the JSON parameters of rpcReq into the arguments of fn,
// returning the corresponding argument values or an error.
func parseParams(fn *RPCFunc, httpReq *http.Request, rpcReq rpctypes.RPCRequest) ([]reflect.Value, error) {
	ctx := rpctypes.WithCallInfo(httpReq.Context(), &rpctypes.CallInfo{
		RPCRequest:  &rpcReq,
		HTTPRequest: httpReq,
	})
	args := []reflect.Value{reflect.ValueOf(ctx)}
	if len(rpcReq.Params) == 0 {
		return args, nil
	}
	fargs, err := jsonParamsToArgs(fn, rpcReq.Params)
	if err != nil {
		return nil, err
	}
	return append(args, fargs...), nil
}

// raw is unparsed json (from json.RawMessage) encoding either a map or an
// array.
//
// Example:
//   rpcFunc.args = [context.Context string]
//   rpcFunc.argNames = ["arg"]
func jsonParamsToArgs(rpcFunc *RPCFunc, raw []byte) ([]reflect.Value, error) {
	const argsOffset = 1

	// TODO: Make more efficient, perhaps by checking the first character for '{' or '['?
	// First, try to get the map.
	var m map[string]json.RawMessage
	err := json.Unmarshal(raw, &m)
	if err == nil {
		return mapParamsToArgs(rpcFunc, m, argsOffset)
	}

	// Otherwise, try an array.
	var a []json.RawMessage
	err = json.Unmarshal(raw, &a)
	if err == nil {
		return arrayParamsToArgs(rpcFunc, a, argsOffset)
	}

	// Otherwise, bad format, we cannot parse
	return nil, fmt.Errorf("unknown type for JSON params: %v. Expected map or array", err)
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
		link := fmt.Sprintf("//%s/%s", r.Host, name)
		buf.WriteString(fmt.Sprintf("<a href=\"%s\">%s</a></br>", link, link))
	}

	buf.WriteString("<br>Endpoints that require arguments:<br>")
	for _, name := range argNames {
		link := fmt.Sprintf("//%s/%s?", r.Host, name)
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
	w.Write(buf.Bytes()) // nolint: errcheck
}
