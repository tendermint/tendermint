package main

import (
	"fmt"
	"os"

	cfg "github.com/tendermint/tendermint/config"
	tmcfg "github.com/tendermint/tendermint/config/tendermint"
	"github.com/tendermint/tendermint/node"
)

func main() {

	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println(`Tendermint

Commands:
    node            Run the tendermint node
    show_validator  Show this node's validator info
    gen_account     Generate new account keypair
    gen_validator   Generate new validator keypair
    get_account     Get account balance
    send_tx         Sign and publish a SendTx
    probe_upnp      Test UPnP functionality
    version         Show version info
`)
		return
	}

	// Get configuration
	config := tmcfg.GetConfig("")
	parseFlags(config, args[1:]) // Command line overrides
	cfg.ApplyConfig(config)      // Notify modules of new config

	switch args[0] {
	case "node":
		node.RunNode()
	case "show_validator":
		show_validator()
	case "gen_account":
		gen_account()
	case "gen_validator":
		gen_validator()
	case "get_account":
		get_account()
	case "send_tx":
		send_tx()
	case "probe_upnp":
		probe_upnp()
	case "unsafe_reset_priv_validator":
		reset_priv_validator()
	case "version":
		fmt.Println(node.Version)
	default:
		Exit(Fmt("Unknown command %v\n", args[0]))
	}
}
