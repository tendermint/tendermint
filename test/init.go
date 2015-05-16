// Import this in all *_test.go files to initialize ~/.tendermint_test.
// TODO: Reset each time?

package test

import (
	cfg "github.com/tendermint/tendermint/config"
	tmcfg "github.com/tendermint/tendermint/config/tendermint_test"
)

func init() {
	// Creates ~/.tendermint_test/*
	config := tmcfg.GetConfig("")
	cfg.ApplyConfig(config)
}
