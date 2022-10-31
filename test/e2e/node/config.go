package main

import (
	"errors"
	"fmt"

	"github.com/BurntSushi/toml"

	"github.com/tendermint/tendermint/test/e2e/app"
)

// Config is the application configuration.
type Config struct {
	ChainID          string `toml:"chain_id"`
	Listen           string
	Protocol         string
	Dir              string
	Mode             string                      `toml:"mode"`
	PersistInterval  uint64                      `toml:"persist_interval"`
	SnapshotInterval uint64                      `toml:"snapshot_interval"`
	RetainBlocks     uint64                      `toml:"retain_blocks"`
	ValidatorUpdates map[string]map[string]uint8 `toml:"validator_update"`
	PrivValServer    string                      `toml:"privval_server"`
	PrivValKey       string                      `toml:"privval_key"`
	PrivValState     string                      `toml:"privval_state"`
	Misbehaviors     map[string]string           `toml:"misbehaviors"`
	KeyType          string                      `toml:"key_type"`
}

// App extracts out the application specific configuration parameters
func (cfg *Config) App() *app.Config {
	return &app.Config{
		Dir:              cfg.Dir,
		SnapshotInterval: cfg.SnapshotInterval,
		RetainBlocks:     cfg.RetainBlocks,
		KeyType:          cfg.KeyType,
		ValidatorUpdates: cfg.ValidatorUpdates,
		PersistInterval:  cfg.PersistInterval,
	}
}

// LoadConfig loads the configuration from disk.
func LoadConfig(file string) (*Config, error) {
	cfg := &Config{
		Listen:          "unix:///var/run/app.sock",
		Protocol:        "socket",
		PersistInterval: 1,
	}
	_, err := toml.DecodeFile(file, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from %q: %w", file, err)
	}
	return cfg, cfg.Validate()
}

// Validate validates the configuration. We don't do exhaustive config
// validation here, instead relying on Testnet.Validate() to handle it.
//
//nolint:goconst
func (cfg Config) Validate() error {
	switch {
	case cfg.ChainID == "":
		return errors.New("chain_id parameter is required")
	case cfg.Listen == "" && cfg.Protocol != "builtin":
		return errors.New("listen parameter is required")
	default:
		return nil
	}
}
