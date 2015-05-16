package main

import (
	flag "github.com/spf13/pflag"
	"os"

	cfg "github.com/tendermint/tendermint/config"
)

func parseFlags(config cfg.Config, args []string) {
	var (
		printHelp bool
		moniker   string
		nodeLaddr string
		seeds     string
		fastSync  bool
		rpcLaddr  string
		logLevel  string
	)

	// Declare flags
	var flags = flag.NewFlagSet("main", flag.ExitOnError)
	flags.BoolVar(&printHelp, "help", false, "Print this help message.")
	flags.StringVar(&moniker, "moniker", config.GetString("moniker"), "Node Name")
	flags.StringVar(&nodeLaddr, "node_laddr", config.GetString("node_laddr"), "Node listen address. (0.0.0.0:0 means any interface, any port)")
	flags.StringVar(&seeds, "seeds", config.GetString("seeds"), "Comma delimited seed nodes")
	flags.BoolVar(&fastSync, "fast_sync", config.GetBool("fast_sync"), "Fast blockchain syncing")
	flags.StringVar(&rpcLaddr, "rpc_laddr", config.GetString("rpc_laddr"), "RPC listen address. Port required")
	flags.StringVar(&logLevel, "log_level", config.GetString("log_level"), "Log level")
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
	config.Set("rpc_laddr", rpcLaddr)
	config.Set("log_level", logLevel)
}
