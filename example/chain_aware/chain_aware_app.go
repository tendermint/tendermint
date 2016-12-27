package main

import (
	"flag"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tmsp/server"
	"github.com/tendermint/tmsp/types"
)

func main() {

	addrPtr := flag.String("addr", "tcp://0.0.0.0:46658", "Listen address")
	tmspPtr := flag.String("tmsp", "socket", "socket | grpc")
	flag.Parse()

	// Start the listener
	srv, err := server.NewServer(*addrPtr, *tmspPtr, NewChainAwareApplication())
	if err != nil {
		Exit(err.Error())
	}

	// Wait forever
	TrapSignal(func() {
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

func (app *ChainAwareApplication) AppendTx(tx []byte) types.Result {
	return types.NewResultOK(nil, "")
}

func (app *ChainAwareApplication) CheckTx(tx []byte) types.Result {
	return types.NewResultOK(nil, "")
}

func (app *ChainAwareApplication) Commit() types.Result {
	return types.NewResultOK([]byte("nil"), "")
}

func (app *ChainAwareApplication) Query(query []byte) types.Result {
	return types.NewResultOK([]byte(Fmt("%d,%d", app.beginCount, app.endCount)), "")
}

func (app *ChainAwareApplication) BeginBlock(hash []byte, header *types.Header) {
	app.beginCount += 1
	return
}

func (app *ChainAwareApplication) EndBlock(height uint64) (resEndBlock types.ResponseEndBlock) {
	app.endCount += 1
	return
}

func (app *ChainAwareApplication) InitChain(vals []*types.Validator) {
	return
}
