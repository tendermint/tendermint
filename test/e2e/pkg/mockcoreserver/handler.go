package mockcoreserver

import (
	"log"
	"net/http"
	"sync"
)

type handler struct {
	pattern string
	calls   []*Call
	guard   sync.Mutex
}

// ServeHTTP is an entrypoint of a server request
func (h *handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	h.guard.Lock()
	c := h.findCall()
	if c == nil {
		h.guard.Unlock()
		log.Fatal("call not found")
	}
	err := c.execute(w, req)
	if err != nil {
		h.guard.Unlock()
		log.Fatalf("URL %s: %s", req.URL.String(), err.Error())
	}
	c.actualCnt++
	h.guard.Unlock()
}

func (h *handler) findCall() *Call {
	for _, c := range h.calls {
		if c.expectedCnt == -1 || c.expectedCnt >= c.actualCnt {
			return c
		}
	}
	return nil
}
