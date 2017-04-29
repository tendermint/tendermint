package commands

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/spf13/cobra"

	//cfg "github.com/tendermint/tendermint/config/tendermint"
	"github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
)

var runNodeCmd = &cobra.Command{
	Use:    "node",
	Short:  "Run the tendermint node",
	PreRun: setConfigFlags,
	RunE:   runNode,
}

//flags
var (
	moniker       string
	nodeLaddr     string
	seeds         string
	fastSync      bool
	skipUPNP      bool
	rpcLaddr      string
	grpcLaddr     string
	proxyApp      string
	abciTransport string
	pex           bool
)

func init() {

	// configuration options
	runNodeCmd.Flags().StringVar(&moniker, "moniker", config.GetString("node.moniker"),
		"Node Name")
	runNodeCmd.Flags().StringVar(&nodeLaddr, "node_laddr", config.GetString("node.listen_addr"),
		"Node listen address. (0.0.0.0:0 means any interface, any port)")
	runNodeCmd.Flags().StringVar(&seeds, "seeds", config.GetString("network.seeds"),
		"Comma delimited host:port seed nodes")
	runNodeCmd.Flags().BoolVar(&fastSync, "fast_sync", config.GetBool("blockchain.fast_sync"),
		"Fast blockchain syncing")
	runNodeCmd.Flags().BoolVar(&skipUPNP, "skip_upnp", config.GetBool("network.skip_upnp"),
		"Skip UPNP configuration")
	runNodeCmd.Flags().StringVar(&rpcLaddr, "rpc_laddr", config.GetString("rpc.listen_addr"),
		"RPC listen address. Port required")
	runNodeCmd.Flags().StringVar(&grpcLaddr, "grpc_laddr", config.GetString("grpc.listen_addr"),
		"GRPC listen address (BroadcastTx only). Port required")
	runNodeCmd.Flags().StringVar(&proxyApp, "proxy_app", config.GetString("abci.proxy_app"),
		"Proxy app address, or 'nilapp' or 'dummy' for local testing.")
	runNodeCmd.Flags().StringVar(&abciTransport, "abci", config.GetString("abci.mode"),
		"Specify abci transport (socket | grpc)")

	// feature flags
	runNodeCmd.Flags().BoolVar(&pex, "pex", config.GetBool("pex_reactor"),
		"Enable Peer-Exchange (dev feature)")

	RootCmd.AddCommand(runNodeCmd)
}

func setConfigFlags(cmd *cobra.Command, args []string) {

	// Merge parsed flag values onto config
	config.Set("node.moniker", moniker)
	config.Set("node.listen_addr", nodeLaddr)
	config.Set("network.seeds", seeds)
	config.Set("network.skip_upnp", skipUPNP)
	config.Set("network.pex_reactor", pex)
	config.Set("blockchain.fast_sync", fastSync)
	config.Set("rpc.listen_addr", rpcLaddr)
	config.Set("rpc.grpc_listen_addr", grpcLaddr)
	config.Set("abci.proxy_app", proxyApp)
	config.Set("abci.mode", abciTransport)
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
	genDocFile := config.GetString("genesis_file")
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
			config.Set("chain_id", genDoc.ChainID)
		}
	}

	// Create & start node
	n := node.NewNodeDefault(config) //tmConfig)
	if _, err := n.Start(); err != nil {
		return fmt.Errorf("Failed to start node: %v", err)
	} else {
		log.Notice("Started node", "nodeInfo", n.Switch().NodeInfo())
	}

	// Trap signal, run forever.
	n.RunForever()

	return nil
}
