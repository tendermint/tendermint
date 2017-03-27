package common

import (
	"fmt"
)

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

//--------------------------------------------------------------------------------------------------
// panic wrappers

// A panic resulting from a sanity check means there is a programmer error
// and some gaurantee is not satisfied.
func PanicSanity(v interface{}) {
	panic(Fmt("Panicked on a Sanity Check: %v", v))
}

// A panic here means something has gone horribly wrong, in the form of data corruption or
// failure of the operating system. In a correct/healthy system, these should never fire.
// If they do, it's indicative of a much more serious problem.
func PanicCrisis(v interface{}) {
	panic(Fmt("Panicked on a Crisis: %v", v))
}

// Indicates a failure of consensus. Someone was malicious or something has
// gone horribly wrong. These should really boot us into an "emergency-recover" mode
func PanicConsensus(v interface{}) {
	panic(Fmt("Panicked on a Consensus Failure: %v", v))
}

// For those times when we're not sure if we should panic
func PanicQ(v interface{}) {
	panic(Fmt("Panicked questionably: %v", v))
}
