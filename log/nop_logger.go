package log

type nopLogger struct{}

// Interface assertions
var _ Logger = (*nopLogger)(nil)

// NewNopLogger returns a logger that doesn't do anything.
func NewNopLogger() Logger { return &nopLogger{} }

func (nopLogger) Info(string, ...interface{}) error {
	return nil
}

func (nopLogger) Debug(string, ...interface{}) error {
	return nil
}

func (nopLogger) Error(string, ...interface{}) error {
	return nil
}

func (l *nopLogger) With(...interface{}) Logger {
	return l
}
