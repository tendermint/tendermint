package main

import (
	"flag"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/abci/example/dummy"
	"github.com/tendermint/abci/server"
	"github.com/tendermint/abci/types"
)

func main() {

	addrPtr := flag.String("addr", "tcp://0.0.0.0:46658", "Listen address")
	abciPtr := flag.String("abci", "socket", "socket | grpc")
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
	srv, err := server.NewServer(*addrPtr, *abciPtr, app)
	if err != nil {
		Exit(err.Error())
	}

	// Wait forever
	TrapSignal(func() {
		// Cleanup
		srv.Stop()
	})

}
