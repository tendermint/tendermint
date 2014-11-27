package alert

import (
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("alert")

func init() {
	logging.SetFormatter(logging.MustStringFormatter("[%{level:.1s}] %{message}"))
}

func SetAlertLogger(l *logging.Logger) {
	log = l
}
