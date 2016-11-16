package main

import (
	"flag"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tmsp/example/dummy"
	"github.com/tendermint/tmsp/server"
	"github.com/tendermint/tmsp/types"
)

func main() {

	addrPtr := flag.String("addr", "tcp://0.0.0.0:46658", "Listen address")
	tmspPtr := flag.String("tmsp", "socket", "socket | grpc")
	persistencePtr := flag.String("persist", "", "directory to use for a database")
	flag.Parse()

	// Create the application - in memory or persisted to disk
	var app types.Application
	if *persistencePtr == "" {
		app = dummy.NewDummyApplication()
	} else {
		app = dummy.NewPersistentDummyApplication(*persistencePtr)
	}

	// Start the listener
	_, err := server.NewServer(*addrPtr, *tmspPtr, app)
	if err != nil {
		Exit(err.Error())
	}

	// Wait forever
	TrapSignal(func() {
		// Cleanup
	})

}
