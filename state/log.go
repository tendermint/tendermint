package state

import (
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("state")

func init() {
	logging.SetFormatter(logging.MustStringFormatter("[%{level:.1s}] %{message}"))
}

func SetStateLogger(l *logging.Logger) {
	log = l
}
