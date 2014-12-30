package binary

import (
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("binary")

func SetBinaryLogger(l *logging.Logger) {
	log = l
}
