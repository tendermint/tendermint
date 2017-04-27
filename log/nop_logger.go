package log

type nopLogger struct{}

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
