package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	assert := assert.New(t)

	// set up some defaults
	cfg := DefaultFileConfig()
	assert.NotNil(cfg.P2P)
	assert.NotNil(cfg.Mempool)
	assert.NotNil(cfg.Consensus)

	// check the root dir stuff...
	cfg.SetRoot("/foo")
	cfg.Genesis = "bar"
	cfg.DBPath = "/opt/data"

	assert.Equal("/foo/bar", cfg.GenesisFile())
	assert.Equal("/opt/data", cfg.DBDir())
}

func TestBaseConfigValidateBasic(t *testing.T) {
	cfg := TestBaseFileConfig()
	assert.NoError(t, cfg.ValidateBasic())

	// tamper with log format
	cfg.LogFormat = "invalid"
	assert.Error(t, cfg.ValidateBasic())
}