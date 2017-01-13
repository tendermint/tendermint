package main

import (
	flag "github.com/spf13/pflag"
	"os"

	cfg "github.com/tendermint/go-config"
)

func parseFlags(config cfg.Config, args []string) {
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

	// Declare flags
	var flags = flag.NewFlagSet("main", flag.ExitOnError)
	flags.BoolVar(&printHelp, "help", false, "Print this help message.")

	// configuration options
	flags.StringVar(&moniker, "moniker", config.GetString("moniker"), "Node Name")
	flags.StringVar(&nodeLaddr, "node_laddr", config.GetString("node_laddr"), "Node listen address. (0.0.0.0:0 means any interface, any port)")
	flags.StringVar(&seeds, "seeds", config.GetString("seeds"), "Comma delimited host:port seed nodes")
	flags.BoolVar(&fastSync, "fast_sync", config.GetBool("fast_sync"), "Fast blockchain syncing")
	flags.BoolVar(&skipUPNP, "skip_upnp", config.GetBool("skip_upnp"), "Skip UPNP configuration")
	flags.StringVar(&rpcLaddr, "rpc_laddr", config.GetString("rpc_laddr"), "RPC listen address. Port required")
	flags.StringVar(&grpcLaddr, "grpc_laddr", config.GetString("grpc_laddr"), "GRPC listen address (BroadcastTx only). Port required")
	flags.StringVar(&logLevel, "log_level", config.GetString("log_level"), "Log level")
	flags.StringVar(&proxyApp, "proxy_app", config.GetString("proxy_app"),
		"Proxy app address, or 'nilapp' or 'dummy' for local testing.")
	flags.StringVar(&abciTransport, "abci", config.GetString("abci"), "Specify abci transport (socket | grpc)")

	// feature flags
	flags.BoolVar(&pex, "pex", config.GetBool("pex_reactor"), "Enable Peer-Exchange (dev feature)")

	flags.Parse(args)
	if printHelp {
		flags.PrintDefaults()
		os.Exit(0)
	}

	// Merge parsed flag values onto app.
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
}
