package rpc

import (
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("rpc")

func SetRPCLogger(l *logging.Logger) {
	log = l
}
