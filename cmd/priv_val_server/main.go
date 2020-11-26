package main

import (
	"flag"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	"github.com/tendermint/tendermint/privval"
	grpcprivval "github.com/tendermint/tendermint/privval/grpc"
)

func main() {
	var (
		addr             = flag.String("addr", ":26659", "Address of client to connect to")
		chainID          = flag.String("chain-id", "mychain", "chain id")
		privValKeyPath   = flag.String("priv-key", "", "priv val key file path")
		privValStatePath = flag.String("priv-state", "", "priv val state file path")
		insecure         = flag.Bool("priv-insecure", false, "")
		withCert         = flag.String("cert", "", "absolute path to server certificate")
		withKey          = flag.String("key", "", "absolute path to server key")

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
	if !*insecure {
		creds, err := credentials.NewServerTLSFromFile(*withCert, *withKey)
		if err != nil {
			logger.Error("Could not load TLS keys:", "err", err)
		}
		opts = append(opts, grpc.Creds(creds))
	} else {
		logger.Error("You are using an insecure gRPC connection! Provide a certificate and key to connect securely")
	}

	ss := grpcprivval.NewSignerServer(*addr, *chainID, pv, logger, opts)

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
