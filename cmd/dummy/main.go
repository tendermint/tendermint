package main

import (
	. "github.com/tendermint/go-common"
	"github.com/tendermint/tmsp/example"
	"github.com/tendermint/tmsp/server"
)

func main() {

	// Start the listener
	_, err := server.StartListener("tcp://127.0.0.1:8080", example.NewDummyApplication())
	if err != nil {
		Exit(err.Error())
	}

	// Wait forever
	TrapSignal(func() {
		// Cleanup
	})

}
