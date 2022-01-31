package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/tendermint/tendermint/libs/log"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

// HTTP + JSON handler

// jsonrpc calls grab the given method's function info and runs reflect.Call
func makeJSONRPCHandler(funcMap map[string]*RPCFunc, logger log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, hreq *http.Request) {
		// For POST requests, reject a non-root URL path. This should not happen
		// in the standard configuration, since the wrapper checks the path.
		if hreq.URL.Path != "/" {
			writeRPCResponse(w, logger, rpctypes.RPCRequest{}.MakeErrorf(
				rpctypes.CodeInvalidRequest, "invalid path: %q", hreq.URL.Path))
			return
		}

		b, err := io.ReadAll(hreq.Body)
		if err != nil {
			writeRPCResponse(w, logger, rpctypes.RPCRequest{}.MakeErrorf(
				rpctypes.CodeInvalidRequest, "reading request body: %v", err))
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
			writeRPCResponse(w, logger, rpctypes.RPCRequest{}.MakeErrorf(
				rpctypes.CodeParseError, "decoding request: %v", err))
			return
		}

		var responses []rpctypes.RPCResponse
		for _, req := range requests {
			// Ignore notifications, which this service does not support.
			if req.IsNotification() {
				logger.Debug("Ignoring notification", "req", req)
				continue
			}

			rpcFunc, ok := funcMap[req.Method]
			if !ok || rpcFunc.ws {
				responses = append(responses, req.MakeErrorf(rpctypes.CodeMethodNotFound, req.Method))
				continue
			}

			req := req
			ctx := rpctypes.WithCallInfo(hreq.Context(), &rpctypes.CallInfo{
				RPCRequest:  &req,
				HTTPRequest: hreq,
			})
			args, err := parseParams(ctx, rpcFunc, req.Params)
			if err != nil {
				responses = append(responses,
					req.MakeErrorf(rpctypes.CodeInvalidParams, "converting JSON parameters: %v", err))
				continue
			}

			returns := rpcFunc.f.Call(args)
			result, err := unreflectResult(returns)
			if err == nil {
				responses = append(responses, req.MakeResponse(result))
			} else {
				responses = append(responses, req.MakeError(err))
			}
		}

		if len(responses) == 0 {
			return
		}
		writeRPCResponse(w, logger, responses...)
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

// parseParams parses the JSON parameters of rpcReq into the arguments of fn,
// returning the corresponding argument values or an error.
func parseParams(ctx context.Context, fn *RPCFunc, paramData []byte) ([]reflect.Value, error) {
	params, err := parseJSONParams(fn, paramData)
	if err != nil {
		return nil, err
	}

	args := make([]reflect.Value, 1+len(params))
	args[0] = reflect.ValueOf(ctx)
	for i, param := range params {
		ptype := fn.args[i+1]
		if len(param) == 0 {
			args[i+1] = reflect.Zero(ptype)
			continue
		}

		var pval reflect.Value
		isPtr := ptype.Kind() == reflect.Ptr
		if isPtr {
			pval = reflect.New(ptype.Elem())
		} else {
			pval = reflect.New(ptype)
		}
		baseType := pval.Type().Elem()

		if isIntType(baseType) && isStringValue(param) {
			var z int64String
			if err := json.Unmarshal(param, &z); err != nil {
				return nil, fmt.Errorf("decoding string %q: %w", fn.argNames[i], err)
			}
			pval.Elem().Set(reflect.ValueOf(z).Convert(baseType))
		} else if err := json.Unmarshal(param, pval.Interface()); err != nil {
			return nil, fmt.Errorf("decoding %q: %w", fn.argNames[i], err)
		}

		if isPtr {
			args[i+1] = pval
		} else {
			args[i+1] = pval.Elem()
		}
	}
	return args, nil
}

// parseJSONParams parses data and returns a slice of JSON values matching the
// positional parameters of fn. It reports an error if data is not "null" and
// does not encode an object or an array, or if the number of array parameters
// does not match the argument list of fn (excluding the context).
func parseJSONParams(fn *RPCFunc, data []byte) ([]json.RawMessage, error) {
	base := bytes.TrimSpace(data)
	if bytes.HasPrefix(base, []byte("{")) {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(base, &m); err != nil {
			return nil, fmt.Errorf("decoding parameter object: %w", err)
		}
		out := make([]json.RawMessage, len(fn.argNames))
		for i, name := range fn.argNames {
			if p, ok := m[name]; ok {
				out[i] = p
			}
		}
		return out, nil

	} else if bytes.HasPrefix(base, []byte("[")) {
		var m []json.RawMessage
		if err := json.Unmarshal(base, &m); err != nil {
			return nil, fmt.Errorf("decoding parameter array: %w", err)
		}
		if len(m) != len(fn.argNames) {
			return nil, fmt.Errorf("got %d parameters, want %d", len(m), len(fn.argNames))
		}
		return m, nil

	} else if bytes.Equal(base, []byte("null")) {
		return make([]json.RawMessage, len(fn.argNames)), nil
	}

	return nil, errors.New("parameters must be an object or an array")
}

// isStringValue reports whether data is a JSON string value.
func isStringValue(data json.RawMessage) bool {
	return len(data) != 0 && data[0] == '"'
}

type int64String int64

func (z *int64String) UnmarshalText(data []byte) error {
	v, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}
	*z = int64String(v)
	return nil
}

// writes a list of available rpc endpoints as an html page
func writeListOfEndpoints(w http.ResponseWriter, r *http.Request, funcMap map[string]*RPCFunc) {
	hasArgs := make(map[string]string)
	noArgs := make(map[string]string)
	for name, rf := range funcMap {
		base := fmt.Sprintf("//%s/%s", r.Host, name)
		// N.B. Check argNames, not args, since the type list includes the type
		// of the leading context argument.
		if len(rf.argNames) == 0 {
			noArgs[name] = base
		} else {
			query := append([]string(nil), rf.argNames...)
			for i, arg := range query {
				query[i] = arg + "=_"
			}
			hasArgs[name] = base + "?" + strings.Join(query, "&")
		}
	}
	w.Header().Set("Content-Type", "text/html")
	_ = listOfEndpoints.Execute(w, map[string]map[string]string{
		"NoArgs":  noArgs,
		"HasArgs": hasArgs,
	})
}

var listOfEndpoints = template.Must(template.New("list").Parse(`<html>
<head><title>List of RPC Endpoints</title></head>
<body>

<h1>Available RPC endpoints:</h1>

{{if .NoArgs}}
<hr />
<h2>Endpoints with no arguments:</h2>

<ul>
{{range $link := .NoArgs}}  <li><a href="{{$link}}">{{$link}}</a></li>
{{end -}}
</ul>{{end}}

{{if .HasArgs}}
<hr />
<h2>Endpoints that require arguments:</h2>

<ul>
{{range $link := .HasArgs}}  <li><a href="{{$link}}">{{$link}}</a></li>
{{end -}}
</ul>{{end}}

</body></html>`))
