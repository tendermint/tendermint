package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/tendermint/tendermint/abci/client"
	abci"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/abci/example/counter"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

// NOTE: Most of the structs & relevant comments + the
// default configuration options were used to manually
// generate the config.toml. Please reflect any changes
// made here in the defaultConfigTemplate constant in
// config/toml.go
// NOTE: libs/cli must know to look in the config dir!
var (
	DefaultTendermintDir = ".tendermint"
	defaultConfigDir     = "config"
	defaultDataDir       = "data"

	defaultConfigFileName  = "config.toml"
	defaultGenesisJSONName = "genesis.json"

	defaultMode             = ModeFull
	defaultPrivValKeyName   = "priv_validator_key.json"
	defaultPrivValStateName = "priv_validator_state.json"

	defaultNodeKeyName  = "node_key.json"
	defaultAddrBookName = "addrbook.json"

	defaultConfigFilePath   = filepath.Join(defaultConfigDir, defaultConfigFileName)
	defaultGenesisJSONPath  = filepath.Join(defaultConfigDir, defaultGenesisJSONName)
	defaultPrivValKeyPath   = filepath.Join(defaultConfigDir, defaultPrivValKeyName)
	defaultPrivValStatePath = filepath.Join(defaultDataDir, defaultPrivValStateName)

	defaultNodeKeyPath  = filepath.Join(defaultConfigDir, defaultNodeKeyName)
	defaultAddrBookPath = filepath.Join(defaultConfigDir, defaultAddrBookName)
)

type FileConfig struct {
	// Top level options use an anonymous struct
	BaseFileConfig `mapstructure:",squash"`

	// Options for services
	RPC             *RPCConfig             `mapstructure:"rpc"`
	P2P             *P2PConfig             `mapstructure:"p2p"`
	Mempool         *MempoolConfig         `mapstructure:"mempool"`
	StateSync       *StateSyncConfig       `mapstructure:"statesync"`
	FastSync        *FastSyncConfig        `mapstructure:"fastsync"`
	Consensus       *ConsensusConfig       `mapstructure:"consensus"`
	TxIndex         *TxIndexConfig         `mapstructure:"tx-index"`
	Instrumentation *InstrumentationConfig `mapstructure:"instrumentation"`
	PrivValidator   *PrivValidatorConfig   `mapstructure:"priv-validator"`
}

// DefaultConfig returns a default configuration for a Tendermint node
func DefaultFileConfig() *FileConfig {
	return &FileConfig{
		BaseFileConfig:  DefaultBaseFileConfig(),
		RPC:             DefaultRPCConfig(),
		P2P:             DefaultP2PConfig(),
		Mempool:         DefaultMempoolConfig(),
		StateSync:       DefaultStateSyncConfig(),
		FastSync:        DefaultFastSyncConfig(),
		Consensus:       DefaultConsensusConfig(),
		TxIndex:         DefaultTxIndexConfig(),
		Instrumentation: DefaultInstrumentationConfig(),
		PrivValidator:   DefaultPrivValidatorConfig(),
	}
}

// TestConfig returns a configuration that can be used for testing
func TestFileConfig() *FileConfig {
	return &FileConfig{
		BaseFileConfig:  TestBaseFileConfig(),
		RPC:             TestRPCConfig(),
		P2P:             TestP2PConfig(),
		Mempool:         TestMempoolConfig(),
		StateSync:       TestStateSyncConfig(),
		FastSync:        TestFastSyncConfig(),
		Consensus:       TestConsensusConfig(),
		TxIndex:         TestTxIndexConfig(),
		Instrumentation: TestInstrumentationConfig(),
		PrivValidator:   DefaultPrivValidatorConfig(),
	}
}

func (cfg FileConfig) Build() (Config, error) {
	baseConfig, err := cfg.BaseFileConfig.Build()
	if err != nil {
		return Config{}, err
	}
	return Config{
		BaseConfig: baseConfig,

		RPC: cfg.RPC,
		P2P: cfg.P2P,
		Mempool: cfg.Mempool,
		StateSync: cfg.StateSync,
		FastSync: cfg.FastSync,
		Consensus: cfg.Consensus,
		TxIndex: cfg.TxIndex,
		Instrumentation: cfg.Instrumentation,
		PrivValidator: cfg.PrivValidator,
	}, nil
}

// SetRoot sets the RootDir for all Config structs
func (cfg *FileConfig) SetRoot(root string) *FileConfig {
	cfg.BaseFileConfig.RootDir = root
	cfg.RPC.RootDir = root
	cfg.P2P.RootDir = root
	cfg.Mempool.RootDir = root
	cfg.Consensus.RootDir = root
	return cfg
}

type BaseFileConfig struct { //nolint: maligned
	// chainID is unexposed and immutable but here for convenience
	chainID string

	// The root directory for all data.
	// This should be set in viper so it can unmarshal into this struct
	RootDir string `mapstructure:"home"`

	// TCP or UNIX socket address of the ABCI application,
	// or the name of an ABCI application compiled in with the Tendermint binary
	ProxyApp string `mapstructure:"proxy-app"`

	// A custom human readable name for this node
	Moniker string `mapstructure:"moniker"`

	// Mode of Node: full | validator | seed
	// * validator
	//   - all reactors
	//   - with priv_validator_key.json, priv_validator_state.json
	// * full
	//   - all reactors
	//   - No priv_validator_key.json, priv_validator_state.json
	// * seed
	//   - only P2P, PEX Reactor
	//   - No priv_validator_key.json, priv_validator_state.json
	Mode string `mapstructure:"mode"`

	// If this node is many blocks behind the tip of the chain, FastSync
	// allows them to catchup quickly by downloading blocks in parallel
	// and verifying their commits
	FastSyncMode bool `mapstructure:"fast-sync"`

	// Database backend: goleveldb | cleveldb | boltdb | rocksdb
	// * goleveldb (github.com/syndtr/goleveldb - most popular implementation)
	//   - pure go
	//   - stable
	// * cleveldb (uses levigo wrapper)
	//   - fast
	//   - requires gcc
	//   - use cleveldb build tag (go build -tags cleveldb)
	// * boltdb (uses etcd's fork of bolt - github.com/etcd-io/bbolt)
	//   - EXPERIMENTAL
	//   - may be faster is some use-cases (random reads - indexer)
	//   - use boltdb build tag (go build -tags boltdb)
	// * rocksdb (uses github.com/tecbot/gorocksdb)
	//   - EXPERIMENTAL
	//   - requires gcc
	//   - use rocksdb build tag (go build -tags rocksdb)
	// * badgerdb (uses github.com/dgraph-io/badger)
	//   - EXPERIMENTAL
	//   - use badgerdb build tag (go build -tags badgerdb)
	DBBackend string `mapstructure:"db-backend"`

	// Database directory
	DBPath string `mapstructure:"db-dir"`

	// Output level for logging
	LogLevel string `mapstructure:"log-level"`

	// Output format: 'plain' (colored text) or 'json'
	LogFormat string `mapstructure:"log-format"`

	// Path to the JSON file containing the initial validator set and other meta data
	Genesis string `mapstructure:"genesis-file"`

	// A JSON file containing the private key to use for p2p authenticated encryption
	NodeKey string `mapstructure:"node-key-file"`

	// Mechanism to connect to the ABCI application: socket | grpc
	ABCI string `mapstructure:"abci"`

	// If true, query the ABCI app on connecting to a new peer
	// so the app can decide if we should keep the connection or not
	FilterPeers bool `mapstructure:"filter-peers"` // false
}

// DefaultBaseConfig returns a default base configuration for a Tendermint node
func DefaultBaseFileConfig() BaseFileConfig {
	return BaseFileConfig{
		Genesis:      defaultGenesisJSONPath,
		NodeKey:      defaultNodeKeyPath,
		Mode:         defaultMode,
		Moniker:      defaultMoniker,
		ProxyApp:     "tcp://127.0.0.1:26658",
		ABCI:         "socket",
		LogLevel:     DefaultLogLevel,
		LogFormat:    LogFormatPlain,
		FastSyncMode: true,
		FilterPeers:  false,
		DBBackend:    "goleveldb",
		DBPath:       "data",
	}
}

// TestBaseConfig returns a base configuration for testing a Tendermint node
func TestBaseFileConfig() BaseFileConfig {
	cfg := DefaultBaseFileConfig()
	cfg.chainID = "tendermint_test"
	cfg.Mode = ModeValidator
	cfg.ProxyApp = "kvstore"
	cfg.FastSyncMode = false
	cfg.DBBackend = "memdb"
	return cfg
}

func (cfg BaseFileConfig) Build() (BaseConfig, error) {
	abciClient, err := createClient(cfg.ProxyApp, cfg.ABCI, cfg.DBDir())
	if err != nil {
		return BaseConfig{}, err
	}
	genDoc, err := types.GenesisDocFromFile(cfg.GenesisFile())
	if err != nil {
		return BaseConfig{}, err
	}

	return BaseConfig{
		chainID: cfg.chainID,
		RootDir: cfg.RootDir,
		ABCI: abciClient,
		Moniker: cfg.Moniker,
		Mode: cfg.Mode,
		FastSyncMode: cfg.FastSyncMode,
		DBProvider: DefaultDBProvider{
			backend: cfg.DBBackend,
			dir: cfg.DBDir(),
		},
		Logger: log.NewTMLogger(log.NewSyncWriter(os.Stdout)),
		Genesis: *genDoc,
		NodeKey: cfg.NodeKey,
		FilterPeers: cfg.FilterPeers,
	}, nil

}

// GenesisFile returns the full path to the genesis.json file
func (cfg BaseFileConfig) GenesisFile() string {
	return rootify(cfg.Genesis, cfg.RootDir)
}

// DBDir returns the full path to the database directory
func (cfg BaseFileConfig) DBDir() string {
	return rootify(cfg.DBPath, cfg.RootDir)
}

func (cfg BaseFileConfig) ValidateBasic() error { 
	switch cfg.LogFormat {
	case LogFormatPlain, LogFormatJSON:
	default:
		return errors.New("unknown log format (must be 'plain' or 'json')")
	}
	return nil
}

func createClient(addr, transport, dbDir string) (abcicli.Client, error) {
	switch addr {
	case "counter":
		return abcicli.NewLocalClient(new(tmsync.RWMutex), counter.NewApplication(false)), nil
	case "counter_serial":
		return abcicli.NewLocalClient(new(tmsync.RWMutex), counter.NewApplication(true)), nil
	case "kvstore":
		return abcicli.NewLocalClient(new(tmsync.RWMutex), kvstore.NewApplication()), nil
	case "persistent_kvstore":
		app := kvstore.NewPersistentKVStoreApplication(dbDir)
		return abcicli.NewLocalClient(new(tmsync.RWMutex), app), nil
	case "noop":
		return abcicli.NewLocalClient(new(tmsync.RWMutex), abci.NewBaseApplication()), nil
	default:
		mustConnect := false // loop retrying
		return abcicli.NewClient(addr, transport, mustConnect)
	}
}