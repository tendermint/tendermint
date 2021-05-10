package mockcoreserver

import (
	"io"
	"net/http"
	"sync"
)

type (
	ExpectedRequestFunc func(req *http.Request) error
	ResponseFunc        func(w http.ResponseWriter) error
	ResponseOptionFunc  func(opt *respOption)
)

type respOption struct {
	status int
	body   io.Reader
	header http.Header
}

// Call is a call expectation structure
type Call struct {
	respFunc    ResponseFunc
	expectFunc  ExpectedRequestFunc
	actualCnt   int
	expectedCnt int
	isStopped   bool
	guard       sync.Mutex
}

// Respond sets a response by a request
func (c *Call) Respond(opts ...ResponseOptionFunc) *Call {
	ro := &respOption{
		status: http.StatusOK,
		header: http.Header{},
	}
	for _, opt := range opts {
		opt(ro)
	}
	c.respFunc = func(w http.ResponseWriter) error {
		if len(ro.header) > 0 {
			err := ro.header.Write(w)
			if err != nil {
				return err
			}
		}
		if ro.body != nil {
			_, err := io.Copy(w, ro.body)
			if err != nil {
				return err
			}
		}
		return nil
	}
	return c
}

// Expect sets an expectation on a request
func (c *Call) Expect(fn ExpectedRequestFunc) *Call {
	c.expectFunc = fn
	return c
}

// Times ...
func (c *Call) Times(cnt int) *Call {
	c.expectedCnt = cnt
	return c
}

// Once ...
func (c *Call) Once() *Call {
	c.Times(1)
	return c
}

func (c *Call) execute(w http.ResponseWriter, req *http.Request) error {
	if c.expectFunc != nil {
		err := c.expectFunc(req)
		if err != nil {
			return err
		}
	}
	if c.respFunc != nil {
		err := c.respFunc(w)
		if err != nil {
			return err
		}
	}
	c.actualCnt++
	return nil
}
