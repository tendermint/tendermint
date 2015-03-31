package binary

import (
	"github.com/tendermint/log15"
	"github.com/tendermint/tendermint2/logger"
)

var log = logger.New("module", "binary")

func init() {
	log.SetHandler(
		log15.LvlFilterHandler(
			log15.LvlWarn,
			//log15.LvlDebug,
			logger.RootHandler(),
		),
	)
}
