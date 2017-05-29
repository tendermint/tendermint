package config

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/tendermint/tendermint/types"
)

type Config struct {
	// Top level options use an anonymous struct
	BaseConfig `mapstructure:",squash"`

	// Options for services
	RPC       *RPCConfig       `mapstructure:"rpc"`
	P2P       *P2PConfig       `mapstructure:"p2p"`
	Mempool   *MempoolConfig   `mapstructure:"mempool"`
	Consensus *ConsensusConfig `mapstructure:"consensus"`
}

func DefaultConfig() *Config {
	return &Config{
		BaseConfig: DefaultBaseConfig(),
		RPC:        DefaultRPCConfig(),
		P2P:        DefaultP2PConfig(),
		Mempool:    DefaultMempoolConfig(),
		Consensus:  DefaultConsensusConfig(),
	}
}

func TestConfig() *Config {
	return &Config{
		BaseConfig: TestBaseConfig(),
		RPC:        TestRPCConfig(),
		P2P:        TestP2PConfig(),
		Mempool:    DefaultMempoolConfig(),
		Consensus:  TestConsensusConfig(),
	}
}

// Set the RootDir for all Config structs
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

// BaseConfig struct for a Tendermint node
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

func TestBaseConfig() BaseConfig {
	conf := DefaultBaseConfig()
	conf.ChainID = "tendermint_test"
	conf.ProxyApp = "dummy"
	conf.FastSync = false
	conf.DBBackend = "memdb"
	return conf
}

func (b BaseConfig) GenesisFile() string {
	return rootify(b.Genesis, b.RootDir)
}

func (b BaseConfig) PrivValidatorFile() string {
	return rootify(b.PrivValidator, b.RootDir)
}

func (b BaseConfig) DBDir() string {
	return rootify(b.DBPath, b.RootDir)
}

func DefaultLogLevel() string {
	return "error"
}

func DefaultPackageLogLevels() string {
	return fmt.Sprintf("state:info,*:%s", DefaultLogLevel())
}

//-----------------------------------------------------------------------------
// RPCConfig

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

func DefaultRPCConfig() *RPCConfig {
	return &RPCConfig{
		ListenAddress:     "tcp://0.0.0.0:46657",
		GRPCListenAddress: "",
		Unsafe:            false,
	}
}

func TestRPCConfig() *RPCConfig {
	conf := DefaultRPCConfig()
	conf.ListenAddress = "tcp://0.0.0.0:36657"
	conf.GRPCListenAddress = "tcp://0.0.0.0:36658"
	conf.Unsafe = true
	return conf
}

//-----------------------------------------------------------------------------
// P2PConfig

type P2PConfig struct {
	RootDir        string `mapstructure:"home"`
	ListenAddress  string `mapstructure:"laddr"`
	Seeds          string `mapstructure:"seeds"`
	SkipUPNP       bool   `mapstructure:"skip_upnp"`
	AddrBook       string `mapstructure:"addr_book_file"`
	AddrBookStrict bool   `mapstructure:"addr_book_strict"`
	PexReactor     bool   `mapstructure:"pex"`
	MaxNumPeers    int    `mapstructure:"max_num_peers"`
}

func DefaultP2PConfig() *P2PConfig {
	return &P2PConfig{
		ListenAddress:  "tcp://0.0.0.0:46656",
		AddrBook:       "addrbook.json",
		AddrBookStrict: true,
		MaxNumPeers:    50,
	}
}

func TestP2PConfig() *P2PConfig {
	conf := DefaultP2PConfig()
	conf.ListenAddress = "tcp://0.0.0.0:36656"
	conf.SkipUPNP = true
	return conf
}

func (p *P2PConfig) AddrBookFile() string {
	return rootify(p.AddrBook, p.RootDir)
}

//-----------------------------------------------------------------------------
// MempoolConfig

type MempoolConfig struct {
	RootDir      string `mapstructure:"home"`
	Recheck      bool   `mapstructure:"recheck"`
	RecheckEmpty bool   `mapstructure:"recheck_empty"`
	Broadcast    bool   `mapstructure:"broadcast"`
	WalPath      string `mapstructure:"wal_dir"`
}

func DefaultMempoolConfig() *MempoolConfig {
	return &MempoolConfig{
		Recheck:      true,
		RecheckEmpty: true,
		Broadcast:    true,
		WalPath:      "data/mempool.wal",
	}
}

func (m *MempoolConfig) WalDir() string {
	return rootify(m.WalPath, m.RootDir)
}

//-----------------------------------------------------------------------------
// ConsensusConfig

// ConsensusConfig holds timeouts and details about the WAL, the block structure,
// and timeouts in the consensus protocol.
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

	// TODO: This probably shouldn't be exposed but it makes it
	// easy to write tests for the wal/replay
	BlockPartSize int `mapstructure:"block_part_size"`
}

// Wait this long for a proposal
func (cfg *ConsensusConfig) Propose(round int) time.Duration {
	return time.Duration(cfg.TimeoutPropose+cfg.TimeoutProposeDelta*round) * time.Millisecond
}

// After receiving any +2/3 prevote, wait this long for stragglers
func (cfg *ConsensusConfig) Prevote(round int) time.Duration {
	return time.Duration(cfg.TimeoutPrevote+cfg.TimeoutPrevoteDelta*round) * time.Millisecond
}

// After receiving any +2/3 precommits, wait this long for stragglers
func (cfg *ConsensusConfig) Precommit(round int) time.Duration {
	return time.Duration(cfg.TimeoutPrecommit+cfg.TimeoutPrecommitDelta*round) * time.Millisecond
}

// After receiving +2/3 precommits for a single block (a commit), wait this long for stragglers in the next height's RoundStepNewHeight
func (cfg *ConsensusConfig) Commit(t time.Time) time.Time {
	return t.Add(time.Duration(cfg.TimeoutCommit) * time.Millisecond)
}

func DefaultConsensusConfig() *ConsensusConfig {
	return &ConsensusConfig{
		WalPath:               "data/cs.wal/wal",
		WalLight:              false,
		TimeoutPropose:        3000,
		TimeoutProposeDelta:   500,
		TimeoutPrevote:        1000,
		TimeoutPrevoteDelta:   500,
		TimeoutPrecommit:      1000,
		TimeoutPrecommitDelta: 500,
		TimeoutCommit:         1000,
		SkipTimeoutCommit:     false,
		MaxBlockSizeTxs:       10000,
		MaxBlockSizeBytes:     1,                          // TODO
		BlockPartSize:         types.DefaultBlockPartSize, // TODO: we shouldnt be importing types
	}
}

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

func (c *ConsensusConfig) WalFile() string {
	if c.walFile != "" {
		return c.walFile
	}
	return rootify(c.WalPath, c.RootDir)
}

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
