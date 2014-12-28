package main

import (
	"fmt"
	"os"

	"github.com/tendermint/tendermint/config"
)

func main() {

	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println(`Tendermint

Commands:
    daemon        Run the tendermint node daemon
    gen_account   Generate new account keypair
    gen_validator Generate new validator keypair
    probe_upnp    Test UPnP functionality
`)
		return
	}

	switch args[0] {
	case "daemon":
		config.ParseFlags(args[1:])
		daemon()
	case "gen_account":
		gen_account()
	case "gen_validator":
		gen_validator()
	case "probe_upnp":
		probe_upnp()
	default:
		fmt.Println("Unknown command %v", args[0])
	}
}
