package mempool

import (
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("mempool")

func SetMempoolLogger(l *logging.Logger) {
	log = l
}
