package main

import (
	"flag"
	"os"

	"github.com/tendermint/abci/server"
	"github.com/tendermint/abci/types"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"
)

func main() {

	addrPtr := flag.String("addr", "tcp://0.0.0.0:46658", "Listen address")
	abciPtr := flag.String("abci", "socket", "socket | grpc")
	flag.Parse()

	logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout))

	// Start the listener
	srv, err := server.NewServer(*addrPtr, *abciPtr, NewChainAwareApplication())
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	srv.SetLogger(logger.With("module", "abci-server"))
	if _, err := srv.Start(); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	// Wait forever
	cmn.TrapSignal(func() {
		// Cleanup
		srv.Stop()
	})

}

type ChainAwareApplication struct {
	types.BaseApplication

	beginCount int
	endCount   int
}

func NewChainAwareApplication() *ChainAwareApplication {
	return &ChainAwareApplication{}
}

func (app *ChainAwareApplication) Query(reqQuery types.RequestQuery) (resQuery types.ResponseQuery) {
	return types.ResponseQuery{
		Value: []byte(cmn.Fmt("%d,%d", app.beginCount, app.endCount)),
	}
}

func (app *ChainAwareApplication) BeginBlock(reqBeginBlock types.RequestBeginBlock) {
	app.beginCount++
}

func (app *ChainAwareApplication) EndBlock(height uint64) (resEndBlock types.ResponseEndBlock) {
	app.endCount++
	return
}
