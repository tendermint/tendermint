package logger

import (
	cfg "github.com/tendermint/go-config"
)

var config cfg.Config = nil

func init() {
	cfg.OnConfig(func(newConfig cfg.Config) {
		config = newConfig
		Reset() // reset log root upon config change.
	})
}
