// Commons for HTTP handling
package rpc

import (
	"bytes"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/tendermint/tendermint/alert"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/config"
)

func StartHTTPServer() {
	initHandlers()

	log.Info(Fmt("Starting RPC HTTP server on %s", config.App().GetString("RPC.HTTP.ListenAddr")))
	go func() {
		log.Crit("RPC HTTPServer stopped", "result", http.ListenAndServe(config.App().GetString("RPC.HTTP.ListenAddr"), RecoverAndLogHandler(http.DefaultServeMux)))
	}()
}

//-----------------------------------------------------------------------------

type APIStatus string

const (
	API_OK            APIStatus = "OK"
	API_ERROR         APIStatus = "ERROR"
	API_INVALID_PARAM APIStatus = "INVALID_PARAM"
	API_UNAUTHORIZED  APIStatus = "UNAUTHORIZED"
	API_REDIRECT      APIStatus = "REDIRECT"
)

type APIResponse struct {
	Status APIStatus   `json:"status"`
	Data   interface{} `json:"data"`
	Error  string      `json:"error"`
}

func (res APIResponse) StatusError() string {
	return fmt.Sprintf("Status(%v) %v", res.Status, res.Error)
}

func WriteAPIResponse(w http.ResponseWriter, status APIStatus, data interface{}, responseErr string) {
	res := APIResponse{}
	res.Status = status
	if data == nil {
		// so json doesn't vommit
		data = struct{}{}
	}
	res.Data = data
	res.Error = responseErr

	buf, n, err := new(bytes.Buffer), new(int64), new(error)
	binary.WriteJSON(res, buf, n, err)
	if *err != nil {
		log.Warn("Failed to write JSON APIResponse", "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	/* Bad idea: (e.g. hard to use with jQuery)
	switch res.Status {
	case API_OK:
		w.WriteHeader(200)
	case API_ERROR:
		w.WriteHeader(400)
	case API_UNAUTHORIZED:
		w.WriteHeader(401)
	case API_INVALID_PARAM:
		w.WriteHeader(420)
	case API_REDIRECT:
		w.WriteHeader(430)
	default:
		w.WriteHeader(440)
	}*/
	w.Write(buf.Bytes())
}

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
		rww.Header().Set("Access-Control-Allow-Origin", "*")
		/*
			origin := r.Header.Get("Origin")
			originUrl, err := url.Parse(origin)
			if err == nil {
				originHost := strings.Split(originUrl.Host, ":")[0]
				if strings.HasSuffix(originHost, ".tendermint.com") {
					rww.Header().Set("Access-Control-Allow-Origin", origin)
					rww.Header().Set("Access-Control-Allow-Credentials", "true")
					rww.Header().Set("Access-Control-Expose-Headers", "X-Server-Time")
				}
			}
		*/
		rww.Header().Set("X-Server-Time", fmt.Sprintf("%v", begin.Unix()))

		defer func() {
			// Send a 500 error if a panic happens during a handler.
			// Without this, Chrome & Firefox were retrying aborted ajax requests,
			// at least to my localhost.
			if e := recover(); e != nil {

				// If APIResponse,
				if res, ok := e.(APIResponse); ok {
					WriteAPIResponse(rww, res.Status, nil, res.Error)
				} else {
					// For the rest,
					rww.WriteHeader(http.StatusInternalServerError)
					rww.Write([]byte("Internal Server Error"))
					log.Error("Panic in HTTP handler", "error", e, "stack", string(debug.Stack()))
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

// Stick it as a deferred statement in gouroutines to prevent the program from crashing.
func Recover(daemonName string) {
	if e := recover(); e != nil {
		stack := string(debug.Stack())
		errorString := fmt.Sprintf("[%s] %s\n%s", daemonName, e, stack)
		alert.Alert(errorString)
	}
}
