package main

import (
	"flag"
	stdlog "log"
	"os"

	"github.com/tendermint/abci/example/dummy"
	"github.com/tendermint/abci/server"
	"github.com/tendermint/abci/types"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"
)

func main() {

	addrPtr := flag.String("addr", "tcp://0.0.0.0:46658", "Listen address")
	abciPtr := flag.String("abci", "socket", "socket | grpc")
	persistencePtr := flag.String("persist", "", "directory to use for a database")
	flag.Parse()

	logger := log.NewTmLogger(os.Stdout)

	// Create the application - in memory or persisted to disk
	var app types.Application
	if *persistencePtr == "" {
		app = dummy.NewDummyApplication()
	} else {
		app = dummy.NewPersistentDummyApplication(*persistencePtr)
		app.(*dummy.PersistentDummyApplication).SetLogger(log.With(logger, "module", "dummy"))
	}

	// Start the listener
	srv, err := server.NewServer(*addrPtr, *abciPtr, app)
	if err != nil {
		stdlog.Fatal(err.Error())
	}
	srv.SetLogger(log.With(logger, "module", "abci-server"))

	// Wait forever
	cmn.TrapSignal(func() {
		// Cleanup
		srv.Stop()
	})

}
