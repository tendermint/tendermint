package main

import (
	"flag"
	"os"

	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"

	priv_val "github.com/tendermint/tendermint/types/priv_validator"
)

func main() {
	var (
		chainID     = flag.String("chain-id", "mychain", "chain id")
		listenAddr  = flag.String("laddr", ":46659", "Validator listen address (0.0.0.0:0 means any interface, any port")
		maxConn     = flag.Int("clients", 3, "maximum of concurrent connections")
		privValPath = flag.String("priv", "", "priv val file path")

		logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout)).With("module", "priv_val")
	)
	flag.Parse()

	logger.Info(
		"Starting private validator",
		"chainID", *chainID,
		"listenAddr", *listenAddr,
		"maxConn", *maxConn,
		"privPath", *privValPath,
	)

	privVal := priv_val.LoadPrivValidatorJSON(*privValPath)

	pvss := priv_val.NewPrivValidatorSocketServer(
		logger,
		*chainID,
		*listenAddr,
		*maxConn,
		privVal,
		nil,
	)
	pvss.Start()

	cmn.TrapSignal(func() {
		pvss.Stop()
	})
}
