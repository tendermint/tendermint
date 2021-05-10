package fakeserver

import (
	"bytes"
	"net/http"
)

// Body ...
func Body(body []byte) ResponseOptionFunc {
	return func(opts *respOption) {
		opts.body = bytes.NewBuffer(body)
	}
}

// Header ...
func Header(key string, values ...string) ResponseOptionFunc {
	return func(opts *respOption) {
		for _, v := range values {
			opts.header.Add(key, v)
		}
	}
}

// ContentType ...
func ContentType(val string) ResponseOptionFunc {
	return Header("content-type", val)
}

// JsonContentType ...
func JsonContentType() ResponseOptionFunc {
	return ContentType("application/json")
}

// EmptyResponse ...
func NotContent() ResponseFunc {
	return func(w http.ResponseWriter) error {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
}

// BadRequest ...
func BadRequest() ResponseFunc {
	return func(w http.ResponseWriter) error {
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}
}
