package common

import (
	"fmt"
	"runtime"
)

//----------------------------------------
// Error & cmnError

type Error interface {
	Error() string
	Trace(msg string) Error
	TraceCause(cause error, msg string) Error
	Cause() error
}

func NewError(msg string) Error {
	return newError(msg, nil)
}

func NewErrorWithCause(cause error, msg string) Error {
	return newError(msg, cause)
}

type traceItem struct {
	msg      string
	filename string
	lineno   int
}

func (ti traceItem) String() string {
	return fmt.Sprintf("%v:%v %v", ti.filename, ti.lineno, ti.msg)
}

type cmnError struct {
	msg    string
	cause  error
	traces []traceItem
}

func newError(msg string, cause error) *cmnError {
	return &cmnError{
		msg:    msg,
		cause:  cause,
		traces: nil,
	}
}

func (err *cmnError) Error() string {
	return fmt.Sprintf("Error{%s,%v,%v}", err.msg, err.cause, len(err.traces))
}

// Add tracing information with msg.
func (err *cmnError) Trace(msg string) Error {
	return err.doTrace(msg, 2)
}

// Add tracing information with cause and msg.
// If a cause was already set before, it is overwritten.
func (err *cmnError) TraceCause(cause error, msg string) Error {
	err.cause = cause
	return err.doTrace(msg, 2)
}

func (err *cmnError) doTrace(msg string, n int) Error {
	_, fn, line, ok := runtime.Caller(n)
	if !ok {
		if fn == "" {
			fn = "<unknown>"
		}
		if line <= 0 {
			line = -1
		}
	}
	// Include file & line number & msg.
	// Do not include the whole stack trace.
	err.traces = append(err.traces, traceItem{
		filename: fn,
		lineno:   line,
		msg:      msg,
	})
	return err
}

// Return last known cause.
// NOTE: The meaning of "cause" is left for the caller to define.
// There exists to canonical definition of "cause".
// Instead of blaming, try to handle-or-organize it.
func (err *cmnError) Cause() error {
	return err.cause
}

//----------------------------------------
// StackError

// NOTE: Used by Tendermint p2p upon recovery.
// Err could be "Reason", since it isn't an error type.
type StackError struct {
	Err   interface{}
	Stack []byte
}

func (se StackError) String() string {
	return fmt.Sprintf("Error: %v\nStack: %s", se.Err, se.Stack)
}

func (se StackError) Error() string {
	return se.String()
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
