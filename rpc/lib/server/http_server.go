// Commons for HTTP handling
package rpcserver

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/pkg/errors"

	types "github.com/tendermint/tendermint/rpc/lib/types"
	"github.com/tendermint/tmlibs/log"
)

func StartHTTPServer(listenAddr string, handler http.Handler, logger log.Logger) (listener net.Listener, err error) {
	var proto, addr string
	parts := strings.SplitN(listenAddr, "://", 2)
	if len(parts) != 2 {
		return nil, errors.Errorf("Invalid listening address %s (use fully formed addresses, including the tcp:// or unix:// prefix)", listenAddr)
	}
	proto, addr = parts[0], parts[1]

	logger.Info(fmt.Sprintf("Starting RPC HTTP server on %s", listenAddr))
	listener, err = net.Listen(proto, addr)
	if err != nil {
		return nil, errors.Errorf("Failed to listen on %v: %v", listenAddr, err)
	}

	go func() {
		err := http.Serve(
			listener,
			RecoverAndLogHandler(handler, logger),
		)
		logger.Error("RPC HTTP server stopped", "err", err)
	}()
	return listener, nil
}

func StartHTTPAndTLSServer(listenAddr string, handler http.Handler, certFile, keyFile string, logger log.Logger) (listener net.Listener, err error) {
	var proto, addr string
	parts := strings.SplitN(listenAddr, "://", 2)
	if len(parts) != 2 {
		return nil, errors.Errorf("Invalid listening address %s (use fully formed addresses, including the tcp:// or unix:// prefix)", listenAddr)
	}
	proto, addr = parts[0], parts[1]

	logger.Info(fmt.Sprintf("Starting RPC HTTPS server on %s (cert: %q, key: %q)", listenAddr, certFile, keyFile))
	listener, err = net.Listen(proto, addr)
	if err != nil {
		return nil, errors.Errorf("Failed to listen on %v: %v", listenAddr, err)
	}

	go func() {
		err := http.ServeTLS(
			listener,
			RecoverAndLogHandler(handler, logger),
			certFile,
			keyFile,
		)
		logger.Error("RPC HTTPS server stopped", "err", err)
	}()
	return listener, nil
}

func WriteRPCResponseHTTPError(w http.ResponseWriter, httpCode int, res types.RPCResponse) {
	jsonBytes, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		panic(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpCode)
	w.Write(jsonBytes) // nolint: errcheck, gas
}

func WriteRPCResponseHTTP(w http.ResponseWriter, res types.RPCResponse) {
	jsonBytes, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		panic(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(jsonBytes) // nolint: errcheck, gas
}

//-----------------------------------------------------------------------------

// Wraps an HTTP handler, adding error logging.
// If the inner function panics, the outer function recovers, logs, sends an
// HTTP 500 error response.
func RecoverAndLogHandler(handler http.Handler, logger log.Logger) http.Handler {
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
				if res, ok := e.(types.RPCResponse); ok {
					WriteRPCResponseHTTP(rww, res)
				} else {
					// For the rest,
					logger.Error("Panic in RPC HTTP handler", "err", e, "stack", string(debug.Stack()))
					rww.WriteHeader(http.StatusInternalServerError)
					WriteRPCResponseHTTP(rww, types.RPCInternalError("", e.(error)))
				}
			}

			// Finally, log.
			durationMS := time.Since(begin).Nanoseconds() / 1000000
			if rww.Status == -1 {
				rww.Status = 200
			}
			logger.Info("Served RPC HTTP response",
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
