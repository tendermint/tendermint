package common

import (
	"fmt"
	"runtime"
)

//----------------------------------------
// Convenience methods

// ErrorWrap will just call .TraceFrom(), or create a new *cmnError.
func ErrorWrap(cause interface{}, format string, args ...interface{}) Error {
	msg := Fmt(format, args...)
	if causeCmnError, ok := cause.(*cmnError); ok {
		return causeCmnError.TraceFrom(1, msg)
	}
	// NOTE: cause may be nil.
	// NOTE: do not use causeCmnError here, not the same as nil.
	return newError(msg, cause, cause).Stacktrace()
}

//----------------------------------------
// Error & cmnError

/*
Usage:

```go
	// Error construction
	var someT = errors.New("Some err type")
	var err1 error = NewErrorWithT(someT, "my message")
	...
	// Wrapping
	var err2 error  = ErrorWrap(err1, "another message")
	if (err1 != err2) { panic("should be the same")
	...
	// Error handling
	switch err2.T() {
		case someT: ...
	    default: ...
	}
```

*/
type Error interface {
	Error() string
	Message() string
	Stacktrace() Error
	Trace(format string, args ...interface{}) Error
	TraceFrom(offset int, format string, args ...interface{}) Error
	Cause() interface{}
	WithT(t interface{}) Error
	T() interface{}
	Format(s fmt.State, verb rune)
}

// New Error with no cause where the type is the format string of the message..
func NewError(format string, args ...interface{}) Error {
	msg := Fmt(format, args...)
	return newError(msg, nil, format)

}

// New Error with specified type and message.
func NewErrorWithT(t interface{}, format string, args ...interface{}) Error {
	msg := Fmt(format, args...)
	return newError(msg, nil, t)
}

// NOTE: The name of a function "NewErrorWithCause()" implies that you are
// creating a new Error, yet, if the cause is an Error, creating a new Error to
// hold a ref to the old Error is probably *not* what you want to do.
// So, use ErrorWrap(cause, format, a...) instead, which returns the same error
// if cause is an Error.
// IF you must set an Error as the cause of an Error,
// then you can use the WithCauser interface to do so manually.
// e.g. (error).(tmlibs.WithCauser).WithCause(causeError)

type WithCauser interface {
	WithCause(cause interface{}) Error
}

type cmnError struct {
	msg        string         // first msg which also appears in msg
	cause      interface{}    // underlying cause (or panic object)
	t          interface{}    // for switching on error
	msgtraces  []msgtraceItem // all messages traced
	stacktrace []uintptr      // first stack trace
}

var _ WithCauser = &cmnError{}
var _ Error = &cmnError{}

// NOTE: do not expose.
func newError(msg string, cause interface{}, t interface{}) *cmnError {
	return &cmnError{
		msg:        msg,
		cause:      cause,
		t:          t,
		msgtraces:  nil,
		stacktrace: nil,
	}
}

func (err *cmnError) Message() string {
	return err.msg
}

func (err *cmnError) Error() string {
	return fmt.Sprintf("%v", err)
}

// Captures a stacktrace if one was not already captured.
func (err *cmnError) Stacktrace() Error {
	if err.stacktrace == nil {
		var offset = 3
		var depth = 32
		err.stacktrace = captureStacktrace(offset, depth)
	}
	return err
}

// Add tracing information with msg.
func (err *cmnError) Trace(format string, args ...interface{}) Error {
	msg := Fmt(format, args...)
	return err.doTrace(msg, 0)
}

// Same as Trace, but traces the line `offset` calls out.
// If n == 0, the behavior is identical to Trace().
func (err *cmnError) TraceFrom(offset int, format string, args ...interface{}) Error {
	msg := Fmt(format, args...)
	return err.doTrace(msg, offset)
}

// Return last known cause.
// NOTE: The meaning of "cause" is left for the caller to define.
// There exists no "canonical" definition of "cause".
// Instead of blaming, try to handle it, or organize it.
func (err *cmnError) Cause() interface{} {
	return err.cause
}

// Overwrites the Error's cause.
func (err *cmnError) WithCause(cause interface{}) Error {
	err.cause = cause
	return err
}

// Overwrites the Error's type.
func (err *cmnError) WithT(t interface{}) Error {
	err.t = t
	return err
}

// Return the "type" of this message, primarily for switching
// to handle this Error.
func (err *cmnError) T() interface{} {
	return err.t
}

func (err *cmnError) doTrace(msg string, n int) Error {
	pc, _, _, _ := runtime.Caller(n + 2) // +1 for doTrace().  +1 for the caller.
	// Include file & line number & msg.
	// Do not include the whole stack trace.
	err.msgtraces = append(err.msgtraces, msgtraceItem{
		pc:  pc,
		msg: msg,
	})
	return err
}

func (err *cmnError) Format(s fmt.State, verb rune) {
	switch verb {
	case 'p':
		s.Write([]byte(fmt.Sprintf("%p", &err)))
	default:
		if s.Flag('#') {
			s.Write([]byte("--= Error =--\n"))
			// Write msg.
			s.Write([]byte(fmt.Sprintf("Message: %#s\n", err.msg)))
			// Write cause.
			s.Write([]byte(fmt.Sprintf("Cause: %#v\n", err.cause)))
			// Write type.
			s.Write([]byte(fmt.Sprintf("T: %#v\n", err.t)))
			// Write msg trace items.
			s.Write([]byte(fmt.Sprintf("Msg Traces:\n")))
			for i, msgtrace := range err.msgtraces {
				s.Write([]byte(fmt.Sprintf(" %4d  %s\n", i, msgtrace.String())))
			}
			// Write stack trace.
			if err.stacktrace != nil {
				s.Write([]byte(fmt.Sprintf("Stack Trace:\n")))
				for i, pc := range err.stacktrace {
					fnc := runtime.FuncForPC(pc)
					file, line := fnc.FileLine(pc)
					s.Write([]byte(fmt.Sprintf(" %4d  %s:%d\n", i, file, line)))
				}
			}
			s.Write([]byte("--= /Error =--\n"))
		} else {
			// Write msg.
			if err.cause != nil {
				s.Write([]byte(fmt.Sprintf("Error{`%s` (cause: %v)}", err.msg, err.cause))) // TODO tick-esc?
			} else {
				s.Write([]byte(fmt.Sprintf("Error{`%s`}", err.msg))) // TODO tick-esc?
			}
		}
	}
}

//----------------------------------------
// stacktrace & msgtraceItem

func captureStacktrace(offset int, depth int) []uintptr {
	var pcs = make([]uintptr, depth)
	n := runtime.Callers(offset, pcs)
	return pcs[0:n]
}

type msgtraceItem struct {
	pc  uintptr
	msg string
}

func (mti msgtraceItem) String() string {
	fnc := runtime.FuncForPC(mti.pc)
	file, line := fnc.FileLine(mti.pc)
	return fmt.Sprintf("%s:%d - %s",
		file, line,
		mti.msg,
	)
}

//----------------------------------------
// Panic wrappers
// XXX DEPRECATED

// A panic resulting from a sanity check means there is a programmer error
// and some guarantee is not satisfied.
// XXX DEPRECATED
func PanicSanity(v interface{}) {
	panic(Fmt("Panicked on a Sanity Check: %v", v))
}

// A panic here means something has gone horribly wrong, in the form of data corruption or
// failure of the operating system. In a correct/healthy system, these should never fire.
// If they do, it's indicative of a much more serious problem.
// XXX DEPRECATED
func PanicCrisis(v interface{}) {
	panic(Fmt("Panicked on a Crisis: %v", v))
}

// Indicates a failure of consensus. Someone was malicious or something has
// gone horribly wrong. These should really boot us into an "emergency-recover" mode
// XXX DEPRECATED
func PanicConsensus(v interface{}) {
	panic(Fmt("Panicked on a Consensus Failure: %v", v))
}

// For those times when we're not sure if we should panic
// XXX DEPRECATED
func PanicQ(v interface{}) {
	panic(Fmt("Panicked questionably: %v", v))
}
