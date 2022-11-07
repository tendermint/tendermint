package main

import (
	"errors"
	"fmt"

	"github.com/BurntSushi/toml"

	"github.com/tendermint/tendermint/test/e2e/app"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

// Config is the application configuration.
type Config struct {
	ChainID          string                      `toml:"chain_id"`
	Listen           string                      `toml:"listen"`
	Protocol         string                      `toml:"protocol"`
	Dir              string                      `toml:"dir"`
	Mode             string                      `toml:"mode"`
	SyncApp          bool                        `toml:"sync_app"`
	PersistInterval  uint64                      `toml:"persist_interval"`
	SnapshotInterval uint64                      `toml:"snapshot_interval"`
	RetainBlocks     uint64                      `toml:"retain_blocks"`
	ValidatorUpdates map[string]map[string]uint8 `toml:"validator_update"`
	PrivValServer    string                      `toml:"privval_server"`
	PrivValKey       string                      `toml:"privval_key"`
	PrivValState     string                      `toml:"privval_state"`
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
	case cfg.SyncApp && cfg.Protocol != string(e2e.ProtocolBuiltin):
		return errors.New("sync_app parameter is only relevant for builtin applications")
	case cfg.SyncApp && cfg.Mode != string(e2e.ModeFull) && cfg.Mode != string(e2e.ModeValidator):
		return errors.New("sync_app parameter is only relevant to full nodes and validators")
	default:
		return nil
	}
}
