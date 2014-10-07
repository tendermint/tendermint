package state

import (
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("state")

func init() {
	logging.SetFormatter(logging.MustStringFormatter("[%{level:.1s}] %{message}"))
}

func SetStatesLogger(l *logging.Logger) {
	log = l
}
