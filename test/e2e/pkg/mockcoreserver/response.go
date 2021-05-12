package mockcoreserver

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
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

// Body ...
func Body(body []byte) HandlerOptionFunc {
	return func(opts *respOption) {
		resp := &response{Result: body}
		opts.body = bytes.NewBuffer(mustMarshal(resp))
	}
}

// Header ...
func Header(key string, values ...string) HandlerOptionFunc {
	return func(opts *respOption) {
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

// NotContent ...
func NotContent() HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) error {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
}

// BadRequest ...
func BadRequest() HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) error {
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}
}
