package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/tendermint/abci/server"
	"github.com/tendermint/abci/types"
	cmn "github.com/tendermint/go-common"
)

func main() {

	addrPtr := flag.String("addr", "tcp://0.0.0.0:46658", "Listen address")
	abciPtr := flag.String("abci", "socket", "socket | grpc")
	flag.Parse()

	// Start the listener
	srv, err := server.NewServer(*addrPtr, *abciPtr, NewChainAwareApplication())
	if err != nil {
		log.Fatal(err.Error())
	}

	// Wait forever
	cmn.TrapSignal(func() {
		// Cleanup
		srv.Stop()
	})

}

type ChainAwareApplication struct {
	beginCount int
	endCount   int
}

func NewChainAwareApplication() *ChainAwareApplication {
	return &ChainAwareApplication{}
}

func (app *ChainAwareApplication) Info() types.ResponseInfo {
	return types.ResponseInfo{}
}

func (app *ChainAwareApplication) SetOption(key string, value string) (log string) {
	return ""
}

func (app *ChainAwareApplication) DeliverTx(tx []byte) types.Result {
	return types.NewResultOK(nil, "")
}

func (app *ChainAwareApplication) CheckTx(tx []byte) types.Result {
	return types.NewResultOK(nil, "")
}

func (app *ChainAwareApplication) Commit() types.Result {
	return types.NewResultOK([]byte("nil"), "")
}

func (app *ChainAwareApplication) Query(query []byte) types.Result {
	return types.NewResultOK([]byte(fmt.Sprintf("%d,%d", app.beginCount, app.endCount)), "")
}

func (app *ChainAwareApplication) BeginBlock(hash []byte, header *types.Header) {
	app.beginCount++
	return
}

func (app *ChainAwareApplication) EndBlock(height uint64) (resEndBlock types.ResponseEndBlock) {
	app.endCount++
	return
}

func (app *ChainAwareApplication) InitChain(vals []*types.Validator) {
	return
}
