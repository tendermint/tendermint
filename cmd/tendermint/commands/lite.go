package commands

import (
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
	LiteCmd.Flags().StringVar(&listenAddr, "laddr", ":8888", "Serve the proxy on the given port")
	LiteCmd.Flags().StringVar(&nodeAddr, "node", "localhost:46657", "Connect to a Tendermint node at this address")
	LiteCmd.Flags().StringVar(&chainID, "chain-id", "tendermint", "Specify the Tendermint chain ID")
	LiteCmd.Flags().StringVar(&home, "home-dir", ".tendermint-lite", "Specify the home directory")
}

func runProxy(cmd *cobra.Command, args []string) error {
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
