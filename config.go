package p2p

import (
	cfg "github.com/tendermint/go-config"
)

const (
	// Switch config keys
	configKeyDialTimeoutSeconds      = "p2p_dial_timeout_seconds"
	configKeyHandshakeTimeoutSeconds = "p2p_handshake_timeout_seconds"
	configKeyMaxNumPeers             = "p2p_max_num_peers"
	configKeyAuthEnc                 = "p2p_authenticated_encryption"

	// MConnection config keys
	configKeySendRate = "p2p_send_rate"
	configKeyRecvRate = "p2p_recv_rate"
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
