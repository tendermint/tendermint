package commands

import (
	"fmt"
	"io/ioutil"
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
	// bind flags
	runNodeCmd.Flags().String("moniker", config.Moniker, "Node Name")

	// node flags
	runNodeCmd.Flags().Bool("fast_sync", config.FastSync, "Fast blockchain syncing")

	// abci flags
	runNodeCmd.Flags().String("proxy_app", config.ProxyApp, "Proxy app address, or 'nilapp' or 'dummy' for local testing.")
	runNodeCmd.Flags().String("abci", config.ABCI, "Specify abci transport (socket | grpc)")

	// rpc flags
	runNodeCmd.Flags().String("rpc.laddr", config.RPC.ListenAddress, "RPC listen address. Port required")
	runNodeCmd.Flags().String("rpc.grpc_laddr", config.RPC.GRPCListenAddress, "GRPC listen address (BroadcastTx only). Port required")
	runNodeCmd.Flags().Bool("rpc.unsafe", config.RPC.Unsafe, "Enabled unsafe rpc methods")

	// p2p flags
	runNodeCmd.Flags().String("p2p.laddr", config.P2P.ListenAddress, "Node listen address. (0.0.0.0:0 means any interface, any port)")
	runNodeCmd.Flags().String("p2p.seeds", config.P2P.Seeds, "Comma delimited host:port seed nodes")
	runNodeCmd.Flags().Bool("p2p.skip_upnp", config.P2P.SkipUPNP, "Skip UPNP configuration")
	runNodeCmd.Flags().Bool("p2p.pex", config.P2P.PexReactor, "Enable Peer-Exchange (dev feature)")

	RootCmd.AddCommand(runNodeCmd)
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
	if !cmn.FileExists(genDocFile) {
		logger.Info(cmn.Fmt("Waiting for genesis file %v...", genDocFile))
		for {
			time.Sleep(time.Second)
			if !cmn.FileExists(genDocFile) {
				continue
			}
			jsonBlob, err := ioutil.ReadFile(genDocFile)
			if err != nil {
				return fmt.Errorf("Couldn't read GenesisDoc file: %v", err)
			}
			genDoc, err := types.GenesisDocFromJSON(jsonBlob)
			if err != nil {
				return fmt.Errorf("Error reading GenesisDoc: %v", err)
			}
			if genDoc.ChainID == "" {
				return fmt.Errorf("Genesis doc %v must include non-empty chain_id", genDocFile)
			}
			config.ChainID = genDoc.ChainID
		}
	}

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
