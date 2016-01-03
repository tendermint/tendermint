package mempool

import (
	"github.com/tendermint/go-logger"
)

var log = logger.New("module", "mempool")

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
