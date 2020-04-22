package log

import (
	"fmt"
	"io"
	stdlog "log"
	"strings"
)

type stdlibLogger struct {
	next    *stdlog.Logger
	keyvals []interface{}
}

// Interface assertions
var _ Logger = (*stdlibLogger)(nil)

// NewStdLibLogger returns a logger that encodes msg and keyvals to the Writer
// using standard library log as an underlying logger.
func NewStdLibLogger(w io.Writer, prefix string, flag int) Logger {
	return &stdlibLogger{
		next:    stdlog.New(w, prefix, flag),
		keyvals: make([]interface{}, 0),
	}
}

// Info logs a message at level Info.
func (l *stdlibLogger) Info(msg string, keyvals ...interface{}) {
	l.logf("info: ", msg, keyvals...)
}

// Debug logs a message at level Debug.
func (l *stdlibLogger) Debug(msg string, keyvals ...interface{}) {
	l.logf("debug: ", msg, keyvals...)
}

// Error logs a message at level Error.
func (l *stdlibLogger) Error(msg string, keyvals ...interface{}) {
	l.logf("error: ", msg, keyvals...)
}

// With returns a new contextual logger with keyvals prepended to those passed
// to calls to Info, Debug or Error.
func (l *stdlibLogger) With(keyvals ...interface{}) Logger {
	return &stdlibLogger{
		next:    l.next,
		keyvals: append(keyvals, l.keyvals...),
	}
}

func (l *stdlibLogger) logf(prefix, msg string, keyvals ...interface{}) {
	// join keyvals
	var b strings.Builder
	for i := 0; i < len(l.keyvals); i += 2 {
		fmt.Fprintf(&b, " %v=%v", l.keyvals[i], l.keyvals[i+1])
	}
	for i := 0; i < len(keyvals); i += 2 {
		fmt.Fprintf(&b, " %v=%v", keyvals[i], keyvals[i+1])
	}

	l.next.Printf("%s%s%s\n", prefix, msg, b.String())
}
