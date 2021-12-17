package commands

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	dashcore "github.com/tendermint/tendermint/dashcore/rpc"

	"github.com/spf13/cobra"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	"github.com/tendermint/tendermint/light"
	lproxy "github.com/tendermint/tendermint/light/proxy"
	lrpc "github.com/tendermint/tendermint/light/rpc"
	dbs "github.com/tendermint/tendermint/light/store/db"
	rpcserver "github.com/tendermint/tendermint/rpc/jsonrpc/server"
)

// LightCmd represents the base command when called without any subcommands
var LightCmd = &cobra.Command{
	Use:   "light [chainID]",
	Short: "Run a light client proxy server, verifying Tendermint rpc",
	Long: `Run a light client proxy server, verifying Tendermint rpc.

All calls that can be tracked back to a block header by a proof
will be verified before passing them back to the caller. Other than
that, it will present the same interface as a full Tendermint node.

Furthermore to the chainID, a fresh instance of a light client will
need a primary RPC address and witness RPC addresses. To restart the node, thereafter
only the chainID is required.

When /abci_query is called, the Merkle key path format is:

	/{store name}/{key}

Please verify with your application that this Merkle key format is used (true
for applications built w/ Cosmos SDK).
`,
	RunE: runProxy,
	Args: cobra.ExactArgs(1),
	Example: `light cosmoshub-3 -p http://52.57.29.196:26657 -w http://public-seed-node.cosmoshub.certus.one:26657
	--height 962118 --hash 28B97BE9F6DE51AC69F70E0B7BFD7E5C9CD1A595B7DC31AFF27C50D4948020CD`,
}

var (
	listenAddr         string
	primaryAddr        string
	witnessAddrsJoined string
	chainID            string
	dir                string
	maxOpenConnections int

	logLevel  string
	logFormat string

	primaryKey   = []byte("primary")
	witnessesKey = []byte("witnesses")

	dashCoreRPCHost string
	dashCoreRPCUser string
	dashCoreRPCPass string
)

func init() {
	LightCmd.Flags().StringVar(&listenAddr, "laddr", "tcp://localhost:8888",
		"serve the proxy on the given address")
	LightCmd.Flags().StringVarP(&primaryAddr, "primary", "p", "",
		"connect to a Tendermint node at this address")
	LightCmd.Flags().StringVarP(&witnessAddrsJoined, "witnesses", "w", "",
		"tendermint nodes to cross-check the primary node, comma-separated")
	LightCmd.Flags().StringVarP(&dir, "dir", "d", os.ExpandEnv(filepath.Join("$HOME", ".tendermint-light")),
		"specify the directory")
	LightCmd.Flags().IntVar(
		&maxOpenConnections,
		"max-open-connections",
		900,
		"maximum number of simultaneous connections (including WebSocket).")
	LightCmd.Flags().StringVar(&logLevel, "log-level", log.LogLevelInfo, "The logging level (debug|info|warn|error|fatal)")
	LightCmd.Flags().StringVar(&logFormat, "log-format", log.LogFormatPlain, "The logging format (text|json)")
	LightCmd.Flags().StringVar(&dashCoreRPCHost, "dchost", "",
		"host address of the Dash Core RPC node")
	LightCmd.Flags().StringVar(&dashCoreRPCHost, "dcuser", "",
		"Dash Core RPC node user")
	LightCmd.Flags().StringVar(&dashCoreRPCHost, "dcpass", "",
		"Dash Core RPC node password")
}

func runProxy(cmd *cobra.Command, args []string) error {
	logger, err := log.NewDefaultLogger(logFormat, logLevel, false)
	if err != nil {
		return err
	}

	chainID = args[0]
	logger.Info("Creating client...", "chainID", chainID)

	witnessesAddrs := []string{}
	if witnessAddrsJoined != "" {
		witnessesAddrs = strings.Split(witnessAddrsJoined, ",")
	}

	lightDB, err := dbm.NewGoLevelDB("light-client-db", dir)
	if err != nil {
		return fmt.Errorf("can't create a db: %w", err)
	}
	// create a prefixed db on the chainID
	db := dbm.NewPrefixDB(lightDB, []byte(chainID))

	if primaryAddr == "" { // check to see if we can start from an existing state
		var err error
		primaryAddr, witnessesAddrs, err = checkForExistingProviders(db)
		if err != nil {
			return fmt.Errorf("failed to retrieve primary or witness from db: %w", err)
		}
		if primaryAddr == "" {
			return errors.New(
				"no primary address was provided nor found. Please provide a primary (using -p)." +
					" Run the command: tendermint light --help for more information",
			)
		}
	} else {
		err := saveProviders(db, primaryAddr, witnessAddrsJoined)
		if err != nil {
			logger.Error("Unable to save primary and or witness addresses", "err", err)
		}
	}

	options := []light.Option{
		light.Logger(logger),
		light.DashCoreVerification(),
	}

	dashCoreRPCClient, _ := dashcore.NewRPCClient(dashCoreRPCHost, dashCoreRPCUser, dashCoreRPCPass)

	c, err := light.NewHTTPClient(
		context.Background(),
		chainID,
		primaryAddr,
		witnessesAddrs,
		dbs.New(db),
		dashCoreRPCClient,
		options...,
	)
	if err != nil {
		return err
	}

	cfg := rpcserver.DefaultConfig()
	cfg.MaxBodyBytes = config.RPC.MaxBodyBytes
	cfg.MaxHeaderBytes = config.RPC.MaxHeaderBytes
	cfg.MaxOpenConnections = maxOpenConnections
	// If necessary adjust global WriteTimeout to ensure it's greater than
	// TimeoutBroadcastTxCommit.
	// See https://github.com/tendermint/tendermint/issues/3435
	if cfg.WriteTimeout <= config.RPC.TimeoutBroadcastTxCommit {
		cfg.WriteTimeout = config.RPC.TimeoutBroadcastTxCommit + 1*time.Second
	}

	p, err := lproxy.NewProxy(c, listenAddr, primaryAddr, cfg, logger, lrpc.KeyPathFn(lrpc.DefaultMerkleKeyPathFn()))
	if err != nil {
		return err
	}

	// Stop upon receiving SIGTERM or CTRL-C.
	tmos.TrapSignal(logger, func() {
		p.Listener.Close()
	})

	logger.Info("Starting proxy...", "laddr", listenAddr)
	if err := p.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		logger.Error("proxy ListenAndServe", "err", err)
	}

	return nil
}

func checkForExistingProviders(db dbm.DB) (string, []string, error) {
	primaryBytes, err := db.Get(primaryKey)
	if err != nil {
		return "", []string{""}, err
	}
	witnessesBytes, err := db.Get(witnessesKey)
	if err != nil {
		return "", []string{""}, err
	}
	witnessesAddrs := strings.Split(string(witnessesBytes), ",")
	return string(primaryBytes), witnessesAddrs, nil
}

func saveProviders(db dbm.DB, primaryAddr, witnessesAddrs string) error {
	err := db.Set(primaryKey, []byte(primaryAddr))
	if err != nil {
		return fmt.Errorf("failed to save primary provider: %w", err)
	}
	err = db.Set(witnessesKey, []byte(witnessesAddrs))
	if err != nil {
		return fmt.Errorf("failed to save witness providers: %w", err)
	}
	return nil
}
