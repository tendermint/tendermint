package rpctest

import (
	cfg "github.com/tendermint/tendermint/config"
	tmcfg "github.com/tendermint/tendermint/config/tendermint_test"
)

var config cfg.Config = nil

func initConfig() {

	cfg.OnConfig(func(newConfig cfg.Config) {
		config = newConfig
	})

	c := tmcfg.GetConfig("")
	cfg.ApplyConfig(c) // Notify modules of new config
}
