package commands

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/tendermint/go-amino"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	lite "github.com/tendermint/tendermint/lite2"
	lproxy "github.com/tendermint/tendermint/lite2/proxy"
	lrpc "github.com/tendermint/tendermint/lite2/rpc"
	dbs "github.com/tendermint/tendermint/lite2/store/db"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	rpcserver "github.com/tendermint/tendermint/rpc/jsonrpc/server"
)

// LiteCmd represents the base command when called without any subcommands
var LiteCmd = &cobra.Command{
	Use:   "lite [chainID]",
	Short: "Run a light client proxy server, verifying Tendermint rpc",
	Long: `Run a light client proxy server, verifying Tendermint rpc.

All calls that can be tracked back to a block header by a proof
will be verified before passing them back to the caller. Other than
that, it will present the same interface as a full Tendermint node.

Example:

start a fresh instance:

lite cosmoshub-3 -p 52.57.29.196:26657 -w public-seed-node.cosmoshub.certus.one:26657
	--height 962118 --hash 28B97BE9F6DE51AC69F70E0B7BFD7E5C9CD1A595B7DC31AFF27C50D4948020CD

continue from latest state:

lite cosmoshub-3 -p 52.57.29.196:26657 -w public-seed-node.cosmoshub.certus.one:26657
`,
	RunE: runProxy,
	Args: cobra.ExactArgs(1),
	Example: `lite cosmoshub-3 -p 52.57.29.196:26657 -w public-seed-node.cosmoshub.certus.one:26657
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
	LiteCmd.Flags().StringVar(&listenAddr, "laddr", "tcp://localhost:8888",
		"Serve the proxy on the given address")
	LiteCmd.Flags().StringVarP(&primaryAddr, "primary", "p", "",
		"Connect to a Tendermint node at this address")
	LiteCmd.Flags().StringVarP(&witnessAddrsJoined, "witnesses", "w", "",
		"Tendermint nodes to cross-check the primary node, comma-separated")
	LiteCmd.Flags().StringVar(&home, "home-dir", ".tendermint-lite", "Specify the home directory")
	LiteCmd.Flags().IntVar(
		&maxOpenConnections,
		"max-open-connections",
		900,
		"Maximum number of simultaneous connections (including WebSocket).")
	LiteCmd.Flags().DurationVar(&trustingPeriod, "trusting-period", 168*time.Hour,
		"Trusting period. Should be significantly less than the unbonding period")
	LiteCmd.Flags().Int64Var(&trustedHeight, "height", 1, "Trusted header's height")
	LiteCmd.Flags().BytesHexVar(&trustedHash, "hash", []byte{}, "Trusted header's hash")
	LiteCmd.Flags().BoolVar(&verbose, "verbose", false, "Verbose output")
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

	db, err := dbm.NewGoLevelDB("lite-client-db", home)
	if err != nil {
		return errors.Wrap(err, "new goleveldb")
	}

	var c *lite.Client
	if trustedHeight > 0 && len(trustedHash) > 0 { // fresh installation
		c, err = lite.NewHTTPClient(
			chainID,
			lite.TrustOptions{
				Period: trustingPeriod,
				Height: trustedHeight,
				Hash:   trustedHash,
			},
			primaryAddr,
			witnessesAddrs,
			dbs.New(db, chainID),
			lite.Logger(logger),
		)
	} else { // continue from latest state
		c, err = lite.NewHTTPClientFromTrustedStore(
			chainID,
			trustingPeriod,
			primaryAddr,
			witnessesAddrs,
			dbs.New(db, chainID),
			lite.Logger(logger),
		)
	}
	if err != nil {
		return err
	}

	rpcClient, err := rpchttp.New(primaryAddr, "/websocket")
	if err != nil {
		return errors.Wrapf(err, "http client for %s", primaryAddr)
	}
	p := lproxy.Proxy{
		Addr:   listenAddr,
		Config: &rpcserver.Config{MaxOpenConnections: maxOpenConnections},
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
