package main

import (
	"fmt"
	"os"
	"time"

	"github.com/tendermint/tendermint/abci/server"
	"github.com/tendermint/tendermint/libs/log"
	tmnet "github.com/tendermint/tendermint/libs/net"
	"github.com/tendermint/tendermint/privval"
)

var logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout))

// main is the binary entrypoint.
func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %v <configfile>", os.Args[0])
		return
	}
	configFile := ""
	if len(os.Args) == 2 {
		configFile = os.Args[1]
	}

	if err := run(configFile); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

// run runs the application - basically like main() with error handling.
func run(configFile string) error {
	// Set up application
	cfg, err := LoadConfig(configFile)
	if err != nil {
		return err
	}
	app, err := NewApplication(cfg)
	if err != nil {
		return err
	}

	// Start ABCI server
	protocol := "socket"
	if cfg.GRPC {
		protocol = "grpc"
	}
	server, err := server.NewServer(cfg.Listen, protocol, app)
	if err != nil {
		return err
	}
	err = server.Start()
	if err != nil {
		return err
	}
	logger.Info(fmt.Sprintf("Server listening on %v (%v protocol)", cfg.Listen, protocol))

	// Start KMS client
	if cfg.PrivValServer != "" {
		filePV := privval.LoadFilePV(cfg.PrivValKey, cfg.PrivValState)
		protocol, address := tmnet.ProtocolAndAddress(cfg.PrivValServer)
		var dialFn privval.SocketDialer
		switch protocol {
		case "tcp":
			dialFn = privval.DialTCPFn(address, 3*time.Second, filePV.Key.PrivKey)
		case "unix":
			dialFn = privval.DialUnixFn(address)
		default:
			return fmt.Errorf("invalid privval protocol %q", protocol)
		}
		endpoint := privval.NewSignerDialerEndpoint(logger, dialFn,
			privval.SignerDialerEndpointRetryWaitInterval(1*time.Second),
			privval.SignerDialerEndpointConnRetries(100))
		err = privval.NewSignerServer(endpoint, cfg.ChainID, filePV).Start()
		if err != nil {
			return err
		}
		logger.Info(fmt.Sprintf("KMS client connecting to %v", cfg.PrivValServer))
	}

	// Apparently there's no way to wait for the server, so we just sleep
	for {
		time.Sleep(1 * time.Hour)
	}
}
