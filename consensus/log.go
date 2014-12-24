package consensus

import (
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("consensus")

func SetConsensusLogger(l *logging.Logger) {
	log = l
}
