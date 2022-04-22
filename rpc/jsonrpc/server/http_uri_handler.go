package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/tendermint/tendermint/libs/log"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

// uriReqID is a placeholder ID used for GET requests, which do not receive a
// JSON-RPC request ID from the caller.
const uriReqID = -1

// convert from a function name to the http handler
func makeHTTPHandler(rpcFunc *RPCFunc, logger log.Logger) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := rpctypes.WithCallInfo(req.Context(), &rpctypes.CallInfo{
			HTTPRequest: req,
		})
		args, err := parseURLParams(ctx, rpcFunc, req)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, err.Error())
			return
		}
		jreq := rpctypes.NewRequest(uriReqID)
		result, err := rpcFunc.Call(ctx, args)
		if err == nil {
			writeHTTPResponse(w, logger, jreq.MakeResponse(result))
		} else {
			writeHTTPResponse(w, logger, jreq.MakeError(err))
		}
	}
}

func parseURLParams(ctx context.Context, rf *RPCFunc, req *http.Request) ([]reflect.Value, error) {
	if err := req.ParseForm(); err != nil {
		return nil, fmt.Errorf("invalid HTTP request: %w", err)
	}
	getArg := func(name string) (string, bool) {
		if req.Form.Has(name) {
			return req.Form.Get(name), true
		}
		return "", false
	}

	params := make(map[string]interface{})
	for _, name := range rf.argNames {
		v, ok := getArg(name)
		if !ok {
			continue
		}
		if z, err := decodeInteger(v); err == nil {
			params[name] = z
		} else if b, err := strconv.ParseBool(v); err == nil {
			params[name] = b
		} else if lc := strings.ToLower(v); strings.HasPrefix(lc, "0x") {
			data, err := hex.DecodeString(lc[2:])
			if err != nil {
				return nil, err
			}
			params[name] = string(data)
		} else if isQuotedString(v) {
			var dec string
			if err := json.Unmarshal([]byte(v), &dec); err != nil {
				return nil, fmt.Errorf("invalid quoted string: %w", err)
			}
			params[name] = dec
		} else {
			params[name] = v
		}
	}
	bits, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	return parseParams(ctx, rf, bits)
}

// isQuotedString reports whether s is enclosed in double quotes.
func isQuotedString(s string) bool {
	return len(s) >= 2 && strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`)
}

// decodeInteger decodes s into an int64. If s is "double quoted" the quotes
// are removed; otherwise s must be a base-10 digit string.
func decodeInteger(s string) (int64, error) {
	if isQuotedString(s) {
		s = s[1 : len(s)-1]
	}
	return strconv.ParseInt(s, 10, 64)
}
