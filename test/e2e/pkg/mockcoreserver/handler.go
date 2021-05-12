package mockcoreserver

import (
	"log"
	"net/http"
	"sync"
	"testing"
)

type handler struct {
	pattern string
	calls   []*Call
	guard   sync.Mutex
	t       *testing.T
}

// ServeHTTP ...
func (h *handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	h.guard.Lock()
	defer h.guard.Unlock()
	c := h.findCall()
	if c == nil {
		log.Panic("call not found")
	}
	err := c.execute(w, req)
	if err != nil {
		h.t.Fatalf("URL %s: %s", req.URL.String(), err.Error())
	}
	c.actualCnt++
}

func (h *handler) findCall() *Call {
	for _, c := range h.calls {
		if c.expectedCnt == -1 || c.expectedCnt >= c.actualCnt {
			return c
		}
	}
	return nil
}
