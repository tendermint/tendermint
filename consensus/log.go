package consensus

import (
	"github.com/tendermint/go-logger"
)

var log = logger.NewBypass("module", "consensus")

func init() {
	log.SetHandler(
		logger.LvlFilterHandler(
			logger.LvlDebug,
			logger.BypassHandler(),
		),
	)
}
