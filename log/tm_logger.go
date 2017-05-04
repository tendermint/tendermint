package log

import (
	"fmt"
	"io"

	kitlog "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/go-kit/kit/log/term"
)

const (
	msgKey = "_msg" // "_" prefixed to avoid collisions
)

type tmLogger struct {
	srcLogger kitlog.Logger
}

// Interface assertions
var _ Logger = (*tmLogger)(nil)

// NewTMTermLogger returns a logger that encodes msg and keyvals to the Writer
// using go-kit's log as an underlying logger and our custom formatter. Note
// that underlying logger could be swapped with something else.
//
// Default logging level is info. You can change it using SetLevel().
func NewTMLogger(w io.Writer) Logger {
	// Color by level value
	colorFn := func(keyvals ...interface{}) term.FgBgColor {
		if keyvals[0] != level.Key() {
			panic(fmt.Sprintf("expected level key to be first, got %v", keyvals[0]))
		}
		switch keyvals[1].(level.Value).String() {
		case "debug":
			return term.FgBgColor{Fg: term.DarkGray}
		case "error":
			return term.FgBgColor{Fg: term.Red}
		default:
			return term.FgBgColor{}
		}
	}

	srcLogger := term.NewLogger(w, NewTMFmtLogger, colorFn)
	srcLogger = level.NewFilter(srcLogger, level.AllowInfo())
	return &tmLogger{srcLogger}
}

// Info logs a message at level Info.
func (l *tmLogger) Info(msg string, keyvals ...interface{}) error {
	lWithLevel := level.Info(l.srcLogger)
	return kitlog.With(lWithLevel, msgKey, msg).Log(keyvals...)
}

// Debug logs a message at level Debug.
func (l *tmLogger) Debug(msg string, keyvals ...interface{}) error {
	lWithLevel := level.Debug(l.srcLogger)
	return kitlog.With(lWithLevel, msgKey, msg).Log(keyvals...)
}

// Error logs a message at level Error.
func (l *tmLogger) Error(msg string, keyvals ...interface{}) error {
	lWithLevel := level.Error(l.srcLogger)
	return kitlog.With(lWithLevel, msgKey, msg).Log(keyvals...)
}

// With returns a new contextual logger with keyvals prepended to those passed
// to calls to Info, Debug or Error.
func (l *tmLogger) With(keyvals ...interface{}) Logger {
	return &tmLogger{kitlog.With(l.srcLogger, keyvals...)}
}

// WithLevel returns a new logger with the level set to lvl.
func (l *tmLogger) WithLevel(lvl string) Logger {
	switch lvl {
	case "info":
		return &tmLogger{level.NewFilter(l.srcLogger, level.AllowInfo())}
	case "debug":
		return &tmLogger{level.NewFilter(l.srcLogger, level.AllowDebug())}
	case "error":
		return &tmLogger{level.NewFilter(l.srcLogger, level.AllowError())}
	default:
		panic(fmt.Sprintf("Unexpected level %v, expect either \"info\" or \"debug\" or \"error\"", lvl))
	}
}
