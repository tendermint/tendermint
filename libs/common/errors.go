package common

import (
	"fmt"
	"runtime"
)

//----------------------------------------
// Convenience method.

func ErrorWrap(cause interface{}, format string, args ...interface{}) Error {
	if causeCmnError, ok := cause.(*cmnError); ok {
		msg := fmt.Sprintf(format, args...)
		return causeCmnError.Stacktrace().Trace(1, msg)
	} else if cause == nil {
		return newCmnError(FmtError{format, args}).Stacktrace()
	} else {
		// NOTE: causeCmnError is a typed nil here.
		msg := fmt.Sprintf(format, args...)
		return newCmnError(cause).Stacktrace().Trace(1, msg)
	}
}

//----------------------------------------
// Error & cmnError

/*

Usage with arbitrary error data:

```go
	// Error construction
	type MyError struct{}
	var err1 error = NewErrorWithData(MyError{}, "my message")
	...
	// Wrapping
	var err2 error  = ErrorWrap(err1, "another message")
	if (err1 != err2) { panic("should be the same")
	...
	// Error handling
	switch err2.Data().(type){
		case MyError: ...
	    default: ...
	}
```
*/
type Error interface {
	Error() string
	Stacktrace() Error
	Trace(offset int, format string, args ...interface{}) Error
	Data() interface{}
}

// New Error with formatted message.
// The Error's Data will be a FmtError type.
func NewError(format string, args ...interface{}) Error {
	err := FmtError{format, args}
	return newCmnError(err)
}

// New Error with specified data.
func NewErrorWithData(data interface{}) Error {
	return newCmnError(data)
}

type cmnError struct {
	data       interface{}    // associated data
	msgtraces  []msgtraceItem // all messages traced
	stacktrace []uintptr      // first stack trace
}

var _ Error = &cmnError{}

// NOTE: do not expose.
func newCmnError(data interface{}) *cmnError {
	return &cmnError{
		data:       data,
		msgtraces:  nil,
		stacktrace: nil,
	}
}

// Implements error.
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
// Set n=0 unless wrapped with some function, then n > 0.
func (err *cmnError) Trace(offset int, format string, args ...interface{}) Error {
	msg := fmt.Sprintf(format, args...)
	return err.doTrace(msg, offset)
}

// Return the "data" of this error.
// Data could be used for error handling/switching,
// or for holding general error/debug information.
func (err *cmnError) Data() interface{} {
	return err.data
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
			// Write data.
			s.Write([]byte(fmt.Sprintf("Data: %#v\n", err.data)))
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
			s.Write([]byte(fmt.Sprintf("Error{%v}", err.data))) // TODO tick-esc?
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
// fmt error

/*

FmtError is the data type for NewError() (e.g. NewError().Data().(FmtError))
Theoretically it could be used to switch on the format string.

```go
	// Error construction
	var err1 error = NewError("invalid username %v", "BOB")
	var err2 error = NewError("another kind of error")
	...
	// Error handling
	switch err1.Data().(cmn.FmtError).Format() {
		case "invalid username %v": ...
		case "another kind of error": ...
	    default: ...
	}
```
*/
type FmtError struct {
	format string
	args   []interface{}
}

func (fe FmtError) Error() string {
	return fmt.Sprintf(fe.format, fe.args...)
}

func (fe FmtError) String() string {
	return fmt.Sprintf("FmtError{format:%v,args:%v}",
		fe.format, fe.args)
}

func (fe FmtError) Format() string {
	return fe.format
}

//----------------------------------------
// Panic wrappers
// XXX DEPRECATED

// A panic resulting from a sanity check means there is a programmer error
// and some guarantee is not satisfied.
// XXX DEPRECATED
func PanicSanity(v interface{}) {
	panic(fmt.Sprintf("Panicked on a Sanity Check: %v", v))
}

// A panic here means something has gone horribly wrong, in the form of data corruption or
// failure of the operating system. In a correct/healthy system, these should never fire.
// If they do, it's indicative of a much more serious problem.
// XXX DEPRECATED
func PanicCrisis(v interface{}) {
	panic(fmt.Sprintf("Panicked on a Crisis: %v", v))
}

// Indicates a failure of consensus. Someone was malicious or something has
// gone horribly wrong. These should really boot us into an "emergency-recover" mode
// XXX DEPRECATED
func PanicConsensus(v interface{}) {
	panic(fmt.Sprintf("Panicked on a Consensus Failure: %v", v))
}

// For those times when we're not sure if we should panic
// XXX DEPRECATED
func PanicQ(v interface{}) {
	panic(fmt.Sprintf("Panicked questionably: %v", v))
}
