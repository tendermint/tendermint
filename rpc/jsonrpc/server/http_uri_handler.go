package server

import (
	"context"
	"encoding"
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
		outs := rpcFunc.f.Call(args)

		logger.Debug("HTTPRestRPC", "method", req.URL.Path, "args", args, "returns", outs)
		result, err := unreflectResult(outs)
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

	vals := make([]reflect.Value, len(rf.argNames)+1)
	vals[0] = reflect.ValueOf(ctx)
	for i, name := range rf.argNames {
		atype := rf.args[i+1]

		text, ok := getArg(name)
		if !ok {
			vals[i+1] = reflect.Zero(atype)
			continue
		}

		val, err := parseArgValue(atype, text)
		if err != nil {
			return nil, fmt.Errorf("decoding parameter %q: %w", name, err)
		}
		vals[i+1] = val
	}
	return vals, nil
}

func parseArgValue(atype reflect.Type, text string) (reflect.Value, error) {
	// Regardless whether the argument is a pointer type, allocate a pointer so
	// we can set the computed value.
	var out reflect.Value
	isPtr := atype.Kind() == reflect.Ptr
	if isPtr {
		out = reflect.New(atype.Elem())
	} else {
		out = reflect.New(atype)
	}

	baseType := out.Type().Elem()
	if isIntType(baseType) {
		// Integral type: Require a base-10 digit string. For compatibility with
		// existing use allow quotation marks.
		v, err := decodeInteger(text)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("invalid integer: %w", err)
		}
		out.Elem().Set(reflect.ValueOf(v).Convert(baseType))

	} else if isStringOrBytes(baseType) {
		// String or byte slice: Check for quotes, hex encoding.
		dec, err := decodeString(text)
		if err != nil {
			return reflect.Value{}, err
		}
		out.Elem().Set(reflect.ValueOf(dec).Convert(baseType))

	} else if baseType.Kind() == reflect.Bool {
		b, err := strconv.ParseBool(text)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("invalid boolean: %w", err)
		}
		out.Elem().Set(reflect.ValueOf(b))

	} else if out.Type().Implements(textUnmarshalerType) {
		s, err := decodeString(text)
		if err != nil {
			return reflect.Value{}, err
		}
		v := reflect.New(baseType)
		dec := v.Interface().(encoding.TextUnmarshaler)
		if err := dec.UnmarshalText(s); err != nil {
			return reflect.Value{}, fmt.Errorf("invalid text: %w", err)
		}
		out.Elem().Set(v.Elem())

	} else {
		// We don't know how to represent other types.
		return reflect.Value{}, fmt.Errorf("unsupported argument type %v", baseType)
	}

	// If the argument wants a pointer, return the value as-is, otherwise
	// indirect the pointer back off.
	if isPtr {
		return out, nil
	}
	return out.Elem(), nil
}

var (
	uint64Type          = reflect.TypeOf(uint64(0))
	textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
)

// isIntType reports whether atype is an integer-shaped type.
func isIntType(atype reflect.Type) bool {
	switch atype.Kind() {
	case reflect.Float32, reflect.Float64:
		return false
	default:
		return atype.ConvertibleTo(uint64Type)
	}
}

// isStringOrBytes reports whether atype is a string or []byte.
func isStringOrBytes(atype reflect.Type) bool {
	switch atype.Kind() {
	case reflect.String:
		return true
	case reflect.Slice:
		return atype.Elem().Kind() == reflect.Uint8
	default:
		return false
	}
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

// decodeString decodes s into a byte slice. If s has an 0x prefix, it is
// treated as a hex-encoded string. If it is "double quoted" it is treated as a
// JSON string value. Otherwise, s is converted to bytes directly.
func decodeString(s string) ([]byte, error) {
	if lc := strings.ToLower(s); strings.HasPrefix(lc, "0x") {
		return hex.DecodeString(lc[2:])
	} else if isQuotedString(s) {
		var dec string
		if err := json.Unmarshal([]byte(s), &dec); err != nil {
			return nil, fmt.Errorf("invalid quoted string: %w", err)
		}
		return []byte(dec), nil
	}
	return []byte(s), nil
}
