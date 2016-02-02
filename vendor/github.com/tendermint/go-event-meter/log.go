package eventmeter

import (
	"github.com/tendermint/go-logger"
)

var log = logger.New("module", "event-meter")

/*
func init() {
	log.SetHandler(
		logger.LvlFilterHandler(
			logger.LvlDebug,
			logger.BypassHandler(),
		),
	)
}
*/
