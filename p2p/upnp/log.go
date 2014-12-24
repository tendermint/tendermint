package upnp

import (
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("upnp")

func SetUPNPLogger(l *logging.Logger) {
	log = l
}
