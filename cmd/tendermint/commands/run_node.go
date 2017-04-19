package commands

import (
	"io/ioutil"
	"time"

	"github.com/spf13/cobra"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/types"
)

var runNodeCmd = &cobra.Command{
	Use:    "node",
	Short:  "Run the tendermint node",
	PreRun: setConfigFlags,
	Run:    runNode,
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
	runNodeCmd.Flags().StringVar(&moniker, "moniker", "anonymous",
		"Node Name")
	runNodeCmd.Flags().StringVar(&nodeLaddr, "node_laddr", "tcp://0.0.0.0:46656",
		"Node listen address. (0.0.0.0:0 means any interface, any port)")
	runNodeCmd.Flags().StringVar(&seeds, "seeds", "",
		"Comma delimited host:port seed nodes")
	runNodeCmd.Flags().BoolVar(&fastSync, "fast_sync", true,
		"Fast blockchain syncing")
	runNodeCmd.Flags().BoolVar(&skipUPNP, "skip_upnp", false,
		"Skip UPNP configuration")
	runNodeCmd.Flags().StringVar(&rpcLaddr, "rpc_laddr", "tcp://0.0.0.0:46657",
		"RPC listen address. Port required")
	runNodeCmd.Flags().StringVar(&grpcLaddr, "grpc_laddr", "",
		"GRPC listen address (BroadcastTx only). Port required")
	runNodeCmd.Flags().StringVar(&proxyApp, "proxy_app", "tcp://127.0.0.1:46658",
		"Proxy app address, or 'nilapp' or 'dummy' for local testing.")
	runNodeCmd.Flags().StringVar(&abciTransport, "abci", "socket",
		"Specify abci transport (socket | grpc)")

	// feature flags
	runNodeCmd.Flags().BoolVar(&pex, "pex", false,
		"Enable Peer-Exchange (dev feature)")

	RootCmd.AddCommand(runNodeCmd)
}

func setConfigFlags(cmd *cobra.Command, args []string) {

	// Merge parsed flag values onto config
	config.Set("moniker", moniker)
	config.Set("node_laddr", nodeLaddr)
	config.Set("seeds", seeds)
	config.Set("fast_sync", fastSync)
	config.Set("skip_upnp", skipUPNP)
	config.Set("rpc_laddr", rpcLaddr)
	config.Set("grpc_laddr", grpcLaddr)
	config.Set("proxy_app", proxyApp)
	config.Set("abci", abciTransport)
	config.Set("pex_reactor", pex)
}

// Users wishing to:
//	* Use an external signer for their validators
//	* Supply an in-proc abci app
// should import github.com/tendermint/tendermint/node and implement
// their own run_node to call node.NewNode (instead of node.NewNodeDefault)
// with their custom priv validator and/or custom proxy.ClientCreator
func runNode(cmd *cobra.Command, args []string) {

	// Wait until the genesis doc becomes available
	// This is for Mintnet compatibility.
	// TODO: If Mintnet gets deprecated or genesis_file is
	// always available, remove.
	genDocFile := config.GetString("genesis_file")
	if !FileExists(genDocFile) {
		log.Notice(Fmt("Waiting for genesis file %v...", genDocFile))
		for {
			time.Sleep(time.Second)
			if !FileExists(genDocFile) {
				continue
			}
			jsonBlob, err := ioutil.ReadFile(genDocFile)
			if err != nil {
				Exit(Fmt("Couldn't read GenesisDoc file: %v", err))
			}
			genDoc, err := types.GenesisDocFromJSON(jsonBlob)
			if err != nil {
				Exit(Fmt("Error reading GenesisDoc: %v", err))
			}
			if genDoc.ChainID == "" {
				Exit(Fmt("Genesis doc %v must include non-empty chain_id", genDocFile))
			}
			config.Set("chain_id", genDoc.ChainID)
		}
	}

	// Create & start node
	n := node.NewNodeDefault(config)
	if _, err := n.Start(); err != nil {
		Exit(Fmt("Failed to start node: %v", err))
	} else {
		log.Notice("Started node", "nodeInfo", n.Switch().NodeInfo())
	}

	// Trap signal, run forever.
	n.RunForever()
}
