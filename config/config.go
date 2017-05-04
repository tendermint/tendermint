package config

import (
	"path/filepath"
	"time"

	"github.com/tendermint/tendermint/types"
)

type Config struct {
	// Top level options use an anonymous struct
	*BaseConfig `mapstructure:",squash"`

	// Options for services
	P2P       *P2PConfig       `mapstructure:"p2p"`
	Mempool   *MempoolConfig   `mapstructure:"mempool"`
	Consensus *ConsensusConfig `mapstructure:"consensus"`
}

func DefaultConfig() *Config {
	return &Config{
		BaseConfig: DefaultBaseConfig(),
		P2P:        DefaultP2PConfig(),
		Mempool:    DefaultMempoolConfig(),
		Consensus:  DefaultConsensusConfig(),
	}
}

func TestConfig() *Config {
	return &Config{
		BaseConfig: DefaultBaseConfig(),
		P2P:        DefaultP2PConfig(),
		Mempool:    DefaultMempoolConfig(),
		Consensus:  TestConsensusConfig(),
	}
}

// TODO: set this from root or home.... or default....
func SetRoot(cfg *Config, root string) {
	cfg.BaseConfig.RootDir = root
	cfg.P2P.RootDir = root
	cfg.Mempool.RootDir = root
	cfg.Consensus.RootDir = root
}

// BaseConfig struct for a Tendermint node
type BaseConfig struct {
	// TODO: set this from root or home.... or default....
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

	// TCP or UNIX socket address for the RPC server to listen on
	RPCListenAddress string `mapstructure:"rpc_laddr"`

	// TCP or UNIX socket address for the gRPC server to listen on
	// NOTE: This server only supports /broadcast_tx_commit
	GRPCListenAddress string `mapstructure:"grpc_laddr"`
}

func DefaultBaseConfig() *BaseConfig {
	return &BaseConfig{
		Genesis:           "genesis.json",
		PrivValidator:     "priv_validator.json",
		Moniker:           "anonymous",
		ProxyApp:          "tcp://127.0.0.1:46658",
		ABCI:              "socket",
		LogLevel:          "info",
		ProfListenAddress: "",
		FastSync:          true,
		FilterPeers:       false,
		TxIndex:           "kv",
		DBBackend:         "leveldb",
		DBPath:            "data",
		RPCListenAddress:  "tcp://0.0.0.0:46657",
		GRPCListenAddress: "",
	}
}

func (b *BaseConfig) GenesisFile() string {
	return rootify(b.Genesis, b.RootDir)
}

func (b *BaseConfig) PrivValidatorFile() string {
	return rootify(b.PrivValidator, b.RootDir)
}

func (b *BaseConfig) DBDir() string {
	return rootify(b.DBPath, b.RootDir)
}

type P2PConfig struct {
	RootDir        string `mapstructure:"home"`
	ListenAddress  string `mapstructure:"laddr"`
	Seeds          string `mapstructure:"seeds"`
	SkipUPNP       bool   `mapstructure:"skip_upnp"`
	AddrBook       string `mapstructure:"addr_book_file"`
	AddrBookStrict bool   `mapstructure:"addr_book_strict"`
	PexReactor     bool   `mapstructure:"pex_reactor"`
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

func (p *P2PConfig) AddrBookFile() string {
	return rootify(p.AddrBook, p.RootDir)
}

type MempoolConfig struct {
	RootDir      string `mapstructure:"home"`
	Recheck      bool   `mapstructure:"recheck"`       // true
	RecheckEmpty bool   `mapstructure:"recheck_empty"` // true
	Broadcast    bool   `mapstructure:"broadcast"`     // true
	WalPath      string `mapstructure:"wal_dir"`       //
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

// ConsensusConfig holds timeouts and details about the WAL, the block structure,
// and timeouts in the consensus protocol.
type ConsensusConfig struct {
	RootDir  string `mapstructure:"home"`
	WalPath  string `mapstructure:"wal_file"`
	WalLight bool   `mapstructure:"wal_light"`

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
		MaxBlockSizeBytes:     1, // TODO
		BlockPartSize:         types.DefaultBlockPartSize,
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
	return rootify(c.WalPath, c.RootDir)
}

// helper function to make config creation independent of root dir
func rootify(path, root string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}
