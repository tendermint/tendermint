package rpc

import (
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("rpc")

func init() {
	logging.SetFormatter(logging.MustStringFormatter("[%{level:.1s}] %{message}"))
}

func SetRPCLogger(l *logging.Logger) {
	log = l
}
