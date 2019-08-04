package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	assert := assert.New(t)

	// set up some defaults
	cfg := DefaultConfig()
	assert.NotNil(cfg.P2P)
	assert.NotNil(cfg.Mempool)
	assert.NotNil(cfg.Consensus)

	// check the root dir stuff...
	cfg.SetRoot("/foo")
	cfg.Genesis = "bar"
	cfg.DBPath = "/opt/data"
	cfg.Mempool.WalPath = "wal/mem/"

	assert.Equal("/foo/bar", cfg.GenesisFile())
	assert.Equal("/opt/data", cfg.DBDir())
	assert.Equal("/foo/wal/mem", cfg.Mempool.WalDir())

}

func TestConfigValidateBasic(t *testing.T) {
	cfg := DefaultConfig()
	assert.NoError(t, cfg.ValidateBasic())

	// tamper with timeout_propose
	cfg.Consensus.TimeoutPropose = -10 * time.Second
	assert.Error(t, cfg.ValidateBasic())
}

func TestTLSConfiguration(t *testing.T) {
	assert := assert.New(t)
	cfg := DefaultConfig()
	cfg.SetRoot("/home/user")

	cfg.RPC.TLSCertFile = "file.crt"
	assert.Equal("/home/user/config/file.crt", cfg.RPC.CertFile())
	cfg.RPC.TLSKeyFile = "file.key"
	assert.Equal("/home/user/config/file.key", cfg.RPC.KeyFile())

	cfg.RPC.TLSCertFile = "/abs/path/to/file.crt"
	assert.Equal("/abs/path/to/file.crt", cfg.RPC.CertFile())
	cfg.RPC.TLSKeyFile = "/abs/path/to/file.key"
	assert.Equal("/abs/path/to/file.key", cfg.RPC.KeyFile())
}

func TestBaseConfigValidateBasic(t *testing.T) {
	cfg := TestBaseConfig()
	assert.NoError(t, cfg.ValidateBasic())

	// tamper with log format
	cfg.LogFormat = "invalid"
	assert.Error(t, cfg.ValidateBasic())
}

func TestRPCConfigValidateBasic(t *testing.T) {
	testCases := []struct {
		testName                     string
		cfgGRPCMaxOpenConnections    int
		cfgMaxOpenConnections        int
		cfgMaxSubscriptionClients    int
		cfgMaxSubscriptionsPerClient int
		cfgTimeoutBroadcastTxCommit  time.Duration
		cfgMaxBodyBytes              int64
		cfgMaxHeaderBytes            int
		expectErr                    bool
	}{
		{"Valid RPC Config", 1, 1, 1, 1, 1, 1, 1, false},
		{"Invalid RPC Config", -1, 1, 1, 1, 1, 1, 1, true},
		{"Invalid RPC Config", 1, -1, 1, 1, 1, 1, 1, true},
		{"Invalid RPC Config", 1, 1, -1, 1, 1, 1, 1, true},
		{"Invalid RPC Config", 1, 1, 1, -1, 1, 1, 1, true},
		{"Invalid RPC Config", 1, 1, 1, 1, -1, 1, 1, true},
		{"Invalid RPC Config", 1, 1, 1, 1, 1, -1, 1, true},
		{"Invalid RPC Config", 1, 1, 1, 1, 1, 1, -1, true},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			cfg := RPCConfig{
				GRPCMaxOpenConnections:    tc.cfgGRPCMaxOpenConnections,
				MaxOpenConnections:        tc.cfgMaxOpenConnections,
				MaxSubscriptionClients:    tc.cfgMaxSubscriptionClients,
				MaxSubscriptionsPerClient: tc.cfgMaxSubscriptionsPerClient,
				TimeoutBroadcastTxCommit:  tc.cfgTimeoutBroadcastTxCommit,
				MaxBodyBytes:              tc.cfgMaxBodyBytes,
				MaxHeaderBytes:            tc.cfgMaxHeaderBytes,
			}
			assert.Equal(t, tc.expectErr, cfg.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestP2PConfigValidateBasic(t *testing.T) {
	testCases := []struct {
		testName                   string
		cfgMaxNumInboundPeers      int
		cfgMaxNumOutboundPeers     int
		cfgFlushThrottleTimeout    time.Duration
		cfgMaxPacketMsgPayloadSize int
		cfgSendRate                int64
		cfgRecvRate                int64
		expectErr                  bool
	}{
		{"Valid RPC Config", 1, 1, 1, 1, 1, 1, false},
		{"Invalid RPC Config", -1, 1, 1, 1, 1, 1, true},
		{"Invalid RPC Config", 1, -1, 1, 1, 1, 1, true},
		{"Invalid RPC Config", 1, 1, -1, 1, 1, 1, true},
		{"Invalid RPC Config", 1, 1, 1, -1, 1, 1, true},
		{"Invalid RPC Config", 1, 1, 1, 1, -1, 1, true},
		{"Invalid RPC Config", 1, 1, 1, 1, 1, -1, true},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			cfg := P2PConfig{
				MaxNumInboundPeers:      tc.cfgMaxNumInboundPeers,
				MaxNumOutboundPeers:     tc.cfgMaxNumOutboundPeers,
				FlushThrottleTimeout:    tc.cfgFlushThrottleTimeout,
				MaxPacketMsgPayloadSize: tc.cfgMaxPacketMsgPayloadSize,
				SendRate:                tc.cfgSendRate,
				RecvRate:                tc.cfgRecvRate,
			}
			assert.Equal(t, tc.expectErr, cfg.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}
