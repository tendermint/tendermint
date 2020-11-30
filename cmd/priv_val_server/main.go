package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
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
		rootCA           = flag.String("rootCA", "", "absolute path to root CA")

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

		certificate, err := tls.LoadX509KeyPair(
			*withCert,
			*withKey,
		)
		if err != nil {
			panic(err)
		}

		certPool := x509.NewCertPool()
		bs, err := ioutil.ReadFile(*rootCA)
		if err != nil {
			panic(fmt.Sprintf("failed to read client ca cert: %s", err))
		}

		ok := certPool.AppendCertsFromPEM(bs)
		if !ok {
			panic("failed to append client certs")
		}

		tlsConfig := &tls.Config{
			ClientAuth:   tls.RequireAndVerifyClientCert,
			Certificates: []tls.Certificate{certificate},
			ClientCAs:    certPool,
			MinVersion:   tls.VersionTLS13,
		}

		creds := grpc.Creds(credentials.NewTLS(tlsConfig))
		opts = append(opts, creds)
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
