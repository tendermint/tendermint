package log

const (
	// LogFormatPlain defines a logging format used for human-readable text-based
	// logging that is not structured. Typically, this format is used for development
	// and testing purposes.
	LogFormatPlain string = "plain"

	// LogFormatText defines a logging format used for human-readable text-based
	// logging that is not structured. Typically, this format is used for development
	// and testing purposes.
	LogFormatText string = "text"

	// LogFormatJSON defines a logging format for structured JSON-based logging
	// that is typically used in production environments, which can be sent to
	// logging facilities that support complex log parsing and querying.
	LogFormatJSON string = "json"

	// Supported loging levels
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
	LogLevelFatal = "fatal"
)

// Logger defines a generic logging interface compatible with Tendermint.
type Logger interface {
	Debug(msg string, keyVals ...interface{})
	Info(msg string, keyVals ...interface{})
	Error(msg string, keyVals ...interface{})

	With(keyVals ...interface{}) Logger
}
