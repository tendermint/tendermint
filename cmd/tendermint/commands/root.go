package commands

import (
	"github.com/spf13/cobra"

	cfg "github.com/tendermint/go-config"
	"github.com/tendermint/go-logger"
	tmcfg "github.com/tendermint/tendermint/config/tendermint"
)

var config cfg.Config
var log = logger.New("module", "main")

//global flags
var (
	printHelp     bool
	moniker       string
	nodeLaddr     string
	seeds         string
	fastSync      bool
	skipUPNP      bool
	rpcLaddr      string
	grpcLaddr     string
	logLevel      string
	proxyApp      string
	abciTransport string

	pex bool
)

var RootCmd = &cobra.Command{
	Use:   "tendermint",
	Short: "Tendermint Core (BFT Consensus) in Go",
}

func init() {

	// Get configuration
	config = tmcfg.GetConfig("")

	/////////////////////
	// parse flags

	// configuration options
	RootCmd.PersistentFlags().StringVar(&moniker, "moniker", config.GetString("moniker"), "Node Name")
	RootCmd.PersistentFlags().StringVar(&nodeLaddr, "node_laddr", config.GetString("node_laddr"), "Node listen address. (0.0.0.0:0 means any interface, any port)")
	RootCmd.PersistentFlags().StringVar(&seeds, "seeds", config.GetString("seeds"), "Comma delimited host:port seed nodes")
	RootCmd.PersistentFlags().BoolVar(&fastSync, "fast_sync", config.GetBool("fast_sync"), "Fast blockchain syncing")
	RootCmd.PersistentFlags().BoolVar(&skipUPNP, "skip_upnp", config.GetBool("skip_upnp"), "Skip UPNP configuration")
	RootCmd.PersistentFlags().StringVar(&rpcLaddr, "rpc_laddr", config.GetString("rpc_laddr"), "RPC listen address. Port required")
	RootCmd.PersistentFlags().StringVar(&grpcLaddr, "grpc_laddr", config.GetString("grpc_laddr"), "GRPC listen address (BroadcastTx only). Port required")
	RootCmd.PersistentFlags().StringVar(&logLevel, "log_level", config.GetString("log_level"), "Log level")
	RootCmd.PersistentFlags().StringVar(&proxyApp, "proxy_app", config.GetString("proxy_app"), "Proxy app address, or 'nilapp' or 'dummy' for local testing.")
	RootCmd.PersistentFlags().StringVar(&abciTransport, "abci", config.GetString("abci"), "Specify abci transport (socket | grpc)")

	// feature flags
	RootCmd.PersistentFlags().BoolVar(&pex, "pex", config.GetBool("pex_reactor"), "Enable Peer-Exchange (dev feature)")

	//------------------

	// Merge parsed flag values onto config
	config.Set("moniker", moniker)
	config.Set("node_laddr", nodeLaddr)
	config.Set("seeds", seeds)
	config.Set("fast_sync", fastSync)
	config.Set("skip_upnp", skipUPNP)
	config.Set("rpc_laddr", rpcLaddr)
	config.Set("grpc_laddr", grpcLaddr)
	config.Set("log_level", logLevel)
	config.Set("proxy_app", proxyApp)
	config.Set("abci", abciTransport)
	config.Set("pex_reactor", pex)

	// set the log level
	logger.SetLogLevel(config.GetString("log_level"))
}
