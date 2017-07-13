package commands

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
)

var runNodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Run the tendermint node",
	RunE:  runNode,
}

func init() {
	AddNodeFlags(runNodeCmd)
	RootCmd.AddCommand(runNodeCmd)
}

// AddNodeFlags exposes some common configuration options on the command-line
// These are exposed for convenience of commands embedding a tendermint node
func AddNodeFlags(cmd *cobra.Command) {
	// bind flags
	cmd.Flags().String("moniker", config.Moniker, "Node Name")

	// node flags
	cmd.Flags().Bool("fast_sync", config.FastSync, "Fast blockchain syncing")

	// abci flags
	cmd.Flags().String("proxy_app", config.ProxyApp, "Proxy app address, or 'nilapp' or 'dummy' for local testing.")
	cmd.Flags().String("abci", config.ABCI, "Specify abci transport (socket | grpc)")

	// rpc flags
	cmd.Flags().String("rpc.laddr", config.RPC.ListenAddress, "RPC listen address. Port required")
	cmd.Flags().String("rpc.grpc_laddr", config.RPC.GRPCListenAddress, "GRPC listen address (BroadcastTx only). Port required")
	cmd.Flags().Bool("rpc.unsafe", config.RPC.Unsafe, "Enabled unsafe rpc methods")

	// p2p flags
	cmd.Flags().String("p2p.laddr", config.P2P.ListenAddress, "Node listen address. (0.0.0.0:0 means any interface, any port)")
	cmd.Flags().String("p2p.seeds", config.P2P.Seeds, "Comma delimited host:port seed nodes")
	cmd.Flags().Bool("p2p.skip_upnp", config.P2P.SkipUPNP, "Skip UPNP configuration")
	cmd.Flags().Bool("p2p.pex", config.P2P.PexReactor, "Enable Peer-Exchange (dev feature)")

	// consensus flags
	cmd.Flags().Bool("consensus.no_empty_blocks", config.Consensus.NoEmptyBlocks, "Prevent empty blocks from being proposed by correct processes")
}

// Users wishing to:
//	* Use an external signer for their validators
//	* Supply an in-proc abci app
// should import github.com/tendermint/tendermint/node and implement
// their own run_node to call node.NewNode (instead of node.NewNodeDefault)
// with their custom priv validator and/or custom proxy.ClientCreator
func runNode(cmd *cobra.Command, args []string) error {

	// Wait until the genesis doc becomes available
	// This is for Mintnet compatibility.
	// TODO: If Mintnet gets deprecated or genesis_file is
	// always available, remove.
	genDocFile := config.GenesisFile()
	for !cmn.FileExists(genDocFile) {
		logger.Info(cmn.Fmt("Waiting for genesis file %v...", genDocFile))
		time.Sleep(time.Second)
	}

	genDoc, err := types.GenesisDocFromFile(genDocFile)
	if err != nil {
		return err
	}
	config.ChainID = genDoc.ChainID

	// Create & start node
	n := node.NewNodeDefault(config, logger.With("module", "node"))
	if _, err := n.Start(); err != nil {
		return fmt.Errorf("Failed to start node: %v", err)
	} else {
		logger.Info("Started node", "nodeInfo", n.Switch().NodeInfo())
	}

	// Trap signal, run forever.
	n.RunForever()

	return nil
}
