package log

import (
	"github.com/rs/zerolog"
)

func NewNopLogger() Logger {
	return &defaultLogger{
		Logger: zerolog.Nop(),
	}
}
