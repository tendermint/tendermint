package log

import (
	"os"
	"testing"
)

var (
	// reuse the same logger across all tests
	_testingLogger Logger
)

// TestingLogger returns a TMLogger which writes to STDOUT if testing being run
// with the verbose (-v) flag, NopLogger otherwise.
//
// Note that the call to TestingLogger() must be made
// inside a test (not in the init func) because
// verbose flag only set at the time of testing.
func TestingLogger() Logger {
	if _testingLogger != nil {
		return _testingLogger
	}

	if testing.Verbose() {
		_testingLogger = NewTMLogger(os.Stdout)
	} else {
		_testingLogger = NewNopLogger()
	}

	return _testingLogger
}
