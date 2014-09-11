package mempool

import (
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("mempool")

func init() {
	logging.SetFormatter(logging.MustStringFormatter("[%{level:.1s}] %{message}"))
}

func SetMempoolLogger(l *logging.Logger) {
	log = l
}
