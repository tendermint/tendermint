package p2p

import (
	cfg "github.com/tendermint/go-config"
)

// XXX: go-p2p requires ApplyConfig be called
var config cfg.Config = nil

func init() {
	initConfigureable(dialTimeoutKey, 3)
	initConfigureable(handshakeTimeoutKey, 20)
	initConfigureable(maxNumPeersKey, 50)

	initConfigureable(sendRateKey, 512000) // 500KB/s
	initConfigureable(recvRateKey, 512000) // 500KB/s

	initConfigureable(maxPayloadSizeKey, 1024)

	cfg.OnConfig(func(newConfig cfg.Config) {
		config = newConfig

		// fill in any config values that might be missing
		for key, value := range defaultConfigValues {
			if !config.IsSet(key) {
				config.Set(key, value)
			}
		}
	})
	c := cfg.NewMapConfig(nil)
	c.Set("log_level", "debug")
	cfg.ApplyConfig(c)
}

// default config map
var defaultConfigValues = make(map[string]int)

func initConfigureable(key string, value int) {
	defaultConfigValues[key] = value
}
