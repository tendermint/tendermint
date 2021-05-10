package fakeserver

import (
	"context"
	"log"
	"net/http"
	"sync"
	"testing"
)

// Server is a mock http server
// the idea is to use this server in a test flow to check requests and passed response
type Server struct {
	t        *testing.T
	mux      *http.ServeMux
	addr     string
	guard    sync.Mutex
	handlers map[string]*handler
	httpSrv  *http.Server
}

// On returns a call structure to setup afterwards
func (s *Server) On(pattern string) *Call {
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
func (s *Server) Start() {
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
func (s *Server) Stop(ctx context.Context) {
	s.guard.Lock()
	defer s.guard.Unlock()
	if err := s.httpSrv.Shutdown(ctx); err != nil {
		log.Panic(err)
	}
}

// HTTPServer returns a mock http server that listens to all requests by "/" URL
func HTTPServer(t *testing.T, addr string) *Server {
	mux := http.NewServeMux()
	srv := &Server{
		t:        t,
		mux:      mux,
		addr:     addr,
		handlers: make(map[string]*handler),
	}
	return srv
}
