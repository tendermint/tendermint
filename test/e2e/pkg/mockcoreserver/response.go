package mockcoreserver

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"

	"github.com/dashevo/dashd-go/btcjson"
)

type response struct {
	Result json.RawMessage `json:"result"`
}

func mustMarshal(v interface{}) []byte {
	j, err := json.Marshal(v)
	if err != nil {
		log.Panicf("unable to encode: %v", err)
	}
	return j
}

func mustUnmarshal(data []byte, out interface{}) {
	err := json.Unmarshal(data, out)
	if err != nil {
		log.Panicf("unable to decode: %v", err)
	}
}

// Body ...
func Body(body []byte) HandlerOptionFunc {
	return func(opts *respOption, _ *http.Request) error {
		opts.body = bytes.NewBuffer(body)
		return nil
	}
}

// JSONBody ...
func JSONBody(v interface{}) HandlerOptionFunc {
	return Body(mustMarshal(v))
}

// JRPCResult ..
func JRPCResult(v interface{}) HandlerOptionFunc {
	return JSONBody(&response{Result: mustMarshal(v)})
}

// Header ...
func Header(key string, values ...string) HandlerOptionFunc {
	return func(opts *respOption, _ *http.Request) error {
		opts.header[key] = values
		return nil
	}
}

// ContentType ...
func ContentType(val string) HandlerOptionFunc {
	return Header("content-type", val)
}

// JSONContentType ...
func JSONContentType() HandlerOptionFunc {
	return ContentType("application/json")
}

// OnMethod ...
func OnMethod(fn func(req btcjson.Request) (interface{}, error)) HandlerOptionFunc {
	return func(opt *respOption, req *http.Request) error {
		return JRPCRequest(func(btcReq btcjson.Request) error {
			val, err := fn(btcReq)
			if err != nil {
				return err
			}
			return JRPCResult(val)(opt, req)
		})(req)
	}
}
