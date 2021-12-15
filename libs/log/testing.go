package log

import (
	"sync"
	"testing"

	"github.com/rs/zerolog"
)

var (
	// reuse the same logger across all tests
	testingLoggerMtx = sync.Mutex{}
	testingLogger    Logger
)

// TestingLogger returns a Logger which writes to STDOUT if test(s) are being
// run with the verbose (-v) flag, NopLogger otherwise.
//
// NOTE:
// - A call to NewTestingLogger() must be made inside a test (not in the init func)
// because verbose flag only set at the time of testing.
// - Repeated calls to this function within a single process will
// produce a single test log instance, and while the logger is safe
// for parallel use it it doesn't produce meaningful feedback for
// parallel tests.
func TestingLogger() Logger {
	testingLoggerMtx.Lock()
	defer testingLoggerMtx.Unlock()

	if testingLogger != nil {
		return testingLogger
	}

	if testing.Verbose() {
		testingLogger = MustNewDefaultLogger(LogFormatText, LogLevelDebug, true)
	} else {
		testingLogger = NewNopLogger()
	}

	return testingLogger
}

type testingWriter struct {
	t testing.TB
}

func (tw testingWriter) Write(in []byte) (int, error) {
	tw.t.Log(string(in))
	return len(in), nil
}

// NewTestingLogger converts a testing.T into a logging interface to
// make test failures and verbose provide better feedback associated
// with test failures. This logging instance is safe for use from
// multiple threads, but in general you should create one of these
// loggers ONCE for each *testing.T instance that you interact with.
//
// By default it collects only ERROR messages, or DEBUG messages in
// verbose mode, and relies on the underlying behavior of testing.T.Log()
func NewTestingLogger(t testing.TB) Logger {
	level := LogLevelError
	if testing.Verbose() {
		level = LogLevelDebug
	}

	return NewTestingLoggerWithLevel(t, level)
}

// NewTestingLoggerWithLevel creates a testing logger instance at a
// specific level that wraps the behavior of testing.T.Log().
func NewTestingLoggerWithLevel(t testing.TB, level string) Logger {
	logLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		t.Fatalf("failed to parse log level (%s): %v", level, err)
	}
	trace := false
	if testing.Verbose() {
		trace = true
	}

	return defaultLogger{
		Logger: zerolog.New(newSyncWriter(testingWriter{t})).Level(logLevel),
		trace:  trace,
	}
}
