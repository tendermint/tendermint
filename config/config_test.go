package config

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	// set up some defaults
	cfg := DefaultConfig()
	assert.NotNil(t, cfg.P2P)
	assert.NotNil(t, cfg.Mempool)
	assert.NotNil(t, cfg.Consensus)

	// check the root dir stuff...
	cfg.SetRoot("/foo")
	cfg.Genesis = "bar"
	cfg.DBPath = "/opt/data"

	assert.Equal(t, "/foo/bar", cfg.GenesisFile())
	assert.Equal(t, "/opt/data", cfg.DBDir())
}

func TestConfigValidateBasic(t *testing.T) {
	cfg := DefaultConfig()
	assert.NoError(t, cfg.ValidateBasic())

	// tamper with timeout_propose
	cfg.Consensus.TimeoutPropose = -10 * time.Second
	assert.Error(t, cfg.ValidateBasic())
}

func TestTLSConfiguration(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SetRoot("/home/user")

	cfg.RPC.TLSCertFile = "file.crt"
	assert.Equal(t, "/home/user/config/file.crt", cfg.RPC.CertFile())
	cfg.RPC.TLSKeyFile = "file.key"
	assert.Equal(t, "/home/user/config/file.key", cfg.RPC.KeyFile())

	cfg.RPC.TLSCertFile = "/abs/path/to/file.crt"
	assert.Equal(t, "/abs/path/to/file.crt", cfg.RPC.CertFile())
	cfg.RPC.TLSKeyFile = "/abs/path/to/file.key"
	assert.Equal(t, "/abs/path/to/file.key", cfg.RPC.KeyFile())
}

func TestBaseConfigValidateBasic(t *testing.T) {
	cfg := TestBaseConfig()
	assert.NoError(t, cfg.ValidateBasic())

	// tamper with log format
	cfg.LogFormat = "invalid"
	assert.Error(t, cfg.ValidateBasic())
}

func TestRPCConfigValidateBasic(t *testing.T) {
	cfg := TestRPCConfig()
	assert.NoError(t, cfg.ValidateBasic())

	fieldsToTest := []string{
		"MaxOpenConnections",
		"MaxSubscriptionClients",
		"MaxSubscriptionsPerClient",
		"TimeoutBroadcastTxCommit",
		"MaxBodyBytes",
		"MaxHeaderBytes",
	}

	for _, fieldName := range fieldsToTest {
		reflect.ValueOf(cfg).Elem().FieldByName(fieldName).SetInt(-1)
		assert.Error(t, cfg.ValidateBasic())
		reflect.ValueOf(cfg).Elem().FieldByName(fieldName).SetInt(0)
	}
}

func TestMempoolConfigValidateBasic(t *testing.T) {
	cfg := TestMempoolConfig()
	assert.NoError(t, cfg.ValidateBasic())

	fieldsToTest := []string{
		"Size",
		"MaxTxsBytes",
		"CacheSize",
		"MaxTxBytes",
	}

	for _, fieldName := range fieldsToTest {
		reflect.ValueOf(cfg).Elem().FieldByName(fieldName).SetInt(-1)
		assert.Error(t, cfg.ValidateBasic())
		reflect.ValueOf(cfg).Elem().FieldByName(fieldName).SetInt(0)
	}
}

func TestStateSyncConfigValidateBasic(t *testing.T) {
	cfg := TestStateSyncConfig()
	require.NoError(t, cfg.ValidateBasic())
}

func TestConsensusConfig_ValidateBasic(t *testing.T) {
	testcases := map[string]struct {
		modify    func(*ConsensusConfig)
		expectErr bool
	}{
		"TimeoutPropose":                       {func(c *ConsensusConfig) { c.TimeoutPropose = time.Second }, false},
		"TimeoutPropose negative":              {func(c *ConsensusConfig) { c.TimeoutPropose = -1 }, true},
		"TimeoutProposeDelta":                  {func(c *ConsensusConfig) { c.TimeoutProposeDelta = time.Second }, false},
		"TimeoutProposeDelta negative":         {func(c *ConsensusConfig) { c.TimeoutProposeDelta = -1 }, true},
		"TimeoutPrevote":                       {func(c *ConsensusConfig) { c.TimeoutPrevote = time.Second }, false},
		"TimeoutPrevote negative":              {func(c *ConsensusConfig) { c.TimeoutPrevote = -1 }, true},
		"TimeoutPrevoteDelta":                  {func(c *ConsensusConfig) { c.TimeoutPrevoteDelta = time.Second }, false},
		"TimeoutPrevoteDelta negative":         {func(c *ConsensusConfig) { c.TimeoutPrevoteDelta = -1 }, true},
		"TimeoutPrecommit":                     {func(c *ConsensusConfig) { c.TimeoutPrecommit = time.Second }, false},
		"TimeoutPrecommit negative":            {func(c *ConsensusConfig) { c.TimeoutPrecommit = -1 }, true},
		"TimeoutPrecommitDelta":                {func(c *ConsensusConfig) { c.TimeoutPrecommitDelta = time.Second }, false},
		"TimeoutPrecommitDelta negative":       {func(c *ConsensusConfig) { c.TimeoutPrecommitDelta = -1 }, true},
		"TimeoutCommit":                        {func(c *ConsensusConfig) { c.TimeoutCommit = time.Second }, false},
		"TimeoutCommit negative":               {func(c *ConsensusConfig) { c.TimeoutCommit = -1 }, true},
		"PeerGossipSleepDuration":              {func(c *ConsensusConfig) { c.PeerGossipSleepDuration = time.Second }, false},
		"PeerGossipSleepDuration negative":     {func(c *ConsensusConfig) { c.PeerGossipSleepDuration = -1 }, true},
		"PeerQueryMaj23SleepDuration":          {func(c *ConsensusConfig) { c.PeerQueryMaj23SleepDuration = time.Second }, false},
		"PeerQueryMaj23SleepDuration negative": {func(c *ConsensusConfig) { c.PeerQueryMaj23SleepDuration = -1 }, true},
		"DoubleSignCheckHeight negative":       {func(c *ConsensusConfig) { c.DoubleSignCheckHeight = -1 }, true},
	}
	for desc, tc := range testcases {
		tc := tc // appease linter
		t.Run(desc, func(t *testing.T) {
			cfg := DefaultConsensusConfig()
			tc.modify(cfg)

			err := cfg.ValidateBasic()
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestInstrumentationConfigValidateBasic(t *testing.T) {
	cfg := TestInstrumentationConfig()
	assert.NoError(t, cfg.ValidateBasic())

	// tamper with maximum open connections
	cfg.MaxOpenConnections = -1
	assert.Error(t, cfg.ValidateBasic())
}

func TestP2PConfigValidateBasic(t *testing.T) {
	cfg := TestP2PConfig()
	assert.NoError(t, cfg.ValidateBasic())

	fieldsToTest := []string{
		"FlushThrottleTimeout",
		"MaxPacketMsgPayloadSize",
		"SendRate",
		"RecvRate",
	}

	for _, fieldName := range fieldsToTest {
		reflect.ValueOf(cfg).Elem().FieldByName(fieldName).SetInt(-1)
		assert.Error(t, cfg.ValidateBasic())
		reflect.ValueOf(cfg).Elem().FieldByName(fieldName).SetInt(0)
	}
}
