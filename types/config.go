package types

import (
	cfg "github.com/eris-ltd/tendermint/config"
)

var config cfg.Config = nil

func init() {
	cfg.OnConfig(func(newConfig cfg.Config) {
		config = newConfig
	})
}
