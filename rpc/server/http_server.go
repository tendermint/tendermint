// Commons for HTTP handling
package rpcserver

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/tendermint/tendermint/alert"
	. "github.com/tendermint/tendermint/common"
	. "github.com/tendermint/tendermint/rpc/types"
	"github.com/tendermint/tendermint/wire"
)

func StartHTTPServer(listenAddr string, handler http.Handler) (net.Listener, error) {
	log.Notice(Fmt("Starting RPC HTTP server on %v", listenAddr))
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("Failed to listen to %v", listenAddr)
	}
	go func() {
		res := http.Serve(
			listener,
			RecoverAndLogHandler(handler),
		)
		log.Crit("RPC HTTP server stopped", "result", res)
	}()
	return listener, nil
}

func WriteRPCResponse(w http.ResponseWriter, res RPCResponse) {
	buf, n, err := new(bytes.Buffer), int64(0), error(nil)
	wire.WriteJSON(res, buf, &n, &err)
	if err != nil {
		log.Error("Failed to write RPC response", "error", err, "res", res)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(buf.Bytes())
}

//-----------------------------------------------------------------------------

// Wraps an HTTP handler, adding error logging.
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
					log.Error("Panic in RPC HTTP handler", "error", e, "stack", string(debug.Stack()))
					rww.WriteHeader(http.StatusInternalServerError)
					WriteRPCResponse(rww, NewRPCResponse("", nil, Fmt("Internal Server Error: %v", e)))
				}
			}

			// Finally, log.
			durationMS := time.Since(begin).Nanoseconds() / 1000000
			if rww.Status == -1 {
				rww.Status = 200
			}
			log.Info("Served RPC HTTP response",
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
