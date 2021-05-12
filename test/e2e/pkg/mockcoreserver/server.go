package mockcoreserver

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"testing"

	"github.com/dashevo/dashd-go/btcjson"
)

var jRPCRequestKey = struct{}{}

// MockServer ...
type MockServer interface {
	Start()
	Stop(ctx context.Context)
	On(pattern string) *Call
}

// HTTPServer is a mock http server
// the idea is to use this server in a test flow to check requests and passed response
type HTTPServer struct {
	t        *testing.T
	mux      *http.ServeMux
	addr     string
	guard    sync.Mutex
	handlers map[string]*handler
	httpSrv  *http.Server
}

// On returns a call structure to setup afterwards
func (s *HTTPServer) On(pattern string) *Call {
	s.guard.Lock()
	defer s.guard.Unlock()
	h, ok := s.handlers[pattern]
	if !ok {
		h = &handler{
			t:       s.t,
			pattern: pattern,
		}
		s.handlers[pattern] = h
		// register http handler
		s.mux.Handle(pattern, h)
	}
	c := &Call{}
	h.calls = append(h.calls, c)
	return c
}

// Start listens and serves http requests
func (s *HTTPServer) Start() {
	s.guard.Lock()
	s.httpSrv = &http.Server{
		Addr:    s.addr,
		Handler: s.mux,
	}
	s.guard.Unlock()
	err := s.httpSrv.ListenAndServe()
	if err != http.ErrServerClosed {
		log.Panic(err)
	}
}

// Stop stops http server
func (s *HTTPServer) Stop(ctx context.Context) {
	s.guard.Lock()
	defer s.guard.Unlock()
	if err := s.httpSrv.Shutdown(ctx); err != nil {
		log.Panic(err)
	}
}

// NewHTTPServer returns a mock http server
func NewHTTPServer(t *testing.T, addr string) *HTTPServer {
	mux := http.NewServeMux()
	srv := &HTTPServer{
		t:        t,
		mux:      mux,
		addr:     addr,
		handlers: make(map[string]*handler),
	}
	return srv
}

// JRPCServer ...
type JRPCServer struct {
	t           *testing.T
	httpSrv     *HTTPServer
	guard       sync.Mutex
	endpointURL string
	calls       map[string][]*Call
}

// NewJRPCServer ...
func NewJRPCServer(t *testing.T, addr, endpointURL string) *JRPCServer {
	return &JRPCServer{
		t:           t,
		httpSrv:     NewHTTPServer(t, addr),
		endpointURL: endpointURL,
		calls:       make(map[string][]*Call),
	}
}

// Start ...
func (s *JRPCServer) Start() {
	httpCall := s.httpSrv.On(s.endpointURL)
	httpCall.Forever()
	httpCall.handlerFunc = func(w http.ResponseWriter, req *http.Request) error {
		ctx := context.Background()
		s.guard.Lock()
		defer s.guard.Unlock()
		jReq := btcjson.Request{}
		buf, err := ioutil.ReadAll(req.Body)
		if err != nil {
			s.t.Fatalf("unable to decode jRPC request: %v", err)
		}
		err = req.Body.Close()
		if err != nil {
			return err
		}
		req.Body = ioutil.NopCloser(bytes.NewBuffer(buf))
		mustUnmarshal(buf, &jReq)
		call, err := s.findCall(jReq)
		if err != nil {
			return err
		}
		ctx = context.WithValue(req.Context(), jRPCRequestKey, jReq)
		return call.execute(w, req.WithContext(ctx))
	}
	s.httpSrv.Start()
}

func (s *JRPCServer) findCall(req btcjson.Request) (*Call, error) {
	calls, ok := s.calls[req.Method]
	if !ok {
		s.t.Fatalf("the expectation for a method %s was not registered", req.Method)
	}
	for _, call := range calls {
		if call.expectedCnt == -1 || call.actualCnt < call.expectedCnt {
			return call, nil
		}
	}
	return nil, fmt.Errorf("unable to find a call fro a method %s", req.Method)
}

// Stop ...
func (s *JRPCServer) Stop(ctx context.Context) {
	s.guard.Lock()
	defer s.guard.Unlock()
	s.httpSrv.Stop(ctx)
}

// On ...
func (s *JRPCServer) On(pattern string) *Call {
	s.guard.Lock()
	defer s.guard.Unlock()
	call := &Call{}
	s.calls[pattern] = append(s.calls[pattern], call)
	return call
}
