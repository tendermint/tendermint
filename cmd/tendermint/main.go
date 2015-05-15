package main

import (
	"fmt"
	"os"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/logger"
	"github.com/tendermint/tendermint/node"
)

func main() {

	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println(`Tendermint

Commands:
    node          Run the tendermint node 
    gen_account   Generate new account keypair
    gen_validator Generate new validator keypair
    gen_tx        Generate new transaction
    probe_upnp    Test UPnP functionality
`)
		return
	}

	switch args[0] {
	case "node":
		config.ParseFlags(args[1:])
		logger.Reset()
		node.RunNode()
	case "gen_account":
		gen_account()
	case "gen_validator":
		gen_validator()
	case "gen_tx":
		config.ParseFlags(args[1:])
		logger.Reset()
		gen_tx()
	case "probe_upnp":
		probe_upnp()
	case "unsafe_reset_priv_validator":
		reset_priv_validator()
	default:
		fmt.Printf("Unknown command %v\n", args[0])
	}
}
