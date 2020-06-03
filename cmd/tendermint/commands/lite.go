package commands

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tendermint/go-amino"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	"github.com/tendermint/tendermint/light"
	lproxy "github.com/tendermint/tendermint/light/proxy"
	lrpc "github.com/tendermint/tendermint/light/rpc"
	dbs "github.com/tendermint/tendermint/light/store/db"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
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

Example:

start a fresh instance:

light cosmoshub-3 -p http://52.57.29.196:26657 -w http://public-seed-node.cosmoshub.certus.one:26657
	--height 962118 --hash 28B97BE9F6DE51AC69F70E0B7BFD7E5C9CD1A595B7DC31AFF27C50D4948020CD

continue from latest state:

light cosmoshub-3 -p http://52.57.29.196:26657 -w http://public-seed-node.cosmoshub.certus.one:26657
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
	home               string
	maxOpenConnections int

	trustingPeriod time.Duration
	trustedHeight  int64
	trustedHash    []byte

	verbose bool
)

func init() {
	LightCmd.Flags().StringVar(&listenAddr, "laddr", "tcp://localhost:8888",
		"Serve the proxy on the given address")
	LightCmd.Flags().StringVarP(&primaryAddr, "primary", "p", "",
		"Connect to a Tendermint node at this address")
	LightCmd.Flags().StringVarP(&witnessAddrsJoined, "witnesses", "w", "",
		"Tendermint nodes to cross-check the primary node, comma-separated")
	LightCmd.Flags().StringVar(&home, "home-dir", ".tendermint-light", "Specify the home directory")
	LightCmd.Flags().IntVar(
		&maxOpenConnections,
		"max-open-connections",
		900,
		"Maximum number of simultaneous connections (including WebSocket).")
	LightCmd.Flags().DurationVar(&trustingPeriod, "trusting-period", 168*time.Hour,
		"Trusting period. Should be significantly less than the unbonding period")
	LightCmd.Flags().Int64Var(&trustedHeight, "height", 1, "Trusted header's height")
	LightCmd.Flags().BytesHexVar(&trustedHash, "hash", []byte{}, "Trusted header's hash")
	LightCmd.Flags().BoolVar(&verbose, "verbose", false, "Verbose output")
}

func runProxy(cmd *cobra.Command, args []string) error {
	// Initialise logger.
	logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout))
	var option log.Option
	if verbose {
		option, _ = log.AllowLevel("debug")
	} else {
		option, _ = log.AllowLevel("info")
	}
	logger = log.NewFilter(logger, option)

	chainID = args[0]
	logger.Info("Creating client...", "chainID", chainID)

	witnessesAddrs := strings.Split(witnessAddrsJoined, ",")

	db, err := dbm.NewGoLevelDB("light-client-db", home)
	if err != nil {
		return fmt.Errorf("can't create a db: %w", err)
	}

	var c *light.Client
	if trustedHeight > 0 && len(trustedHash) > 0 { // fresh installation
		c, err = light.NewHTTPClient(
			chainID,
			light.TrustOptions{
				Period: trustingPeriod,
				Height: trustedHeight,
				Hash:   trustedHash,
			},
			primaryAddr,
			witnessesAddrs,
			dbs.New(db, chainID),
			light.Logger(logger),
		)
	} else { // continue from latest state
		c, err = light.NewHTTPClientFromTrustedStore(
			chainID,
			trustingPeriod,
			primaryAddr,
			witnessesAddrs,
			dbs.New(db, chainID),
			light.Logger(logger),
		)
	}
	if err != nil {
		return err
	}

	rpcClient, err := rpchttp.New(primaryAddr, "/websocket")
	if err != nil {
		return fmt.Errorf("http client for %s: %w", primaryAddr, err)
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

	p := lproxy.Proxy{
		Addr:   listenAddr,
		Config: cfg,
		Codec:  amino.NewCodec(),
		Client: lrpc.NewClient(rpcClient, c),
		Logger: logger,
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
