// Commons for HTTP handling
package rpc

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/tendermint/tendermint/alert"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/events"
)

func StartHTTPServer(listenAddr string, funcMap map[string]*RPCFunc, evsw *events.EventSwitch) {
	log.Info(Fmt("Starting RPC HTTP server on %s", listenAddr))
	mux := http.NewServeMux()
	RegisterRPCFuncs(mux, funcMap)
	if evsw != nil {
		RegisterEventsHandler(mux, evsw)
	}
	go func() {
		res := http.ListenAndServe(
			listenAddr,
			RecoverAndLogHandler(mux),
		)
		log.Crit("RPC HTTPServer stopped", "result", res)
	}()
}

func WriteRPCResponse(w http.ResponseWriter, res RPCResponse) {
	buf, n, err := new(bytes.Buffer), new(int64), new(error)
	binary.WriteJSON(res, buf, n, err)
	if *err != nil {
		log.Warn("Failed to write JSON RPCResponse", "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(buf.Bytes())
}

//-----------------------------------------------------------------------------

// Wraps an HTTP handler, adding error logging.
//
// If the inner function panics, the outer function recovers, logs, sends an
// HTTP 500 error response.
func RecoverAndLogHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wrap the ResponseWriter to remember the status
		rww := &ResponseWriterWrapper{-1, w}
		begin := time.Now()

		// Common headers
		origin := r.Header.Get("Origin")
		rww.Header().Set("Access-Control-Allow-Origin", origin)
		rww.Header().Set("Access-Control-Allow-Credentials", "true")
		rww.Header().Set("Access-Control-Expose-Headers", "X-Server-Time")
		rww.Header().Set("X-Server-Time", fmt.Sprintf("%v", begin.Unix()))

		defer func() {
			// Send a 500 error if a panic happens during a handler.
			// Without this, Chrome & Firefox were retrying aborted ajax requests,
			// at least to my localhost.
			if e := recover(); e != nil {

				// If RPCResponse
				if res, ok := e.(RPCResponse); ok {
					WriteRPCResponse(rww, res)
				} else {
					// For the rest,
					log.Error("Panic in HTTP handler", "error", e, "stack", string(debug.Stack()))
					rww.WriteHeader(http.StatusInternalServerError)
					WriteRPCResponse(rww, NewRPCResponse(nil, Fmt("Internal Server Error: %v", e)))
				}
			}

			// Finally, log.
			durationMS := time.Since(begin).Nanoseconds() / 1000000
			if rww.Status == -1 {
				rww.Status = 200
			}
			log.Debug("Served HTTP response",
				"method", r.Method, "url", r.URL,
				"status", rww.Status, "duration", durationMS,
				"remoteAddr", r.RemoteAddr,
			)
		}()

		handler.ServeHTTP(rww, r)
	})
}

// Remember the status for logging
type ResponseWriterWrapper struct {
	Status int
	http.ResponseWriter
}

func (w *ResponseWriterWrapper) WriteHeader(status int) {
	w.Status = status
	w.ResponseWriter.WriteHeader(status)
}

// implements http.Hijacker
func (w *ResponseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.ResponseWriter.(http.Hijacker).Hijack()
}

// Stick it as a deferred statement in gouroutines to prevent the program from crashing.
func Recover(daemonName string) {
	if e := recover(); e != nil {
		stack := string(debug.Stack())
		errorString := fmt.Sprintf("[%s] %s\n%s", daemonName, e, stack)
		alert.Alert(errorString)
	}
}
