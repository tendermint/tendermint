package main

import (
	"flag"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tmsp/example/golang"
	"github.com/tendermint/tmsp/server"
)

func main() {

	addrPtr := flag.String("addr", "tcp://0.0.0.0:46658", "Listen address")
	flag.Parse()

	// Start the listener
	_, err := server.StartListener(*addrPtr, example.NewDummyApplication())
	if err != nil {
		Exit(err.Error())
	}

	// Wait forever
	TrapSignal(func() {
		// Cleanup
	})

}
