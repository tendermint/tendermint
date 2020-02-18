package commands

import (
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	amino "github.com/tendermint/go-amino"
	dbm "github.com/tendermint/tm-db"

	tmos "github.com/tendermint/tendermint/libs/os"
	lite "github.com/tendermint/tendermint/lite2"
	"github.com/tendermint/tendermint/lite2/provider"
	httpp "github.com/tendermint/tendermint/lite2/provider/http"
	lproxy "github.com/tendermint/tendermint/lite2/proxy"
	lrpc "github.com/tendermint/tendermint/lite2/rpc"
	dbs "github.com/tendermint/tendermint/lite2/store/db"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	rpcserver "github.com/tendermint/tendermint/rpc/lib/server"
)

// LiteCmd represents the base command when called without any subcommands
var LiteCmd = &cobra.Command{
	Use:   "lite",
	Short: "Run a light client proxy server, verifying Tendermint rpc",
	Long: `Run a light client proxy server, verifying Tendermint rpc.

All calls that can be tracked back to a block header by a proof
will be verified before passing them back to the caller. Other than
that, it will present the same interface as a full Tendermint node.`,
	RunE:         runProxy,
	SilenceUsage: true,
}

var (
	listenAddr         string
	primaryAddr        string
	chainID            string
	home               string
	witnessesAddrs     string
	maxOpenConnections int

	trustingPeriod time.Duration
	trustedHeight  int64
	trustedHash    []byte
)

func init() {
	LiteCmd.Flags().StringVar(&listenAddr, "laddr", "tcp://localhost:8888",
		"Serve the proxy on the given address")
	LiteCmd.Flags().StringVar(&chainID, "chain-id", "tendermint", "Specify the Tendermint chain ID")
	LiteCmd.Flags().StringVar(&primaryAddr, "primary", "tcp://localhost:26657",
		"Connect to a Tendermint node at this address")
	LiteCmd.Flags().StringVar(&witnessesAddrs, "witnesses", "",
		"Tendermint nodes to cross-check the primary node, comma-separated")
	LiteCmd.Flags().StringVar(&home, "home-dir", ".tendermint-lite", "Specify the home directory")
	LiteCmd.Flags().IntVar(
		&maxOpenConnections,
		"max-open-connections",
		900,
		"Maximum number of simultaneous connections (including WebSocket).")
	LiteCmd.Flags().DurationVar(&trustingPeriod, "trusting-period", 168*time.Hour,
		"Trusting period. Should be significantly less than the unbonding period")
	LiteCmd.Flags().Int64Var(&trustedHeight, "trusted-height", 1, "Trusted header's height")
	LiteCmd.Flags().BytesHexVar(&trustedHash, "trusted-hash", []byte{}, "Trusted header's hash")
}

func runProxy(cmd *cobra.Command, args []string) error {
	liteLogger := logger.With("module", "lite")

	logger.Info("Connecting to the primary node...")
	rpcClient, err := rpcclient.NewHTTP(chainID, primaryAddr)
	if err != nil {
		return errors.Wrapf(err, "http client for %s", primaryAddr)
	}
	primary := httpp.NewWithClient(chainID, rpcClient)

	logger.Info("Connecting to the witness nodes...")
	addrs := strings.Split(witnessesAddrs, ",")
	witnesses := make([]provider.Provider, len(addrs))
	for i, addr := range addrs {
		p, err := httpp.New(chainID, addr)
		if err != nil {
			return errors.Wrapf(err, "http provider for %s", addr)
		}
		witnesses[i] = p
	}

	logger.Info("Creating client...")
	db, err := dbm.NewGoLevelDB("lite-client-db", home)
	if err != nil {
		return err
	}
	c, err := lite.NewClient(
		chainID,
		lite.TrustOptions{
			Period: trustingPeriod,
			Height: trustedHeight,
			Hash:   trustedHash,
		},
		primary,
		witnesses,
		dbs.New(db, chainID),
		lite.Logger(liteLogger),
	)
	if err != nil {
		return err
	}

	p := lproxy.Proxy{
		Addr:   listenAddr,
		Config: &rpcserver.Config{MaxOpenConnections: maxOpenConnections},
		Codec:  amino.NewCodec(),
		Client: lrpc.NewClient(rpcClient, c),
		Logger: liteLogger,
	}
	// Stop upon receiving SIGTERM or CTRL-C.
	tmos.TrapSignal(liteLogger, func() {
		p.Listener.Close()
	})

	logger.Info("Starting proxy...")
	if err := p.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		logger.Error("proxy ListenAndServe", "err", err)
	}

	return nil
}
