package log

import (
	"fmt"

	kitlog "github.com/go-kit/kit/log"
)

// Logger is what any Tendermint library should take.
type Logger interface {
	Debug(msg string, keyvals ...interface{}) error
	Info(msg string, keyvals ...interface{}) error
	Error(msg string, keyvals ...interface{}) error
}

// With returns a new contextual logger with keyvals prepended to those passed
// to calls to Info, Debug or Error.
func With(logger Logger, keyvals ...interface{}) Logger {
	switch logger.(type) {
	case *tmLogger:
		return &tmLogger{kitlog.With(logger.(*tmLogger).srcLogger, keyvals...)}
	case *nopLogger:
		return logger
	default:
		panic(fmt.Sprintf("Unexpected logger of type %T", logger))
	}
}
