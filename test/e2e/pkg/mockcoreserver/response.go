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
		log.Panicf("unable to encode: %v", err)
	}
}

// Body ...
func Body(body []byte) HandlerOptionFunc {
	return func(opts *respOption, _ *http.Request) {
		opts.body = bytes.NewBuffer(body)
	}
}

// JsonBody ...
func JsonBody(v interface{}) HandlerOptionFunc {
	return Body(mustMarshal(v))
}

// JRPCResult ..
func JRPCResult(v interface{}) HandlerOptionFunc {
	return JsonBody(&response{Result: mustMarshal(v)})
}

// Header ...
func Header(key string, values ...string) HandlerOptionFunc {
	return func(opts *respOption, _ *http.Request) {
		opts.header[key] = values
	}
}

// ContentType ...
func ContentType(val string) HandlerOptionFunc {
	return Header("content-type", val)
}

// JsonContentType ...
func JsonContentType() HandlerOptionFunc {
	return ContentType("application/json")
}

// OnMethod ...
func OnMethod(fn func(req btcjson.Request) (interface{}, error)) HandlerOptionFunc {
	return func(opt *respOption, req *http.Request) {
		_ = JRPCRequest(func(btcReq btcjson.Request) error {
			val, err := fn(btcReq)
			if err != nil {
				return err
			}
			JRPCResult(val)(opt, req)
			return nil
		})(req)
	}
}
