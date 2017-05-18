package log

import "fmt"

// NewFilter wraps next and implements filtering. See the commentary on the
// Option functions for a detailed description of how to configure levels. If
// no options are provided, all leveled log events created with Debug, Info or
// Error helper methods are squelched.
func NewFilter(next Logger, options ...Option) Logger {
	l := &filter{
		next:           next,
		allowedKeyvals: make(map[keyval]level),
	}
	for _, option := range options {
		option(l)
	}
	return l
}

// AllowLevel returns an option for the given level or error if no option exist
// for such level.
func AllowLevel(lvl string) (Option, error) {
	switch lvl {
	case "debug":
		return AllowDebug(), nil
	case "info":
		return AllowInfo(), nil
	case "error":
		return AllowError(), nil
	case "none":
		return AllowNone(), nil
	default:
		return nil, fmt.Errorf("Expected either \"info\", \"debug\", \"error\" or \"none\" level, given %s", lvl)
	}
}

type filter struct {
	next           Logger
	allowed        level
	allowedKeyvals map[keyval]level
	errNotAllowed  error
}

type keyval struct {
	key   interface{}
	value interface{}
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

// With implements Logger by constructing a new filter with a keyvals appended
// to the logger.
//
// If custom level was set for a keyval pair using one of the
// Allow*With methods, it is used as the logger's level.
//
// Examples:
//     logger = log.NewFilter(logger, log.AllowError(), log.AllowInfoWith("module", "crypto"))
//		 logger.With("module", "crypto").Info("Hello") # produces "I... Hello module=crypto"
//
//     logger = log.NewFilter(logger, log.AllowError(), log.AllowInfoWith("module", "crypto"), log.AllowNoneWith("user", "Sam"))
//		 logger.With("module", "crypto", "user", "Sam").Info("Hello") # returns nil
//
//     logger = log.NewFilter(logger, log.AllowError(), log.AllowInfoWith("module", "crypto"), log.AllowNoneWith("user", "Sam"))
//		 logger.With("user", "Sam").With("module", "crypto").Info("Hello") # produces "I... Hello module=crypto user=Sam"
func (l *filter) With(keyvals ...interface{}) Logger {
	for i := len(keyvals) - 2; i >= 0; i -= 2 {
		for kv, allowed := range l.allowedKeyvals {
			if keyvals[i] == kv.key && keyvals[i+1] == kv.value {
				return &filter{next: l.next.With(keyvals...), allowed: allowed, errNotAllowed: l.errNotAllowed, allowedKeyvals: l.allowedKeyvals}
			}
		}
	}
	return &filter{next: l.next.With(keyvals...), allowed: l.allowed, errNotAllowed: l.errNotAllowed, allowedKeyvals: l.allowedKeyvals}
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

// AllowDebugWith allows error, info and debug level log events to pass for a specific key value pair.
func AllowDebugWith(key interface{}, value interface{}) Option {
	return func(l *filter) { l.allowedKeyvals[keyval{key, value}] = levelError | levelInfo | levelDebug }
}

// AllowInfoWith allows error and info level log events to pass for a specific key value pair.
func AllowInfoWith(key interface{}, value interface{}) Option {
	return func(l *filter) { l.allowedKeyvals[keyval{key, value}] = levelError | levelInfo }
}

// AllowErrorWith allows only error level log events to pass for a specific key value pair.
func AllowErrorWith(key interface{}, value interface{}) Option {
	return func(l *filter) { l.allowedKeyvals[keyval{key, value}] = levelError }
}

// AllowNoneWith allows no leveled log events to pass for a specific key value pair.
func AllowNoneWith(key interface{}, value interface{}) Option {
	return func(l *filter) { l.allowedKeyvals[keyval{key, value}] = 0 }
}

type level byte

const (
	levelDebug level = 1 << iota
	levelInfo
	levelError
)
