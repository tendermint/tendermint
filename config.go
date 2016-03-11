package p2p

import (
	cfg "github.com/tendermint/go-config"
)

var config cfg.Config = nil

func init() {
	initConfigureable(dialTimeoutKey, 3)
	initConfigureable(handshakeTimeoutKey, 20)
	initConfigureable(maxNumPeersKey, 50)
	initConfigureable(sendRateKey, 512000) // 500KB/s
	initConfigureable(recvRateKey, 512000) // 500KB/s
	initConfigureable(maxPayloadSizeKey, 1024)

	initConfigureable(authEncKey, true)

	cfg.OnConfig(func(newConfig cfg.Config) {
		config = newConfig

		// fill in any config values that might be missing
		for key, value := range defaultConfigValues {
			config.SetDefault(key, value)
		}
	})
}

// default config map
var defaultConfigValues = make(map[string]interface{})

func initConfigureable(key string, value interface{}) {
	defaultConfigValues[key] = value
}
