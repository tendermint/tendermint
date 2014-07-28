package blocks

import (
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("block")

func init() {
	logging.SetFormatter(logging.MustStringFormatter("[%{level:.1s}] %{message}"))
}

func SetLogger(l *logging.Logger) {
	log = l
}
