package log

import "fmt"

// NewFilter wraps next and implements filtering. See the commentary on the
// Option functions for a detailed description of how to configure levels. If
// no options are provided, all leveled log events created with Debug, Info or
// Error helper methods are squelched.
func NewFilter(next Logger, options ...Option) Logger {
	l := &filter{
		next: next,
	}
	for _, option := range options {
		option(l)
	}
	return l
}

// NewFilterByLevel wraps next and implements filtering based on a given level.
// Error is returned if level is not info, error or debug.
func NewFilterByLevel(next Logger, lvl string) (Logger, error) {
	var option Option
	switch lvl {
	case "info":
		option = AllowInfo()
	case "debug":
		option = AllowDebug()
	case "error":
		option = AllowError()
	default:
		return nil, fmt.Errorf("Expected either \"info\", \"debug\" or \"error\" log level, given %v", lvl)
	}
	return NewFilter(next, option), nil
}

type filter struct {
	next          Logger
	allowed       level
	errNotAllowed error
}

func (l *filter) Info(msg string, keyvals ...interface{}) error {
	levelAllowed := l.allowed&levelInfo != 0
	if !levelAllowed {
		return l.errNotAllowed
	}
	return l.next.Info(msg, keyvals...)
}

func (l *filter) Debug(msg string, keyvals ...interface{}) error {
	levelAllowed := l.allowed&levelDebug != 0
	if !levelAllowed {
		return l.errNotAllowed
	}
	return l.next.Debug(msg, keyvals...)
}

func (l *filter) Error(msg string, keyvals ...interface{}) error {
	levelAllowed := l.allowed&levelError != 0
	if !levelAllowed {
		return l.errNotAllowed
	}
	return l.next.Error(msg, keyvals...)
}

func (l *filter) With(keyvals ...interface{}) Logger {
	return &filter{next: l.next.With(keyvals...), allowed: l.allowed, errNotAllowed: l.errNotAllowed}
}

// Option sets a parameter for the filter.
type Option func(*filter)

// AllowAll is an alias for AllowDebug.
func AllowAll() Option {
	return AllowDebug()
}

// AllowDebug allows error, info and debug level log events to pass.
func AllowDebug() Option {
	return allowed(levelError | levelInfo | levelDebug)
}

// AllowInfo allows error and info level log events to pass.
func AllowInfo() Option {
	return allowed(levelError | levelInfo)
}

// AllowError allows only error level log events to pass.
func AllowError() Option {
	return allowed(levelError)
}

// AllowNone allows no leveled log events to pass.
func AllowNone() Option {
	return allowed(0)
}

func allowed(allowed level) Option {
	return func(l *filter) { l.allowed = allowed }
}

// ErrNotAllowed sets the error to return from Log when it squelches a log
// event disallowed by the configured Allow[Level] option. By default,
// ErrNotAllowed is nil; in this case the log event is squelched with no
// error.
func ErrNotAllowed(err error) Option {
	return func(l *filter) { l.errNotAllowed = err }
}

type level byte

const (
	levelDebug level = 1 << iota
	levelInfo
	levelError
)
