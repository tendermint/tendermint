package main

import (
	"fmt"
	"os"

	cfg "github.com/tendermint/go-config"
	"github.com/tendermint/go-logger"
	tmcfg "github.com/tendermint/tendermint/config/tendermint"
	"github.com/tendermint/tendermint/consensus"
	"github.com/tendermint/tendermint/version"
)

var config cfg.Config

func main() {

	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println(`Tendermint

Commands:
    init                        Initialize tendermint
    node                        Run the tendermint node
    show_validator              Show this node's validator info
    gen_validator               Generate new validator keypair
    probe_upnp                  Test UPnP functionality
    replay <walfile>            Replay messages from WAL
    replay_console <walfile>    Replay messages from WAL in a console
    unsafe_reset_all            (unsafe) Remove all the data and WAL, reset this node's validator
    unsafe_reset_priv_validator (unsafe) Reset this node's validator
    version                     Show version info
`)
		return
	}

	// Get configuration
	config = tmcfg.GetConfig("")
	parseFlags(config, args[1:]) // Command line overrides

	// set the log level
	logger.SetLogLevel(config.GetString("log_level"))

	switch args[0] {
	case "node":
		run_node(config)
	case "replay":
		if len(args) > 1 {
			consensus.RunReplayFile(config, args[1], false)
		} else {
			fmt.Println("replay requires an argument (walfile)")
			os.Exit(1)
		}
	case "replay_console":
		if len(args) > 1 {
			consensus.RunReplayFile(config, args[1], true)
		} else {
			fmt.Println("replay_console requires an argument (walfile)")
			os.Exit(1)
		}
	case "init":
		init_files()
	case "show_validator":
		show_validator()
	case "gen_validator":
		gen_validator()
	case "probe_upnp":
		probe_upnp()
	case "unsafe_reset_all":
		reset_all()
	case "unsafe_reset_priv_validator":
		reset_priv_validator()
	case "version":
		fmt.Println(version.Version)
	default:
		fmt.Printf("Unknown command %v\n", args[0])
		os.Exit(1)
	}
}
