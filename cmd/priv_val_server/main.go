package main

import (
	"flag"
	"fmt"
	"os"

	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"

	priv_val "github.com/tendermint/tendermint/types/priv_validator"
)

var chainID = flag.String("chain-id", "mychain", "chain id")
var privValPath = flag.String("priv", "", "priv val file path")

func main() {
	flag.Parse()

	logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout)).With("module", "priv_val")
	socketAddr := "localhost:46659"
	fmt.Println(*chainID)
	fmt.Println(*privValPath)
	privVal := priv_val.LoadPrivValidatorJSON(*privValPath)

	pvss := priv_val.NewPrivValidatorSocketServer(logger, socketAddr, *chainID, privVal)
	pvss.Start()

	cmn.TrapSignal(func() {
		pvss.Stop()
	})
}
