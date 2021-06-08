package log

import (
	"io"
	"os"
	"sync"
	"testing"
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
func TestingLogger() Logger {
	return TestingLoggerWithOutput(os.Stdout)
}

func TestingLoggerWithOutput(w io.Writer) Logger {
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
