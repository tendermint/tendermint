package consensus

import (
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("consensus")

func init() {
	logging.SetFormatter(logging.MustStringFormatter("[%{level:.1s}] %{message}"))
}

func SetConsensusLogger(l *logging.Logger) {
	log = l
}
