package p2p

import (
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("p2p")

func SetP2PLogger(l *logging.Logger) {
	log = l
}
