package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
	"github.com/tendermint/tendermint/abci/server"
	"github.com/tendermint/tendermint/config"
	tmflags "github.com/tendermint/tendermint/libs/cli/flags"
	"github.com/tendermint/tendermint/libs/log"
	tmnet "github.com/tendermint/tendermint/libs/net"
	"github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/proxy"
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
	cfg, err := LoadConfig(configFile)
	if err != nil {
		return err
	}

	switch cfg.Protocol {
	case "socket", "grpc":
		err = startApp(cfg)
	case "builtin":
		err = startNode(cfg)
	default:
		err = fmt.Errorf("invalid protocol %q", cfg.Protocol)
	}
	if err != nil {
		return err
	}

	// Start remote signer
	if cfg.PrivValServer != "" {
		if err = startSigner(cfg); err != nil {
			return err
		}
	}

	// Apparently there's no way to wait for the server, so we just sleep
	for {
		time.Sleep(1 * time.Hour)
	}
}

// startApp starts the application server, listening for connections from Tendermint.
func startApp(cfg *Config) error {
	app, err := NewApplication(cfg)
	if err != nil {
		return err
	}
	server, err := server.NewServer(cfg.Listen, cfg.Protocol, app)
	if err != nil {
		return err
	}
	err = server.Start()
	if err != nil {
		return err
	}
	logger.Info(fmt.Sprintf("Server listening on %v (%v protocol)", cfg.Listen, cfg.Protocol))
	return nil
}

// startNode starts a Tendermint node running the application directly. It assumes the Tendermint
// configuration is in $TMHOME/config/tendermint.toml.
//
// FIXME There is no way to simply load the configuration from a file, so we need to pull in Viper.
func startNode(cfg *Config) error {
	app, err := NewApplication(cfg)
	if err != nil {
		return err
	}

	home := os.Getenv("TMHOME")
	if home == "" {
		return errors.New("TMHOME not set")
	}
	viper.AddConfigPath(filepath.Join(home, "config"))
	viper.SetConfigName("config")
	err = viper.ReadInConfig()
	if err != nil {
		return err
	}
	tmcfg := config.DefaultConfig()
	err = viper.Unmarshal(tmcfg)
	if err != nil {
		return err
	}
	tmcfg.SetRoot(home)
	if err = tmcfg.ValidateBasic(); err != nil {
		return fmt.Errorf("error in config file: %v", err)
	}
	if tmcfg.LogFormat == config.LogFormatJSON {
		logger = log.NewTMJSONLogger(log.NewSyncWriter(os.Stdout))
	}
	logger, err = tmflags.ParseLogLevel(tmcfg.LogLevel, logger, config.DefaultLogLevel())
	if err != nil {
		return err
	}
	logger = logger.With("module", "main")

	nodeKey, err := p2p.LoadOrGenNodeKey(tmcfg.NodeKeyFile())
	if err != nil {
		return fmt.Errorf("failed to load or gen node key %s: %w", tmcfg.NodeKeyFile(), err)
	}

	n, err := node.NewNode(tmcfg,
		privval.LoadOrGenFilePV(tmcfg.PrivValidatorKeyFile(), tmcfg.PrivValidatorStateFile()),
		nodeKey,
		proxy.NewLocalClientCreator(app),
		node.DefaultGenesisDocProviderFunc(tmcfg),
		node.DefaultDBProvider,
		node.DefaultMetricsProvider(tmcfg.Instrumentation),
		logger,
	)
	if err != nil {
		return err
	}
	return n.Start()
}

// startSigner starts a signer server connecting to the given endpoint.
func startSigner(cfg *Config) error {
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
	err := privval.NewSignerServer(endpoint, cfg.ChainID, filePV).Start()
	if err != nil {
		return err
	}
	logger.Info(fmt.Sprintf("Remote signer connecting to %v", cfg.PrivValServer))
	return nil
}
