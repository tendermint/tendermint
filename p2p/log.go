package p2p

import (
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("p2p")

func init() {
	logging.SetFormatter(logging.MustStringFormatter("[%{level:.1s}] %{message}"))
}

func SetLogger(l *logging.Logger) {
	log = l
}
