package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
	"google.golang.org/grpc"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/server"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/libs/log"
	tmnet "github.com/tendermint/tendermint/libs/net"
	"github.com/tendermint/tendermint/light"
	lproxy "github.com/tendermint/tendermint/light/proxy"
	lrpc "github.com/tendermint/tendermint/light/rpc"
	dbs "github.com/tendermint/tendermint/light/store/db"
	"github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/privval"
	grpcprivval "github.com/tendermint/tendermint/privval/grpc"
	privvalproto "github.com/tendermint/tendermint/proto/tendermint/privval"
	rpcserver "github.com/tendermint/tendermint/rpc/jsonrpc/server"
	"github.com/tendermint/tendermint/test/e2e/app"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

// main is the binary entrypoint.
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if len(os.Args) != 2 {
		fmt.Printf("Usage: %v <configfile>", os.Args[0])
		return
	}
	configFile := ""
	if len(os.Args) == 2 {
		configFile = os.Args[1]
	}

	if err := run(ctx, configFile); err != nil {
		os.Exit(1)
	}
}

// run runs the application - basically like main() with error handling.
func run(ctx context.Context, configFile string) error {
	cfg, err := LoadConfig(configFile)
	if err != nil {
		return err
	}

	logger, err := log.NewDefaultLogger(log.LogFormatPlain, log.LogLevelInfo)
	if err != nil {
		// have print here because we can't log (yet), use the logger
		// everywhere else.
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		return err
	}

	if cfg.Mode == string(e2e.ModeLight) {
		err = startLightNode(ctx, logger, cfg)
	} else {
		// Start remote signer (must start before node if running builtin).
		if cfg.PrivValServer != "" {
			if err = startSigner(ctx, logger, cfg); err != nil {
				logger.Error("starting signer",
					"server", cfg.PrivValServer,
					"err", err)
				return err
			}
			if cfg.Protocol == "builtin" {
				time.Sleep(1 * time.Second)
			}
		}

		// Start app server.
		switch cfg.Protocol {
		case "socket", "grpc":
			err = startApp(ctx, logger, cfg)
		case "builtin":
			if cfg.Mode == string(e2e.ModeSeed) {
				err = startSeedNode(ctx)
			} else {
				err = startNode(ctx, cfg)
			}
		default:
			err = fmt.Errorf("invalid protocol %q", cfg.Protocol)
		}
	}

	if err != nil {
		logger.Error("starting node",
			"protocol", cfg.Protocol,
			"mode", cfg.Mode,
			"err", err)
		return err
	}

	// Apparently there's no way to wait for the server, so we just sleep
	for {
		time.Sleep(1 * time.Hour)
	}
}

// startApp starts the application server, listening for connections from Tendermint.
func startApp(ctx context.Context, logger log.Logger, cfg *Config) error {
	app, err := app.NewApplication(cfg.App())
	if err != nil {
		return err
	}
	server, err := server.NewServer(logger, cfg.Listen, cfg.Protocol, app)
	if err != nil {
		return err
	}
	err = server.Start(ctx)
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
func startNode(ctx context.Context, cfg *Config) error {
	app, err := app.NewApplication(cfg.App())
	if err != nil {
		return err
	}

	tmcfg, nodeLogger, err := setupNode()
	if err != nil {
		return fmt.Errorf("failed to setup config: %w", err)
	}

	n, err := node.New(
		ctx,
		tmcfg,
		nodeLogger,
		abciclient.NewLocalClient(nodeLogger, app),
		nil,
	)
	if err != nil {
		return err
	}
	return n.Start(ctx)
}

func startSeedNode(ctx context.Context) error {
	tmcfg, nodeLogger, err := setupNode()
	if err != nil {
		return fmt.Errorf("failed to setup config: %w", err)
	}

	tmcfg.Mode = config.ModeSeed

	n, err := node.New(ctx, tmcfg, nodeLogger, nil, nil)
	if err != nil {
		return err
	}
	return n.Start(ctx)
}

func startLightNode(ctx context.Context, logger log.Logger, cfg *Config) error {
	tmcfg, nodeLogger, err := setupNode()
	if err != nil {
		return err
	}

	dbContext := &config.DBContext{ID: "light", Config: tmcfg}
	lightDB, err := config.DefaultDBProvider(dbContext)
	if err != nil {
		return err
	}

	providers := rpcEndpoints(tmcfg.P2P.PersistentPeers)

	c, err := light.NewHTTPClient(
		ctx,
		cfg.ChainID,
		light.TrustOptions{
			Period: tmcfg.StateSync.TrustPeriod,
			Height: tmcfg.StateSync.TrustHeight,
			Hash:   tmcfg.StateSync.TrustHashBytes(),
		},
		providers[0],
		providers[1:],
		dbs.New(lightDB),
		light.Logger(nodeLogger),
	)
	if err != nil {
		return err
	}

	rpccfg := rpcserver.DefaultConfig()
	rpccfg.MaxBodyBytes = tmcfg.RPC.MaxBodyBytes
	rpccfg.MaxHeaderBytes = tmcfg.RPC.MaxHeaderBytes
	rpccfg.MaxOpenConnections = tmcfg.RPC.MaxOpenConnections
	// If necessary adjust global WriteTimeout to ensure it's greater than
	// TimeoutBroadcastTxCommit.
	// See https://github.com/tendermint/tendermint/issues/3435
	// Note we don't need to adjust anything if the timeout is already unlimited.
	if rpccfg.WriteTimeout > 0 && rpccfg.WriteTimeout <= tmcfg.RPC.TimeoutBroadcastTxCommit {
		rpccfg.WriteTimeout = tmcfg.RPC.TimeoutBroadcastTxCommit + 1*time.Second
	}

	p, err := lproxy.NewProxy(c, tmcfg.RPC.ListenAddress, providers[0], rpccfg, nodeLogger,
		lrpc.KeyPathFn(lrpc.DefaultMerkleKeyPathFn()))
	if err != nil {
		return err
	}

	logger.Info("Starting proxy...", "laddr", tmcfg.RPC.ListenAddress)
	if err := p.ListenAndServe(ctx); err != http.ErrServerClosed {
		// Error starting or closing listener:
		logger.Error("proxy ListenAndServe", "err", err)
	}

	return nil
}

// startSigner starts a signer server connecting to the given endpoint.
func startSigner(ctx context.Context, logger log.Logger, cfg *Config) error {
	filePV, err := privval.LoadFilePV(cfg.PrivValKey, cfg.PrivValState)
	if err != nil {
		return err
	}

	protocol, address := tmnet.ProtocolAndAddress(cfg.PrivValServer)
	var dialFn privval.SocketDialer
	switch protocol {
	case "tcp":
		dialFn = privval.DialTCPFn(address, 3*time.Second, ed25519.GenPrivKey())
	case "unix":
		dialFn = privval.DialUnixFn(address)
	case "grpc":
		lis, err := net.Listen("tcp", address)
		if err != nil {
			return err
		}
		ss := grpcprivval.NewSignerServer(logger, cfg.ChainID, filePV)

		s := grpc.NewServer()

		privvalproto.RegisterPrivValidatorAPIServer(s, ss)

		go func() { // no need to clean up since we remove docker containers
			if err := s.Serve(lis); err != nil {
				panic(err)
			}
			go func() {
				<-ctx.Done()
				s.GracefulStop()
			}()
		}()

		return nil
	default:
		return fmt.Errorf("invalid privval protocol %q", protocol)
	}

	endpoint := privval.NewSignerDialerEndpoint(logger, dialFn,
		privval.SignerDialerEndpointRetryWaitInterval(1*time.Second),
		privval.SignerDialerEndpointConnRetries(100))

	err = privval.NewSignerServer(endpoint, cfg.ChainID, filePV).Start(ctx)
	if err != nil {
		return err
	}

	logger.Info(fmt.Sprintf("Remote signer connecting to %v", cfg.PrivValServer))
	return nil
}

func setupNode() (*config.Config, log.Logger, error) {
	var tmcfg *config.Config

	home := os.Getenv("TMHOME")
	if home == "" {
		return nil, nil, errors.New("TMHOME not set")
	}

	viper.AddConfigPath(filepath.Join(home, "config"))
	viper.SetConfigName("config")

	if err := viper.ReadInConfig(); err != nil {
		return nil, nil, err
	}

	tmcfg = config.DefaultConfig()

	if err := viper.Unmarshal(tmcfg); err != nil {
		return nil, nil, err
	}

	tmcfg.SetRoot(home)

	if err := tmcfg.ValidateBasic(); err != nil {
		return nil, nil, fmt.Errorf("error in config file: %w", err)
	}

	nodeLogger, err := log.NewDefaultLogger(tmcfg.LogFormat, tmcfg.LogLevel)
	if err != nil {
		return nil, nil, err
	}

	return tmcfg, nodeLogger.With("module", "main"), nil
}

// rpcEndpoints takes a list of persistent peers and splits them into a list of rpc endpoints
// using 26657 as the port number
func rpcEndpoints(peers string) []string {
	arr := strings.Split(peers, ",")
	endpoints := make([]string, len(arr))
	for i, v := range arr {
		addr, err := p2p.ParseNodeAddress(v)
		if err != nil {
			panic(err)
		}
		// use RPC port instead
		addr.Port = 26657
		var rpcEndpoint string
		// for ipv6 addresses
		if strings.Contains(addr.Hostname, ":") {
			rpcEndpoint = "http://[" + addr.Hostname + "]:" + fmt.Sprint(addr.Port)
		} else { // for ipv4 addresses
			rpcEndpoint = "http://" + addr.Hostname + ":" + fmt.Sprint(addr.Port)
		}
		endpoints[i] = rpcEndpoint
	}
	return endpoints
}
