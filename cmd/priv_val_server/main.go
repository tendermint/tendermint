package main

import (
	"flag"
	"os"
	"time"

	"github.com/tendermint/tendermint/crypto/ed25519"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/tendermint/tendermint/privval"
)

func main() {
	var (
		addr             = flag.String("addr", ":26659", "Address of client to connect to")
		chainID          = flag.String("chain-id", "mychain", "chain id")
		privValKeyPath   = flag.String("priv-key", "", "priv val key file path")
		privValStatePath = flag.String("priv-state", "", "priv val state file path")

		logger = log.NewTMLogger(
			log.NewSyncWriter(os.Stdout),
		).With("module", "priv_val")
	)
	flag.Parse()

	logger.Info(
		"Starting private validator",
		"addr", *addr,
		"chainID", *chainID,
		"privKeyPath", *privValKeyPath,
		"privStatePath", *privValStatePath,
	)

	pv := privval.LoadFilePV(*privValKeyPath, *privValStatePath)

	var dialer privval.Dialer
	protocol, address := cmn.ProtocolAndAddress(*addr)
	switch protocol {
	case "unix":
		dialer = privval.DialUnixFn(address)
	case "tcp":
		connTimeout := 3 * time.Second // TODO
		dialer = privval.DialTCPFn(address, connTimeout, ed25519.GenPrivKey())
	default:
		logger.Error("Unknown protocol", "protocol", protocol)
		os.Exit(1)
	}

	rs := privval.NewRemoteSigner(logger, *chainID, pv, dialer)
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
