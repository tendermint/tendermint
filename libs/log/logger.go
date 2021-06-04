package log

import (
	"io"
	"sync"
)

const (
	// LogFormatPlain defines a logging format used for human-readable text-based
	// logging that is not structured. Typically, this format is used for development
	// and testing purposes.
	LogFormatPlain string = "plain"

	// LogFormatText defines a logging format used for human-readable text-based
	// logging that is not structured. Typically, this format is used for development
	// and testing purposes.
	LogFormatText string = "text"

	// LogFormatJSON defines a logging format for structured JSON-based logging
	// that is typically used in production environments, which can be sent to
	// logging facilities that support complex log parsing and querying.
	LogFormatJSON string = "json"

	// Supported loging levels
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

// Logger defines a generic logging interface compatible with Tendermint.
type Logger interface {
	Debug(msg string, keyVals ...interface{})
	Info(msg string, keyVals ...interface{})
	Error(msg string, keyVals ...interface{})

	With(keyVals ...interface{}) Logger
}

// syncWriter wraps an io.Writer that can be used in a Logger that is safe for
// concurrent use by multiple goroutines.
type syncWriter struct {
	sync.Mutex
	io.Writer
}

func newSyncWriter(w io.Writer) io.Writer {
	return &syncWriter{Writer: w}
}

// Write writes p to the underlying io.Writer. If another write is already in
// progress, the calling goroutine blocks until the syncWriter is available.
func (w *syncWriter) Write(p []byte) (int, error) {
	w.Lock()
	defer w.Unlock()
	return w.Writer.Write(p)
}
