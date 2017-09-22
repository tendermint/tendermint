package config

import (
	"fmt"
	"path/filepath"
	"time"
)

// Config defines the top level configuration for a Tendermint node
type Config struct {
	// Top level options use an anonymous struct
	BaseConfig `mapstructure:",squash"`

	// Options for services
	RPC       *RPCConfig       `mapstructure:"rpc"`
	P2P       *P2PConfig       `mapstructure:"p2p"`
	Mempool   *MempoolConfig   `mapstructure:"mempool"`
	Consensus *ConsensusConfig `mapstructure:"consensus"`
}

// DefaultConfig returns a default configuration for a Tendermint node
func DefaultConfig() *Config {
	return &Config{
		BaseConfig: DefaultBaseConfig(),
		RPC:        DefaultRPCConfig(),
		P2P:        DefaultP2PConfig(),
		Mempool:    DefaultMempoolConfig(),
		Consensus:  DefaultConsensusConfig(),
	}
}

// TestConfig returns a configuration that can be used for testing
func TestConfig() *Config {
	return &Config{
		BaseConfig: TestBaseConfig(),
		RPC:        TestRPCConfig(),
		P2P:        TestP2PConfig(),
		Mempool:    DefaultMempoolConfig(),
		Consensus:  TestConsensusConfig(),
	}
}

// SetRoot sets the RootDir for all Config structs
func (cfg *Config) SetRoot(root string) *Config {
	cfg.BaseConfig.RootDir = root
	cfg.RPC.RootDir = root
	cfg.P2P.RootDir = root
	cfg.Mempool.RootDir = root
	cfg.Consensus.RootDir = root
	return cfg
}

//-----------------------------------------------------------------------------
// BaseConfig

// BaseConfig defines the base configuration for a Tendermint node
type BaseConfig struct {
	// The root directory for all data.
	// This should be set in viper so it can unmarshal into this struct
	RootDir string `mapstructure:"home"`

	// The ID of the chain to join (should be signed with every transaction and vote)
	ChainID string `mapstructure:"chain_id"`

	// A JSON file containing the initial validator set and other meta data
	Genesis string `mapstructure:"genesis_file"`

	// A JSON file containing the private key to use as a validator in the consensus protocol
	PrivValidator string `mapstructure:"priv_validator_file"`

	// A custom human readable name for this node
	Moniker string `mapstructure:"moniker"`

	// TCP or UNIX socket address of the ABCI application,
	// or the name of an ABCI application compiled in with the Tendermint binary
	ProxyApp string `mapstructure:"proxy_app"`

	// Mechanism to connect to the ABCI application: socket | grpc
	ABCI string `mapstructure:"abci"`

	// Output level for logging
	LogLevel string `mapstructure:"log_level"`

	// TCP or UNIX socket address for the profiling server to listen on
	ProfListenAddress string `mapstructure:"prof_laddr"`

	// If this node is many blocks behind the tip of the chain, FastSync
	// allows them to catchup quickly by downloading blocks in parallel
	// and verifying their commits
	FastSync bool `mapstructure:"fast_sync"`

	// If true, query the ABCI app on connecting to a new peer
	// so the app can decide if we should keep the connection or not
	FilterPeers bool `mapstructure:"filter_peers"` // false

	// What indexer to use for transactions
	TxIndex string `mapstructure:"tx_index"`

	// Database backend: leveldb | memdb
	DBBackend string `mapstructure:"db_backend"`

	// Database directory
	DBPath string `mapstructure:"db_dir"`
}

// DefaultBaseConfig returns a default base configuration for a Tendermint node
func DefaultBaseConfig() BaseConfig {
	return BaseConfig{
		Genesis:           "genesis.json",
		PrivValidator:     "priv_validator.json",
		Moniker:           "anonymous",
		ProxyApp:          "tcp://127.0.0.1:46658",
		ABCI:              "socket",
		LogLevel:          DefaultPackageLogLevels(),
		ProfListenAddress: "",
		FastSync:          true,
		FilterPeers:       false,
		TxIndex:           "kv",
		DBBackend:         "leveldb",
		DBPath:            "data",
	}
}

// TestBaseConfig returns a base configuration for testing a Tendermint node
func TestBaseConfig() BaseConfig {
	conf := DefaultBaseConfig()
	conf.ChainID = "tendermint_test"
	conf.ProxyApp = "dummy"
	conf.FastSync = false
	conf.DBBackend = "memdb"
	return conf
}

// GenesisFile returns the full path to the genesis.json file
func (b BaseConfig) GenesisFile() string {
	return rootify(b.Genesis, b.RootDir)
}

// PrivValidatorFile returns the full path to the priv_validator.json file
func (b BaseConfig) PrivValidatorFile() string {
	return rootify(b.PrivValidator, b.RootDir)
}

// DBDir returns the full path to the database directory
func (b BaseConfig) DBDir() string {
	return rootify(b.DBPath, b.RootDir)
}

// DefaultLogLevel returns a default log level of "error"
func DefaultLogLevel() string {
	return "error"
}

// DefaultPackageLogLevels returns a default log level setting so all packages log at "error", while the `state` package logs at "info"
func DefaultPackageLogLevels() string {
	return fmt.Sprintf("state:info,*:%s", DefaultLogLevel())
}

//-----------------------------------------------------------------------------
// RPCConfig

// RPCConfig defines the configuration options for the Tendermint RPC server
type RPCConfig struct {
	RootDir string `mapstructure:"home"`

	// TCP or UNIX socket address for the RPC server to listen on
	ListenAddress string `mapstructure:"laddr"`

	// TCP or UNIX socket address for the gRPC server to listen on
	// NOTE: This server only supports /broadcast_tx_commit
	GRPCListenAddress string `mapstructure:"grpc_laddr"`

	// Activate unsafe RPC commands like /dial_seeds and /unsafe_flush_mempool
	Unsafe bool `mapstructure:"unsafe"`
}

// DefaultRPCConfig returns a default configuration for the RPC server
func DefaultRPCConfig() *RPCConfig {
	return &RPCConfig{
		ListenAddress:     "tcp://0.0.0.0:46657",
		GRPCListenAddress: "",
		Unsafe:            false,
	}
}

// TestRPCConfig returns a configuration for testing the RPC server
func TestRPCConfig() *RPCConfig {
	conf := DefaultRPCConfig()
	conf.ListenAddress = "tcp://0.0.0.0:36657"
	conf.GRPCListenAddress = "tcp://0.0.0.0:36658"
	conf.Unsafe = true
	return conf
}

//-----------------------------------------------------------------------------
// P2PConfig

// P2PConfig defines the configuration options for the Tendermint peer-to-peer networking layer
type P2PConfig struct {
	RootDir string `mapstructure:"home"`

	// Address to listen for incoming connections
	ListenAddress string `mapstructure:"laddr"`

	// Comma separated list of seed nodes to connect to
	Seeds string `mapstructure:"seeds"`

	// Skip UPNP port forwarding
	SkipUPNP bool `mapstructure:"skip_upnp"`

	// Path to address book
	AddrBook string `mapstructure:"addr_book_file"`

	// Set true for strict address routability rules
	AddrBookStrict bool `mapstructure:"addr_book_strict"`

	// Set true to enable the peer-exchange reactor
	PexReactor bool `mapstructure:"pex"`

	// Maximum number of peers to connect to
	MaxNumPeers int `mapstructure:"max_num_peers"`

	// Time to wait before flushing messages out on the connection, in ms
	FlushThrottleTimeout int `mapstructure:"flush_throttle_timeout"`

	// Maximum size of a message packet payload, in bytes
	MaxMsgPacketPayloadSize int `mapstructure:"max_msg_packet_payload_size"`

	// Rate at which packets can be sent, in bytes/second
	SendRate int64 `mapstructure:"send_rate"`

	// Rate at which packets can be received, in bytes/second
	RecvRate int64 `mapstructure:"recv_rate"`
}

// DefaultP2PConfig returns a default configuration for the peer-to-peer layer
func DefaultP2PConfig() *P2PConfig {
	return &P2PConfig{
		ListenAddress:           "tcp://0.0.0.0:46656",
		AddrBook:                "addrbook.json",
		AddrBookStrict:          true,
		MaxNumPeers:             50,
		FlushThrottleTimeout:    100,
		MaxMsgPacketPayloadSize: 1024,   // 1 kB
		SendRate:                512000, // 500 kB/s
		RecvRate:                512000, // 500 kB/s
	}
}

// TestP2PConfig returns a configuration for testing the peer-to-peer layer
func TestP2PConfig() *P2PConfig {
	conf := DefaultP2PConfig()
	conf.ListenAddress = "tcp://0.0.0.0:36656"
	conf.SkipUPNP = true
	return conf
}

// AddrBookFile returns the full path to the address bool
func (p *P2PConfig) AddrBookFile() string {
	return rootify(p.AddrBook, p.RootDir)
}

//-----------------------------------------------------------------------------
// MempoolConfig

// MempoolConfig defines the configuration options for the Tendermint mempool
type MempoolConfig struct {
	RootDir      string `mapstructure:"home"`
	Recheck      bool   `mapstructure:"recheck"`
	RecheckEmpty bool   `mapstructure:"recheck_empty"`
	Broadcast    bool   `mapstructure:"broadcast"`
	WalPath      string `mapstructure:"wal_dir"`
}

// DefaultMempoolConfig returns a default configuration for the Tendermint mempool
func DefaultMempoolConfig() *MempoolConfig {
	return &MempoolConfig{
		Recheck:      true,
		RecheckEmpty: true,
		Broadcast:    true,
		WalPath:      "data/mempool.wal",
	}
}

// WalDir returns the full path to the mempool's write-ahead log
func (m *MempoolConfig) WalDir() string {
	return rootify(m.WalPath, m.RootDir)
}

//-----------------------------------------------------------------------------
// ConsensusConfig

// ConsensusConfig defines the confuguration for the Tendermint consensus service,
// including timeouts and details about the WAL and the block structure.
type ConsensusConfig struct {
	RootDir  string `mapstructure:"home"`
	WalPath  string `mapstructure:"wal_file"`
	WalLight bool   `mapstructure:"wal_light"`
	walFile  string // overrides WalPath if set

	// All timeouts are in ms
	TimeoutPropose        int `mapstructure:"timeout_propose"`
	TimeoutProposeDelta   int `mapstructure:"timeout_propose_delta"`
	TimeoutPrevote        int `mapstructure:"timeout_prevote"`
	TimeoutPrevoteDelta   int `mapstructure:"timeout_prevote_delta"`
	TimeoutPrecommit      int `mapstructure:"timeout_precommit"`
	TimeoutPrecommitDelta int `mapstructure:"timeout_precommit_delta"`
	TimeoutCommit         int `mapstructure:"timeout_commit"`

	// Make progress as soon as we have all the precommits (as if TimeoutCommit = 0)
	SkipTimeoutCommit bool `mapstructure:"skip_timeout_commit"`

	// BlockSize
	MaxBlockSizeTxs   int `mapstructure:"max_block_size_txs"`
	MaxBlockSizeBytes int `mapstructure:"max_block_size_bytes"`

	// EmptyBlocks mode and possible interval between empty blocks in seconds
	CreateEmptyBlocks         bool `mapstructure:"create_empty_blocks"`
	CreateEmptyBlocksInterval int  `mapstructure:"create_empty_blocks_interval"`

	// Reactor sleep duration parameters are in ms
	PeerGossipSleepDuration     int `mapstructure:"peer_gossip_sleep_duration"`
	PeerQueryMaj23SleepDuration int `mapstructure:"peer_query_maj23_sleep_duration"`
}

// WaitForTxs returns true if the consensus should wait for transactions before entering the propose step
func (cfg *ConsensusConfig) WaitForTxs() bool {
	return !cfg.CreateEmptyBlocks || cfg.CreateEmptyBlocksInterval > 0
}

// EmptyBlocks returns the amount of time to wait before proposing an empty block or starting the propose timer if there are no txs available
func (cfg *ConsensusConfig) EmptyBlocksInterval() time.Duration {
	return time.Duration(cfg.CreateEmptyBlocksInterval) * time.Second
}

// Propose returns the amount of time to wait for a proposal
func (cfg *ConsensusConfig) Propose(round int) time.Duration {
	return time.Duration(cfg.TimeoutPropose+cfg.TimeoutProposeDelta*round) * time.Millisecond
}

// Prevote returns the amount of time to wait for straggler votes after receiving any +2/3 prevotes
func (cfg *ConsensusConfig) Prevote(round int) time.Duration {
	return time.Duration(cfg.TimeoutPrevote+cfg.TimeoutPrevoteDelta*round) * time.Millisecond
}

// Precommit returns the amount of time to wait for straggler votes after receiving any +2/3 precommits
func (cfg *ConsensusConfig) Precommit(round int) time.Duration {
	return time.Duration(cfg.TimeoutPrecommit+cfg.TimeoutPrecommitDelta*round) * time.Millisecond
}

// Commit returns the amount of time to wait for straggler votes after receiving +2/3 precommits for a single block (ie. a commit).
func (cfg *ConsensusConfig) Commit(t time.Time) time.Time {
	return t.Add(time.Duration(cfg.TimeoutCommit) * time.Millisecond)
}

// PeerGossipSleep returns the amount of time to sleep if there is nothing to send from the ConsensusReactor
func (cfg *ConsensusConfig) PeerGossipSleep() time.Duration {
	return time.Duration(cfg.PeerGossipSleepDuration) * time.Millisecond
}

// PeerQueryMaj23Sleep returns the amount of time to sleep after each VoteSetMaj23Message is sent in the ConsensusReactor
func (cfg *ConsensusConfig) PeerQueryMaj23Sleep() time.Duration {
	return time.Duration(cfg.PeerQueryMaj23SleepDuration) * time.Millisecond
}

// DefaultConsensusConfig returns a default configuration for the consensus service
func DefaultConsensusConfig() *ConsensusConfig {
	return &ConsensusConfig{
		WalPath:                     "data/cs.wal/wal",
		WalLight:                    false,
		TimeoutPropose:              3000,
		TimeoutProposeDelta:         500,
		TimeoutPrevote:              1000,
		TimeoutPrevoteDelta:         500,
		TimeoutPrecommit:            1000,
		TimeoutPrecommitDelta:       500,
		TimeoutCommit:               1000,
		SkipTimeoutCommit:           false,
		MaxBlockSizeTxs:             10000,
		MaxBlockSizeBytes:           1, // TODO
		CreateEmptyBlocks:           true,
		CreateEmptyBlocksInterval:   0,
		PeerGossipSleepDuration:     100,
		PeerQueryMaj23SleepDuration: 2000,
	}
}

// TestConsensusConfig returns a configuration for testing the consensus service
func TestConsensusConfig() *ConsensusConfig {
	config := DefaultConsensusConfig()
	config.TimeoutPropose = 2000
	config.TimeoutProposeDelta = 1
	config.TimeoutPrevote = 10
	config.TimeoutPrevoteDelta = 1
	config.TimeoutPrecommit = 10
	config.TimeoutPrecommitDelta = 1
	config.TimeoutCommit = 10
	config.SkipTimeoutCommit = true
	return config
}

// WalFile returns the full path to the write-ahead log file
func (c *ConsensusConfig) WalFile() string {
	if c.walFile != "" {
		return c.walFile
	}
	return rootify(c.WalPath, c.RootDir)
}

// SetWalFile sets the path to the write-ahead log file
func (c *ConsensusConfig) SetWalFile(walFile string) {
	c.walFile = walFile
}

//-----------------------------------------------------------------------------
// Utils

// helper function to make config creation independent of root dir
func rootify(path, root string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}
