package mockcoreserver

import (
	"bytes"
	"io"
	"net/http"
	// "sync"
)

var (
	jsonEmptyString = mustMarshal("")
)

type (
	ExpectFunc        func(req *http.Request) error
	HandlerFunc       func(w http.ResponseWriter, req *http.Request) error
	HandlerOptionFunc func(opt *respOption, req *http.Request) error
)

type respOption struct {
	status      int
	body        io.Reader
	header      map[string][]string
	// handlerFunc func(w http.ResponseWriter, req *http.Request)
}

// Call is a call expectation structure
type Call struct {
	handlerFunc HandlerFunc
	expectFunc  ExpectFunc
	actualCnt   int
	expectedCnt int
	// guard       sync.Mutex
}

// Respond sets a response by a request
func (c *Call) Respond(opts ...HandlerOptionFunc) *Call {
	ro := &respOption{
		status: http.StatusOK,
		body:   bytes.NewBuffer(jsonEmptyString),
		header: make(map[string][]string),
	}
	c.handlerFunc = func(w http.ResponseWriter, req *http.Request) error {
		for _, opt := range opts {
			err := opt(ro, req)
			if err != nil {
				return err
			}
		}
		if len(ro.header) > 0 {
			for k, vals := range ro.header {
				if len(vals) == 1 {
					w.Header().Set(k, vals[0])
				} else {
					for _, v := range vals {
						w.Header().Add(k, v)
					}
				}
			}
			w.WriteHeader(ro.status)
		}
		_, err := io.Copy(w, ro.body)
		if err != nil {
			return err
		}
		return nil
	}
	return c
}

// Expect sets an expectation on a request
func (c *Call) Expect(fn ExpectFunc) *Call {
	c.expectFunc = fn
	return c
}

// Times sets the expected number of calls
func (c *Call) Times(cnt int) *Call {
	c.expectedCnt = cnt
	return c
}

// Once sets only one expected call
func (c *Call) Once() *Call {
	c.Times(1)
	return c
}

// Forever do not use number expected calls for a mock
func (c *Call) Forever() *Call {
	c.Times(-1)
	return c
}

func (c *Call) execute(w http.ResponseWriter, req *http.Request) error {
	if c.expectFunc != nil {
		err := c.expectFunc(req)
		if err != nil {
			return err
		}
	}
	if c.handlerFunc != nil {
		err := c.handlerFunc(w, req)
		if err != nil {
			return err
		}
	}
	c.actualCnt++
	return nil
}
