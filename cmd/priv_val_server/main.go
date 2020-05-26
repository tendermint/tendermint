package main

import (
	"flag"
	"os"

	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/tendermint/tendermint/privval"
)

func main() {
	var (
		addr             = flag.String("addr", "tcp://127.0.0.1:26659", "Address of client to connect to")
		chainID          = flag.String("chain-id", "mychain", "chain id")
		privValKeyPath   = flag.String("priv-key", "", "priv val key file path")
		privValStatePath = flag.String("priv-state", "", "priv val state file path")
		withCert         = flag.String("cert", "", "absolutepath to certificate")
		withKey          = flag.String("key", "", "absolutepath to key")

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

	opts := []grpc.ServerOption{}
	if *withCert != "" && *withKey != "" {
		creds, err := credentials.NewServerTLSFromFile(*withCert, *withKey)
		if err != nil {
			logger.Error("Could not load TLS keys:", "err", err)
		}
		opts = append(opts, grpc.Creds(creds))
	} else {
		logger.Error("You are using an insecure gRPC connection! Provide a certificate and key to connect securely")
	}
	s := grpc.NewServer(opts...)

	ss := privval.NewSignerServer(*addr, *chainID, pv, s, logger)

	err := ss.Start()
	if err != nil {
		panic(err)
	}

	// Stop upon receiving SIGTERM or CTRL-C.
	tmos.TrapSignal(logger, func() {
		err := ss.Stop()
		if err != nil {
			panic(err)
		}
	})

	// Run forever.
	select {}
}
