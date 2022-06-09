package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
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
			result, err := rpcFunc.Call(ctx, req.Params)
			if err != nil {
				responses = append(responses, req.MakeError(err))
			} else {
				responses = append(responses, req.MakeResponse(result))
			}
		}

		if len(responses) == 0 {
			return
		}
		writeRPCResponse(w, logger, responses...)
	}
}

func ensureBodyClose(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		next(w, r)
	}
}

func handleInvalidJSONRPCPaths(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

// writes a list of available rpc endpoints as an html page
func writeListOfEndpoints(w http.ResponseWriter, r *http.Request, funcMap map[string]*RPCFunc) {
	hasArgs := make(map[string]string)
	noArgs := make(map[string]string)
	for name, rf := range funcMap {
		base := fmt.Sprintf("//%s/%s", r.Host, name)
		if len(rf.args) == 0 {
			noArgs[name] = base
			continue
		}
		var query []string
		for _, arg := range rf.args {
			query = append(query, arg.name+"=_")
		}
		hasArgs[name] = base + "?" + strings.Join(query, "&")
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
