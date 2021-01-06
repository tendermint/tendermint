package log

import (
	"fmt"
	"io"

	kitlog "github.com/go-kit/kit/log"
)

// Logger is what any Tendermint library should take.
type Logger interface {
	Debug(msg string, keyvals ...interface{})
	Info(msg string, keyvals ...interface{})
	Error(msg string, keyvals ...interface{})

	With(keyvals ...interface{}) Logger
}

// Hexadecimal is intended to convert a []byte
// type to a value that is hexadecimal (uppercase).
type Hexadecimal struct {
	b []byte
}

// String fulfills the Stringer interface within the
// fmt package.
func (s Hexadecimal) String() string {
	return fmt.Sprintf("%X", s.b)
}

// NewSyncWriter returns a new writer that is safe for concurrent use by
// multiple goroutines. Writes to the returned writer are passed on to w. If
// another write is already in progress, the calling goroutine blocks until
// the writer is available.
//
// If w implements the following interface, so does the returned writer.
//
//    interface {
//        Fd() uintptr
//    }
func NewSyncWriter(w io.Writer) io.Writer {
	return kitlog.NewSyncWriter(w)
}
