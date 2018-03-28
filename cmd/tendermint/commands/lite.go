package commands

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"

	cmn "github.com/tendermint/tmlibs/common"

	"github.com/tendermint/tendermint/lite/proxy"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
)

// LiteCmd represents the base command when called without any subcommands
var LiteCmd = &cobra.Command{
	Use:   "lite",
	Short: "Run lite-client proxy server, verifying tendermint rpc",
	Long: `This node will run a secure proxy to a tendermint rpc server.

All calls that can be tracked back to a block header by a proof
will be verified before passing them back to the caller. Other that
that it will present the same interface as a full tendermint node,
just with added trust and running locally.`,
	RunE:         runProxy,
	SilenceUsage: true,
}

var (
	listenAddr string
	nodeAddr   string
	chainID    string
	home       string
)

func init() {
	LiteCmd.Flags().StringVar(&listenAddr, "laddr", "tcp://localhost:8888", "Serve the proxy on the given address")
	LiteCmd.Flags().StringVar(&nodeAddr, "node", "tcp://localhost:46657", "Connect to a Tendermint node at this address")
	LiteCmd.Flags().StringVar(&chainID, "chain-id", "tendermint", "Specify the Tendermint chain ID")
	LiteCmd.Flags().StringVar(&home, "home-dir", ".tendermint-lite", "Specify the home directory")
}

func ensureAddrHasSchemeOrDefaultToTCP(addr string) (string, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "tcp", "unix":
	case "":
		u.Scheme = "tcp"
	default:
		return "", fmt.Errorf("unknown scheme %q, use either tcp or unix", u.Scheme)
	}
	return u.String(), nil
}

func runProxy(cmd *cobra.Command, args []string) error {
	nodeAddr, err := ensureAddrHasSchemeOrDefaultToTCP(nodeAddr)
	if err != nil {
		return err
	}
	listenAddr, err := ensureAddrHasSchemeOrDefaultToTCP(listenAddr)
	if err != nil {
		return err
	}

	// First, connect a client
	node := rpcclient.NewHTTP(nodeAddr, "/websocket")

	cert, err := proxy.GetCertifier(chainID, home, nodeAddr)
	if err != nil {
		return err
	}
	sc := proxy.SecureClient(node, cert)

	err = proxy.StartProxy(sc, listenAddr, logger)
	if err != nil {
		return err
	}

	cmn.TrapSignal(func() {
		// TODO: close up shop
	})

	return nil
}
