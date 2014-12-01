// Commons for HTTP handling
package rpc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	"github.com/tendermint/tendermint/alert"
)

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
}

func (res APIResponse) Error() string {
	return fmt.Sprintf("Status(%v) %v", res.Status, res.Data)
}

// Throws a panic which the RecoverAndLogHandler catches.
func ReturnJSON(status APIStatus, data interface{}) {
	res := APIResponse{}
	res.Status = status
	res.Data = data
	panic(res)
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
		origin := r.Header.Get("Origin")
		originUrl, err := url.Parse(origin)
		if err == nil {
			originHost := strings.Split(originUrl.Host, ":")[0]
			if strings.HasSuffix(originHost, ".ftnox.com") {
				rww.Header().Set("Access-Control-Allow-Origin", origin)
				rww.Header().Set("Access-Control-Allow-Credentials", "true")
				rww.Header().Set("Access-Control-Expose-Headers", "X-Server-Time")
			}
		}
		rww.Header().Set("X-Server-Time", fmt.Sprintf("%v", begin.Unix()))

		defer func() {
			// Send a 500 error if a panic happens during a handler.
			// Without this, Chrome & Firefox were retrying aborted ajax requests,
			// at least to my localhost.
			if e := recover(); e != nil {

				// If APIResponse,
				if res, ok := e.(APIResponse); ok {
					resJSON, err := json.Marshal(res)
					if err != nil {
						panic(err)
					}
					rww.Header().Set("Content-Type", "application/json")
					switch res.Status {
					case API_OK:
						rww.WriteHeader(200)
					case API_ERROR:
						rww.WriteHeader(400)
					case API_UNAUTHORIZED:
						rww.WriteHeader(401)
					case API_INVALID_PARAM:
						rww.WriteHeader(420)
					case API_REDIRECT:
						rww.WriteHeader(430)
					default:
						rww.WriteHeader(440)
					}
					rww.Write(resJSON)
				} else {
					// For the rest,
					rww.WriteHeader(http.StatusInternalServerError)
					rww.Write([]byte("Internal Server Error"))
					log.Error("%s: %s", e, debug.Stack())
				}
			}

			// Finally, log.
			durationMS := time.Since(begin).Nanoseconds() / 1000000
			if rww.Status == -1 {
				rww.Status = 200
			}
			log.Debug("%s %s %v %v %s", r.RemoteAddr, r.Method, rww.Status, durationMS, r.URL)
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
