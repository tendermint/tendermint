package mempool

import (
	"github.com/tendermint/tmlibs/logger"
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
