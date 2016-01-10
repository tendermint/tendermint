package main

import (
	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	tmcfg "github.com/tendermint/tendermint/config/tendermint"
	"github.com/tendermint/tendermint/types"
)

func init() {

	config := tmcfg.GetConfig("")
	config.Set("log_level", "debug")
	cfg.ApplyConfig(config) // Notify modules of new config
}

func main() {
	em := NewEventMeter("ws://localhost:46657/websocket")
	if _, err := em.Start(); err != nil {
		Exit(err.Error())
	}
	if err := em.Subscribe(types.EventStringNewBlock()); err != nil {
		Exit(err.Error())
	}
	TrapSignal(func() {
		em.Stop()
	})

}
