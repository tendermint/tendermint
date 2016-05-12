package p2p

import (
	cfg "github.com/tendermint/go-config"
)

const (
	// Switch config keys
	configKeyDialTimeoutSeconds      = "dial_timeout_seconds"
	configKeyHandshakeTimeoutSeconds = "handshake_timeout_seconds"
	configKeyMaxNumPeers             = "max_num_peers"
	configKeyAuthEnc                 = "authenticated_encryption"

	// MConnection config keys
	configKeySendRate = "send_rate"
	configKeyRecvRate = "recv_rate"
)

func setConfigDefaults(config cfg.Config) {
	// Switch default config
	config.SetDefault(configKeyDialTimeoutSeconds, 3)
	config.SetDefault(configKeyHandshakeTimeoutSeconds, 20)
	config.SetDefault(configKeyMaxNumPeers, 50)
	config.SetDefault(configKeyAuthEnc, true)

	// MConnection default config
	config.SetDefault(configKeySendRate, 512000) // 500KB/s
	config.SetDefault(configKeyRecvRate, 512000) // 500KB/s
}
