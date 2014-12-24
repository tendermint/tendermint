package alert

import (
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("alert")

func SetAlertLogger(l *logging.Logger) {
	log = l
}
