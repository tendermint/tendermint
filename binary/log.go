package binary

import (
	"os"

	"github.com/tendermint/log15"
)

var log = log15.New("module", "binary")

func init() {
	log.SetHandler(
		log15.LvlFilterHandler(
			//log15.LvlWarn,
			log15.LvlDebug,
			log15.StreamHandler(os.Stderr, log15.LogfmtFormat()),
		),
	)
}
