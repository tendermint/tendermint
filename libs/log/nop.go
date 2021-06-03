package log

import (
	"github.com/rs/zerolog"
)

func NewNopLogger() DefaultLogger {
	return DefaultLogger{
		Logger: zerolog.Nop(),
		trace:  false,
	}
}
