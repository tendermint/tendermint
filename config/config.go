package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// NOTE: Most of the structs & relevant comments + the
// default configuration options were used to manually
// generate the config.toml. Please reflect any changes
// made here in the defaultConfigTemplate constant in
// config/toml.go
// NOTE: tmlibs/cli must know to look in the config dir!
var (
	DefaultTendermintDir = ".tendermint"
	defaultConfigDir     = "config"
	defaultDataDir       = "data"

	defaultConfigFileName  = "config.toml"
	defaultGenesisJSONName = "genesis.json"

	defaultPrivValName  = "priv_validator.json"
	defaultNodeKeyName  = "node_key.json"
	defaultAddrBookName = "addrbook.json"

	defaultConfigFilePath  = filepath.Join(defaultConfigDir, defaultConfigFileName)
	defaultGenesisJSONPath = filepath.Join(defaultConfigDir, defaultGenesisJSONName)
	defaultPrivValPath     = filepath.Join(defaultConfigDir, defaultPrivValName)
	defaultNodeKeyPath     = filepath.Join(defaultConfigDir, defaultNodeKeyName)
	defaultAddrBookPath    = filepath.Join(defaultConfigDir, defaultAddrBookName)
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
	TxIndex   *TxIndexConfig   `mapstructure:"tx_index"`
}

// DefaultConfig returns a default configuration for a Tendermint node
func DefaultConfig() *Config {
	return &Config{
		BaseConfig: DefaultBaseConfig(),
		RPC:        DefaultRPCConfig(),
		P2P:        DefaultP2PConfig(),
		Mempool:    DefaultMempoolConfig(),
		Consensus:  DefaultConsensusConfig(),
		TxIndex:    DefaultTxIndexConfig(),
	}
}

// TestConfig returns a configuration that can be used for testing
func TestConfig() *Config {
	return &Config{
		BaseConfig: TestBaseConfig(),
		RPC:        TestRPCConfig(),
		P2P:        TestP2PConfig(),
		Mempool:    TestMempoolConfig(),
		Consensus:  TestConsensusConfig(),
		TxIndex:    TestTxIndexConfig(),
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

	// chainID is unexposed and immutable but here for convenience
	chainID string

	// The root directory for all data.
	// This should be set in viper so it can unmarshal into this struct
	RootDir string `mapstructure:"home"`

	// Path to the JSON file containing the initial validator set and other meta data
	Genesis string `mapstructure:"genesis_file"`

	// Path to the JSON file containing the private key to use as a validator in the consensus protocol
	PrivValidator string `mapstructure:"priv_validator_file"`

	// A JSON file containing the private key to use for p2p authenticated encryption
	NodeKey string `mapstructure:"node_key_file"`

	// A custom human readable name for this node
	Moniker string `mapstructure:"moniker"`

	// TCP or UNIX socket address for Tendermint to listen on for
	// connections from an external PrivValidator process
	PrivValidatorListenAddr string `mapstructure:"priv_validator_laddr"`

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

	// Database backend: leveldb | memdb
	DBBackend string `mapstructure:"db_backend"`

	// Database directory
	DBPath string `mapstructure:"db_dir"`
}

// DefaultBaseConfig returns a default base configuration for a Tendermint node
func DefaultBaseConfig() BaseConfig {
	return BaseConfig{
		Genesis:           defaultGenesisJSONPath,
		PrivValidator:     defaultPrivValPath,
		NodeKey:           defaultNodeKeyPath,
		Moniker:           defaultMoniker,
		ProxyApp:          "tcp://127.0.0.1:46658",
		ABCI:              "socket",
		LogLevel:          DefaultPackageLogLevels(),
		ProfListenAddress: "",
		FastSync:          true,
		FilterPeers:       false,
		DBBackend:         "leveldb",
		DBPath:            "data",
	}
}

// TestBaseConfig returns a base configuration for testing a Tendermint node
func TestBaseConfig() BaseConfig {
	cfg := DefaultBaseConfig()
	cfg.chainID = "tendermint_test"
	cfg.ProxyApp = "kvstore"
	cfg.FastSync = false
	cfg.DBBackend = "memdb"
	return cfg
}

func (cfg BaseConfig) ChainID() string {
	return cfg.chainID
}

// GenesisFile returns the full path to the genesis.json file
func (cfg BaseConfig) GenesisFile() string {
	return rootify(cfg.Genesis, cfg.RootDir)
}

// PrivValidatorFile returns the full path to the priv_validator.json file
func (cfg BaseConfig) PrivValidatorFile() string {
	return rootify(cfg.PrivValidator, cfg.RootDir)
}

// NodeKeyFile returns the full path to the node_key.json file
func (cfg BaseConfig) NodeKeyFile() string {
	return rootify(cfg.NodeKey, cfg.RootDir)
}

// DBDir returns the full path to the database directory
func (cfg BaseConfig) DBDir() string {
	return rootify(cfg.DBPath, cfg.RootDir)
}

// DefaultLogLevel returns a default log level of "error"
func DefaultLogLevel() string {
	return "error"
}

// DefaultPackageLogLevels returns a default log level setting so all packages
// log at "error", while the `state` and `main` packages log at "info"
func DefaultPackageLogLevels() string {
	return fmt.Sprintf("main:info,state:info,*:%s", DefaultLogLevel())
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

	// Activate unsafe RPC commands like /dial_persistent_peers and /unsafe_flush_mempool
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
	cfg := DefaultRPCConfig()
	cfg.ListenAddress = "tcp://0.0.0.0:36657"
	cfg.GRPCListenAddress = "tcp://0.0.0.0:36658"
	cfg.Unsafe = true
	return cfg
}

//-----------------------------------------------------------------------------
// P2PConfig

// P2PConfig defines the configuration options for the Tendermint peer-to-peer networking layer
type P2PConfig struct {
	RootDir string `mapstructure:"home"`

	// Address to listen for incoming connections
	ListenAddress string `mapstructure:"laddr"`

	// Comma separated list of seed nodes to connect to
	// We only use these if we can’t connect to peers in the addrbook
	Seeds string `mapstructure:"seeds"`

	// Comma separated list of nodes to keep persistent connections to
	// Do not add private peers to this list if you don't want them advertised
	PersistentPeers string `mapstructure:"persistent_peers"`

	// Skip UPNP port forwarding
	SkipUPNP bool `mapstructure:"skip_upnp"`

	// Path to address book
	AddrBook string `mapstructure:"addr_book_file"`

	// Set true for strict address routability rules
	AddrBookStrict bool `mapstructure:"addr_book_strict"`

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

	// Set true to enable the peer-exchange reactor
	PexReactor bool `mapstructure:"pex"`

	// Seed mode, in which node constantly crawls the network and looks for
	// peers. If another node asks it for addresses, it responds and disconnects.
	//
	// Does not work if the peer-exchange reactor is disabled.
	SeedMode bool `mapstructure:"seed_mode"`

	// Authenticated encryption
	AuthEnc bool `mapstructure:"auth_enc"`

	// Comma separated list of peer IDs to keep private (will not be gossiped to other peers)
	PrivatePeerIDs string `mapstructure:"private_peer_ids"`
}

// DefaultP2PConfig returns a default configuration for the peer-to-peer layer
func DefaultP2PConfig() *P2PConfig {
	return &P2PConfig{
		ListenAddress:           "tcp://0.0.0.0:46656",
		AddrBook:                defaultAddrBookPath,
		AddrBookStrict:          true,
		MaxNumPeers:             50,
		FlushThrottleTimeout:    100,
		MaxMsgPacketPayloadSize: 1024,   // 1 kB
		SendRate:                512000, // 500 kB/s
		RecvRate:                512000, // 500 kB/s
		PexReactor:              true,
		SeedMode:                false,
		AuthEnc:                 true,
	}
}

// TestP2PConfig returns a configuration for testing the peer-to-peer layer
func TestP2PConfig() *P2PConfig {
	cfg := DefaultP2PConfig()
	cfg.ListenAddress = "tcp://0.0.0.0:36656"
	cfg.SkipUPNP = true
	cfg.FlushThrottleTimeout = 10
	return cfg
}

// AddrBookFile returns the full path to the address book
func (cfg *P2PConfig) AddrBookFile() string {
	return rootify(cfg.AddrBook, cfg.RootDir)
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
	CacheSize    int    `mapstructure:"cache_size"`
}

// DefaultMempoolConfig returns a default configuration for the Tendermint mempool
func DefaultMempoolConfig() *MempoolConfig {
	return &MempoolConfig{
		Recheck:      true,
		RecheckEmpty: true,
		Broadcast:    true,
		WalPath:      filepath.Join(defaultDataDir, "mempool.wal"),
		CacheSize:    100000,
	}
}

// TestMempoolConfig returns a configuration for testing the Tendermint mempool
func TestMempoolConfig() *MempoolConfig {
	cfg := DefaultMempoolConfig()
	cfg.CacheSize = 1000
	return cfg
}

// WalDir returns the full path to the mempool's write-ahead log
func (cfg *MempoolConfig) WalDir() string {
	return rootify(cfg.WalPath, cfg.RootDir)
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

	// All timeouts are in milliseconds
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

	// Reactor sleep duration parameters are in milliseconds
	PeerGossipSleepDuration     int `mapstructure:"peer_gossip_sleep_duration"`
	PeerQueryMaj23SleepDuration int `mapstructure:"peer_query_maj23_sleep_duration"`
}

// DefaultConsensusConfig returns a default configuration for the consensus service
func DefaultConsensusConfig() *ConsensusConfig {
	return &ConsensusConfig{
		WalPath:                     filepath.Join(defaultDataDir, "cs.wal", "wal"),
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
	cfg := DefaultConsensusConfig()
	cfg.TimeoutPropose = 100
	cfg.TimeoutProposeDelta = 1
	cfg.TimeoutPrevote = 10
	cfg.TimeoutPrevoteDelta = 1
	cfg.TimeoutPrecommit = 10
	cfg.TimeoutPrecommitDelta = 1
	cfg.TimeoutCommit = 10
	cfg.SkipTimeoutCommit = true
	cfg.PeerGossipSleepDuration = 5
	cfg.PeerQueryMaj23SleepDuration = 250
	return cfg
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

// WalFile returns the full path to the write-ahead log file
func (cfg *ConsensusConfig) WalFile() string {
	if cfg.walFile != "" {
		return cfg.walFile
	}
	return rootify(cfg.WalPath, cfg.RootDir)
}

// SetWalFile sets the path to the write-ahead log file
func (cfg *ConsensusConfig) SetWalFile(walFile string) {
	cfg.walFile = walFile
}

//-----------------------------------------------------------------------------
// TxIndexConfig

// TxIndexConfig defines the confuguration for the transaction
// indexer, including tags to index.
type TxIndexConfig struct {
	// What indexer to use for transactions
	//
	// Options:
	//   1) "null" (default)
	//   2) "kv" - the simplest possible indexer, backed by key-value storage (defaults to levelDB; see DBBackend).
	Indexer string `mapstructure:"indexer"`

	// Comma-separated list of tags to index (by default the only tag is tx hash)
	//
	// It's recommended to index only a subset of tags due to possible memory
	// bloat. This is, of course, depends on the indexer's DB and the volume of
	// transactions.
	IndexTags string `mapstructure:"index_tags"`

	// When set to true, tells indexer to index all tags. Note this may be not
	// desirable (see the comment above). IndexTags has a precedence over
	// IndexAllTags (i.e. when given both, IndexTags will be indexed).
	IndexAllTags bool `mapstructure:"index_all_tags"`
}

// DefaultTxIndexConfig returns a default configuration for the transaction indexer.
func DefaultTxIndexConfig() *TxIndexConfig {
	return &TxIndexConfig{
		Indexer:      "kv",
		IndexTags:    "",
		IndexAllTags: false,
	}
}

// TestTxIndexConfig returns a default configuration for the transaction indexer.
func TestTxIndexConfig() *TxIndexConfig {
	return DefaultTxIndexConfig()
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

//-----------------------------------------------------------------------------
// Moniker

var defaultMoniker = getDefaultMoniker()

// getDefaultMoniker returns a default moniker, which is the host name. If runtime
// fails to get the host name, "anonymous" will be returned.
func getDefaultMoniker() string {
	moniker, err := os.Hostname()
	if err != nil {
		moniker = "anonymous"
	}
	return moniker
}
