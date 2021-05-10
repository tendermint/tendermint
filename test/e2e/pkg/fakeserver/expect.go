package fakeserver

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
)

// BodyShouldBeSame compares a body from received request and passed byte slice
func BodyShouldBeSame(v interface{}) ExpectedRequestFunc {
	var body []byte
	switch t := v.(type) {
	case []byte:
		body = t
	case string:
		body = []byte(t)
	default:
		log.Panicf("unsupported type %s", t)
	}
	return func(req *http.Request) error {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		buf, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return err
		}
		if bytes.Compare(body, buf) != 0 {
			return fmt.Errorf("the request body retried by URL %s is not equal\nexpected: %s\nactual: %s", req.URL.String(), buf, body)
		}
		return nil
	}
}

// BodyShouldBeEmpty ...
func BodyShouldBeEmpty() ExpectedRequestFunc {
	return BodyShouldBeSame("")
}

// QueryShouldHave ...
func QueryShouldHave(expectedVales url.Values) ExpectedRequestFunc {
	return func(req *http.Request) error {
		actuallyVales := req.URL.Query()
		for k, eVals := range expectedVales {
			aVals, ok := actuallyVales[k]
			if !ok {
				return fmt.Errorf("query parameter %q not found in a request", k)
			}
			for i, ev := range eVals {
				if aVals[i] != ev {
					return fmt.Errorf("query parameter %q should be equal to %q", aVals[i], ev)
				}
			}
		}
		return nil
	}
}

// And ...
func And(fns ...ExpectedRequestFunc) ExpectedRequestFunc {
	return func(req *http.Request) error {
		for _, fn := range fns {
			err := fn(req)
			if err != nil {
				return err
			}
		}
		return nil
	}
}
