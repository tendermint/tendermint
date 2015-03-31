package config

import (
	// We can't use github.com/tendermint/tendermint2/logger
	// because that would create a dependency cycle.
	"github.com/tendermint/log15"
)

var log = log15.New("module", "config")
