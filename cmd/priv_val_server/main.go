package main

import (
	"flag"
	"os"

	crypto "github.com/tendermint/tendermint/crypto"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/tendermint/tendermint/privval"
)

func main() {
	var (
		addr        = flag.String("addr", ":26659", "Address of client to connect to")
		chainID     = flag.String("chain-id", "mychain", "chain id")
		privValPath = flag.String("priv", "", "priv val file path")

		logger = log.NewTMLogger(
			log.NewSyncWriter(os.Stdout),
		).With("module", "priv_val")
	)
	flag.Parse()

	logger.Info(
		"Starting private validator",
		"addr", *addr,
		"chainID", *chainID,
		"privPath", *privValPath,
	)

	pv := privval.LoadFilePV(*privValPath)

	rs := privval.NewRemoteSigner(
		logger,
		*chainID,
		*addr,
		pv,
		crypto.GenPrivKeyEd25519(),
	)
	err := rs.Start()
	if err != nil {
		panic(err)
	}

	cmn.TrapSignal(func() {
		err := rs.Stop()
		if err != nil {
			panic(err)
		}
	})
}
