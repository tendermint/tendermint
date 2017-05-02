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

func registerRunNodeFlagString(flagName, desc string) {
	runNodeCmd.Flags().String(flagName, viperConfig.GetString(flagName), desc)
	viperConfig.BindPFlag(flagName, runNodeCmd.Flags().Lookup(flagName))
}

func registerRunNodeFlagBool(flagName, desc string) {
	runNodeCmd.Flags().Bool(flagName, viperConfig.GetBool(flagName), desc)
	viperConfig.BindPFlag(flagName, runNodeCmd.Flags().Lookup(flagName))
}

func init() {
	// bind flags

	// node flags
	registerRunNodeFlagString("moniker", "Node Name")
	registerRunNodeFlagBool("fast_sync", "Fast blockchain syncing")

	// abci flags
	registerRunNodeFlagString("proxy_app", "Proxy app address, or 'nilapp' or 'dummy' for local testing.")
	registerRunNodeFlagString("abci", "Specify abci transport (socket | grpc)")

	// rpc flags
	registerRunNodeFlagString("rpc_laddr", "RPC listen address. Port required")
	registerRunNodeFlagString("grpc_laddr", "GRPC listen address (BroadcastTx only). Port required")

	// p2p flags
	registerRunNodeFlagString("p2p.laddr", "Node listen address. (0.0.0.0:0 means any interface, any port)")
	registerRunNodeFlagString("p2p.seeds", "Comma delimited host:port seed nodes")
	registerRunNodeFlagBool("p2p.skip_upnp", "Skip UPNP configuration")

	// feature flags
	registerRunNodeFlagBool("p2p.pex", "Enable Peer-Exchange (dev feature)")

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
	genDocFile := config.GenesisFile
	if !cmn.FileExists(genDocFile) {
		log.Notice(cmn.Fmt("Waiting for genesis file %v...", genDocFile))
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
	n := node.NewNodeDefault(config)
	if _, err := n.Start(); err != nil {
		return fmt.Errorf("Failed to start node: %v", err)
	} else {
		log.Notice("Started node", "nodeInfo", n.Switch().NodeInfo())
	}

	// Trap signal, run forever.
	n.RunForever()

	return nil
}
