package main

import (
	"flag"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tmsp/example"
	"github.com/tendermint/tmsp/server"
)

func main() {

	serialPtr := flag.Bool("serial", false, "Enforce incrementing (serial) txs")
	flag.Parse()
	app := example.NewCounterApplication(*serialPtr)

	// Start the listener
	_, err := server.StartListener("tcp://0.0.0.0:46658", app)
	if err != nil {
		Exit(err.Error())
	}

	// Wait forever
	TrapSignal(func() {
		// Cleanup
	})

}
