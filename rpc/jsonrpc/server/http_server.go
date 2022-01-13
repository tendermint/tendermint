// Commons for HTTP handling
package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"golang.org/x/net/netutil"

	"github.com/tendermint/tendermint/libs/log"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

// Config is a RPC server configuration.
type Config struct {
	// see netutil.LimitListener
	MaxOpenConnections int
	// mirrors http.Server#ReadTimeout
	ReadTimeout time.Duration
	// mirrors http.Server#WriteTimeout
	WriteTimeout time.Duration
	// MaxBodyBytes controls the maximum number of bytes the
	// server will read parsing the request body.
	MaxBodyBytes int64
	// mirrors http.Server#MaxHeaderBytes
	MaxHeaderBytes int
}

// DefaultConfig returns a default configuration.
func DefaultConfig() *Config {
	return &Config{
		MaxOpenConnections: 0, // unlimited
		ReadTimeout:        10 * time.Second,
		WriteTimeout:       10 * time.Second,
		MaxBodyBytes:       int64(1000000), // 1MB
		MaxHeaderBytes:     1 << 20,        // same as the net/http default
	}
}

// Serve creates a http.Server and calls Serve with the given listener. It
// wraps handler with RecoverAndLogHandler and a handler, which limits the max
// body size to config.MaxBodyBytes.
//
// NOTE: This function blocks - you may want to call it in a go-routine.
func Serve(
	ctx context.Context,
	listener net.Listener,
	handler http.Handler,
	logger log.Logger,
	config *Config,
) error {
	logger.Info(fmt.Sprintf("Starting RPC HTTP server on %s", listener.Addr()))
	s := &http.Server{
		Handler:        RecoverAndLogHandler(maxBytesHandler{h: handler, n: config.MaxBodyBytes}, logger),
		ReadTimeout:    config.ReadTimeout,
		WriteTimeout:   config.WriteTimeout,
		MaxHeaderBytes: config.MaxHeaderBytes,
	}
	sig := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			sctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = s.Shutdown(sctx)
		case <-sig:
		}
	}()

	if err := s.Serve(listener); err != nil {
		logger.Info("RPC HTTP server stopped", "err", err)
		close(sig)
		return err
	}
	return nil
}

// Serve creates a http.Server and calls ServeTLS with the given listener,
// certFile and keyFile. It wraps handler with RecoverAndLogHandler and a
// handler, which limits the max body size to config.MaxBodyBytes.
//
// NOTE: This function blocks - you may want to call it in a go-routine.
func ServeTLS(
	ctx context.Context,
	listener net.Listener,
	handler http.Handler,
	certFile, keyFile string,
	logger log.Logger,
	config *Config,
) error {
	logger.Info(fmt.Sprintf("Starting RPC HTTPS server on %s (cert: %q, key: %q)",
		listener.Addr(), certFile, keyFile))
	s := &http.Server{
		Handler:        RecoverAndLogHandler(maxBytesHandler{h: handler, n: config.MaxBodyBytes}, logger),
		ReadTimeout:    config.ReadTimeout,
		WriteTimeout:   config.WriteTimeout,
		MaxHeaderBytes: config.MaxHeaderBytes,
	}
	sig := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			sctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = s.Shutdown(sctx)
		case <-sig:
		}
	}()

	if err := s.ServeTLS(listener, certFile, keyFile); err != nil {
		logger.Error("RPC HTTPS server stopped", "err", err)
		close(sig)
		return err
	}
	return nil
}

// writeInternalError writes an internal server error (500) to w with the text
// of err in the body. This is a fallback used when a handler is unable to
// write the expected response.
func writeInternalError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintln(w, err.Error())
}

// writeHTTPResponse writes a JSON-RPC response to w. If rsp encodes an error,
// the response body is its error object; otherwise its responses is the result.
//
// Unless there is an error encoding the response, the status is 200 OK.
func writeHTTPResponse(w http.ResponseWriter, log log.Logger, rsp rpctypes.RPCResponse) {
	var body []byte
	var err error
	if rsp.Error != nil {
		body, err = json.Marshal(rsp.Error)
	} else {
		body = rsp.Result
	}
	if err != nil {
		log.Error("Error encoding RPC response: %w", err)
		writeInternalError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// writeRPCResponse writes one or more JSON-RPC responses to w. A single
// response is encoded as an object, otherwise the response is sent as a batch
// (array) of response objects.
//
// Unless there is an error encoding the responses, the status is 200 OK.
func writeRPCResponse(w http.ResponseWriter, log log.Logger, rsps ...rpctypes.RPCResponse) {
	var body []byte
	var err error
	if len(rsps) == 1 {
		body, err = json.Marshal(rsps[0])
	} else {
		body, err = json.Marshal(rsps)
	}
	if err != nil {
		log.Error("Error encoding RPC response: %w", err)
		writeInternalError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

//-----------------------------------------------------------------------------

// RecoverAndLogHandler wraps an HTTP handler, adding error logging.  If the
// inner handler panics, the wrapper recovers, logs, sends an HTTP 500 error
// response to the client.
func RecoverAndLogHandler(handler http.Handler, logger log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wrap the ResponseWriter to remember the status
		rww := &responseWriterWrapper{-1, w}

		// Recover panics from inside handler and try to send the client
		// 500 Internal server error. If the handler panicked after already
		// sending a (partial) response, this is a no-op.
		defer func() {
			if v := recover(); v != nil {
				var err error
				switch e := v.(type) {
				case error:
					err = e
				case string:
					err = errors.New(e)
				case fmt.Stringer:
					err = errors.New(e.String())
				default:
					err = fmt.Errorf("panic with value %v", v)
				}

				logger.Error("Panic in RPC HTTP handler",
					"err", err, "stack", string(debug.Stack()))
				writeInternalError(rww, err)
			}
		}()

		// Log timing and response information from the handler.
		begin := time.Now()
		defer func() {
			elapsed := time.Since(begin)
			logger.Debug("served RPC HTTP response",
				"method", r.Method,
				"url", r.URL,
				"status", rww.Status,
				"duration", int(elapsed/time.Millisecond),
				"remoteAddr", r.RemoteAddr,
			)
		}()

		rww.Header().Set("X-Server-Time", fmt.Sprintf("%v", begin.Unix()))
		handler.ServeHTTP(rww, r)
	})
}

// Remember the status for logging
type responseWriterWrapper struct {
	Status int
	http.ResponseWriter
}

func (w *responseWriterWrapper) WriteHeader(status int) {
	w.Status = status
	w.ResponseWriter.WriteHeader(status)
}

// implements http.Hijacker
func (w *responseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.ResponseWriter.(http.Hijacker).Hijack()
}

type maxBytesHandler struct {
	h http.Handler
	n int64
}

func (h maxBytesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.n)
	h.h.ServeHTTP(w, r)
}

// Listen starts a new net.Listener on the given address.
// It returns an error if the address is invalid or the call to Listen() fails.
func Listen(addr string, maxOpenConnections int) (listener net.Listener, err error) {
	parts := strings.SplitN(addr, "://", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf(
			"invalid listening address %s (use fully formed addresses, including the tcp:// or unix:// prefix)",
			addr,
		)
	}
	proto, addr := parts[0], parts[1]
	listener, err = net.Listen(proto, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %v: %v", addr, err)
	}
	if maxOpenConnections > 0 {
		listener = netutil.LimitListener(listener, maxOpenConnections)
	}

	return listener, nil
}
