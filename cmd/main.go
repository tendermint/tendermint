package main

import (
	"flag"
	"fmt"

	"github.com/tendermint/tendermint/config"
)

func main() {

	// Parse config flags
	config.ParseFlags()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Println(`Tendermint

Commands:
    daemon        Run the tendermint node daemon
    gen_account   Generate new account keypair
    gen_validator Generate new validator keypair
	
tendermint --help for command options`)
		return
	}

	switch args[0] {
	case "daemon":
		daemon()
	case "gen_account":
		gen_account()
	case "gen_validator":
		gen_validator()
	}
}
