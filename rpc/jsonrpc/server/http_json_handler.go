package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	stdlog "log"
	"net/http"
	"reflect"
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

			result, err := rpcFunc.Call(ctx, args)
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
	// If fn does not accept parameters, there is no decoding to do, but verify
	// that no parameters were passed.
	if fn.param == nil {
		if len(paramData) != 0 && !bytes.Equal(paramData, []byte("null")) {
			return nil, errors.New("method does not take parameters")
		}
		return []reflect.Value{reflect.ValueOf(ctx)}, nil
	}
	bits, err := adjustParams(fn, paramData)
	if err != nil {
		return nil, err
	}
	arg := reflect.New(fn.param)
	if err := json.Unmarshal(bits, arg.Interface()); err != nil {
		stdlog.Printf("MJF :: unmarshal bits=%#q err=%v", string(bits), err)
		return nil, err
	}
	return []reflect.Value{reflect.ValueOf(ctx), arg}, nil
}

// adjustParams checks whether data is encoded as a JSON array, and if so
// adjusts the values to match the corresponding parameter names.
func adjustParams(fn *RPCFunc, data []byte) (json.RawMessage, error) {
	base := bytes.TrimSpace(data)
	if bytes.HasPrefix(base, []byte("[")) {
		var args []json.RawMessage
		if err := json.Unmarshal(base, &args); err != nil {
			return nil, err
		} else if len(args) != len(fn.argNames) {
			return nil, fmt.Errorf("got %d arguments, want %d", len(args), len(fn.argNames))
		}
		m := make(map[string]json.RawMessage)
		for i, arg := range args {
			m[fn.argNames[i]] = arg
		}
		return json.Marshal(m)
	} else if !bytes.HasPrefix(base, []byte("{")) {
		return nil, errors.New("parameters must be an object or an array")
	}
	return base, nil
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
