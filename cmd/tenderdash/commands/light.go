package commands

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/config"
	dashcore "github.com/tendermint/tendermint/dash/core"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/light"
	lproxy "github.com/tendermint/tendermint/light/proxy"
	lrpc "github.com/tendermint/tendermint/light/rpc"
	dbs "github.com/tendermint/tendermint/light/store/db"
	rpcserver "github.com/tendermint/tendermint/rpc/jsonrpc/server"
)

// LightCmd constructs the base command called when invoked without any subcommands.
func MakeLightCommand(conf *config.Config, logger log.Logger) *cobra.Command {
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

	checkForExistingProviders := func(db dbm.DB) (string, []string, error) {
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

	saveProviders := func(db dbm.DB, primaryAddr, witnessesAddrs string) error {
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

	cmd := &cobra.Command{
		Use:   "light [chainID]",
		Short: "Run a light client proxy server, verifying Tendermint rpc",
		Long: `Run a light client proxy server, verifying Tendermint rpc.

All calls that can be tracked back to a block header by a proof
will be verified before passing them back to the caller. Other than
that, it will present the same interface as a full Tendermint node.

Furthermore to the chainID, a fresh instance of a light client will
need a primary RPC address and witness RPC addresses. To restart the node, thereafter
only the chainID is required.

To restart the node, thereafter only the chainID is required.

When /abci_query is called, the Merkle key path format is:

	/{store name}/{key}

Please verify with your application that this Merkle key format is used (true
for applications built w/ Cosmos SDK).
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			chainID = args[0]
			logger.Info("Creating client...", "chainID", chainID)

			var witnessesAddrs []string
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
					return errors.New("no primary address was provided nor found. Please provide a primary (using -p)." +
						" Run the command: tendermint light --help for more information")
				}
			} else {
				err := saveProviders(db, primaryAddr, witnessAddrsJoined)
				if err != nil {
					logger.Error("Unable to save primary and or witness addresses", "err", err)
				}
			}
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

			rpcLogger := logger.With("module", dashcore.ModuleName)
			dashCoreRPCClient, _ := dashcore.NewRPCClient(dashCoreRPCHost, dashCoreRPCUser, dashCoreRPCPass, rpcLogger)

			c, err := light.NewHTTPClient(
				cmd.Context(),
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
			cfg.MaxBodyBytes = conf.RPC.MaxBodyBytes
			cfg.MaxHeaderBytes = conf.RPC.MaxHeaderBytes
			cfg.MaxOpenConnections = maxOpenConnections
			// If necessary adjust global WriteTimeout to ensure it's greater than
			// TimeoutBroadcastTxCommit.
			// See https://github.com/tendermint/tendermint/issues/3435
			if cfg.WriteTimeout <= conf.RPC.TimeoutBroadcastTxCommit {
				cfg.WriteTimeout = conf.RPC.TimeoutBroadcastTxCommit + 1*time.Second
			}

			p, err := lproxy.NewProxy(c, listenAddr, primaryAddr, cfg, logger, lrpc.KeyPathFn(lrpc.DefaultMerkleKeyPathFn()))
			if err != nil {
				return err
			}

			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			go func() {
				<-ctx.Done()
				p.Listener.Close()
			}()

			logger.Info("Starting proxy...", "laddr", listenAddr)
			if err := p.ListenAndServe(ctx); err != http.ErrServerClosed {
				// Error starting or closing listener:
				logger.Error("proxy ListenAndServe", "err", err)
			}

			return nil
		},
		Args: cobra.ExactArgs(1),
		Example: `light cosmoshub-3 -p http://52.57.29.196:26657 -w http://public-seed-node.cosmoshub.certus.one:26657
	--height 962118 --hash 28B97BE9F6DE51AC69F70E0B7BFD7E5C9CD1A595B7DC31AFF27C50D4948020CD`,
	}

	cmd.Flags().StringVar(&listenAddr, "laddr", "tcp://localhost:8888",
		"serve the proxy on the given address")
	cmd.Flags().StringVarP(&primaryAddr, "primary", "p", "",
		"connect to a Tendermint node at this address")
	cmd.Flags().StringVarP(&witnessAddrsJoined, "witnesses", "w", "",
		"tendermint nodes to cross-check the primary node, comma-separated")
	cmd.Flags().StringVarP(&dir, "dir", "d", os.ExpandEnv(filepath.Join("$HOME", ".tendermint-light")),
		"specify the directory")
	cmd.Flags().IntVar(
		&maxOpenConnections,
		"max-open-connections",
		900,
		"maximum number of simultaneous connections (including WebSocket).")
	cmd.Flags().StringVar(&logLevel, "log-level", log.LogLevelInfo, "The logging level (debug|info|warn|error|fatal)")
	cmd.Flags().StringVar(&logFormat, "log-format", log.LogFormatPlain, "The logging format (text|json)")
	cmd.Flags().StringVar(&dashCoreRPCHost, "dchost", "",
		"host address of the Dash Core RPC node")
	cmd.Flags().StringVar(&dashCoreRPCHost, "dcuser", "",
		"Dash Core RPC node user")
	cmd.Flags().StringVar(&dashCoreRPCHost, "dcpass", "",
		"Dash Core RPC node password")

	return cmd
}
