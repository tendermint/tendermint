package mockcoreserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"

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
	if err != nil && err != http.ErrServerClosed {
		log.Fatalf("unexpected stop a server: %v", err)
	}
}

// Stop stops http server
func (s *HTTPServer) Stop(ctx context.Context) {
	s.guard.Lock()
	defer s.guard.Unlock()
	if err := s.httpSrv.Shutdown(ctx); err != nil {
		log.Fatalf("unable to stop a server graceful: %v", err)
	}
}

// NewHTTPServer returns a mock http server
func NewHTTPServer(addr string) *HTTPServer {
	mux := http.NewServeMux()
	srv := &HTTPServer{
		mux:      mux,
		addr:     addr,
		handlers: make(map[string]*handler),
	}
	return srv
}

// JRPCServer is a mock JRPC server implementation
type JRPCServer struct {
	httpSrv     *HTTPServer
	guard       sync.Mutex
	endpointURL string
	calls       map[string][]*Call
}

// NewJRPCServer creates and returns a new mock of JRPC server
func NewJRPCServer(addr, endpointURL string) *JRPCServer {
	return &JRPCServer{
		httpSrv:     NewHTTPServer(addr),
		endpointURL: endpointURL,
		calls:       make(map[string][]*Call),
	}
}

// Start starts listening and handling JRPC requests
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
			return fmt.Errorf("unable to decode jRPC request: %v", err)
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
		// put unmarshalled JRPC request into a context
		ctx = context.WithValue(req.Context(), jRPCRequestKey, jReq)
		return call.execute(w, req.WithContext(ctx))
	}
	s.httpSrv.Start()
}

func (s *JRPCServer) findCall(req btcjson.Request) (*Call, error) {
	name, err := callName(req)
	if err != nil {
		return nil, err
	}
	calls, ok := s.calls[name]
	if !ok {
		method := req.Method
		if len(req.Params) > 0 {
			method = method + " " + string(req.Params[0])
		}
		return nil, fmt.Errorf("the expectation for a method %q was not registered", method)
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

func callName(req btcjson.Request) (string, error) {
	name := req.Method
	if len(req.Params) == 0 {
		return name, nil
	}
	s := ""
	err := json.Unmarshal(req.Params[0], &s)
	if err != nil {
		return "", err
	}
	return name + " " + s, nil
}
