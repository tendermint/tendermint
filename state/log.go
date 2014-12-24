package state

import (
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("state")

func SetStateLogger(l *logging.Logger) {
	log = l
}
