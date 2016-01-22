package types

import (
	cfg "github.com/tendermint/netmon/Godeps/_workspace/src/github.com/tendermint/go-config"
)

var config cfg.Config = nil

func init() {
	cfg.OnConfig(func(newConfig cfg.Config) {
		config = newConfig
	})
}
