package main

import (
	"flag"
	stdlog "log"
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

	// Start the listener
	srv, err := server.NewServer(*addrPtr, *abciPtr, NewChainAwareApplication())
	if err != nil {
		stdlog.Fatal(err.Error())
	}
	logger := log.NewTmLogger(os.Stdout)
	srv.SetLogger(log.With(logger, "module", "abci-server"))

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

func (app *ChainAwareApplication) BeginBlock(hash []byte, header *types.Header) {
	app.beginCount++
	return
}

func (app *ChainAwareApplication) EndBlock(height uint64) (resEndBlock types.ResponseEndBlock) {
	app.endCount++
	return
}
