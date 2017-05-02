package p2p

import (
	"github.com/spf13/viper"
)

// for node.Config
type NetworkConfig struct {
	ListenAddress  string `mapstructure:"laddr"`
	Seeds          string `mapstructure:"seeds"`
	SkipUPNP       bool   `mapstructure:"skip_upnp"`
	AddrBookFile   string `mapstructure:"addr_book_file"`
	AddrBookStrict bool   `mapstructure:"addr_book_strict"`
	PexReactor     bool   `mapstructure:"pex_reactor"`
}

func NewDefaultConfig(rootDir string) *NetworkConfig {
	return &NetworkConfig{
		AddrBookFile:   rootDir + "/addrbook.json",
		AddrBookStrict: true,
	}
}

const (
	// Switch config keys
	configKeyDialTimeoutSeconds      = "dial_timeout_seconds"
	configKeyHandshakeTimeoutSeconds = "handshake_timeout_seconds"
	configKeyMaxNumPeers             = "max_num_peers"
	configKeyAuthEnc                 = "authenticated_encryption"

	// MConnection config keys
	configKeySendRate = "send_rate"
	configKeyRecvRate = "recv_rate"

	// Fuzz params
	configFuzzEnable               = "fuzz_enable" // use the fuzz wrapped conn
	configFuzzMode                 = "fuzz_mode"   // eg. drop, delay
	configFuzzMaxDelayMilliseconds = "fuzz_max_delay_milliseconds"
	configFuzzProbDropRW           = "fuzz_prob_drop_rw"
	configFuzzProbDropConn         = "fuzz_prob_drop_conn"
	configFuzzProbSleep            = "fuzz_prob_sleep"
)

func setConfigDefaults(config *viper.Viper) {
	// Switch default config
	config.SetDefault(configKeyDialTimeoutSeconds, 3)
	config.SetDefault(configKeyHandshakeTimeoutSeconds, 20)
	config.SetDefault(configKeyMaxNumPeers, 50)
	config.SetDefault(configKeyAuthEnc, true)

	// MConnection default config
	config.SetDefault(configKeySendRate, 512000) // 500KB/s
	config.SetDefault(configKeyRecvRate, 512000) // 500KB/s

	// Fuzz defaults
	config.SetDefault(configFuzzEnable, false)
	config.SetDefault(configFuzzMode, FuzzModeDrop)
	config.SetDefault(configFuzzMaxDelayMilliseconds, 3000)
	config.SetDefault(configFuzzProbDropRW, 0.2)
	config.SetDefault(configFuzzProbDropConn, 0.00)
	config.SetDefault(configFuzzProbSleep, 0.00)
}
