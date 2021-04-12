package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	// FuzzModeDrop is a mode in which we randomly drop reads/writes, connections or sleep
	FuzzModeDrop = iota
	// FuzzModeDelay is a mode in which we randomly sleep
	FuzzModeDelay

	// LogFormatPlain is a format for colored text
	LogFormatPlain = "plain"
	// LogFormatJSON is a format for json output
	LogFormatJSON = "json"

	// DefaultLogLevel defines a default log level as INFO.
	DefaultLogLevel = "info"

	ModeFull      = "full"
	ModeValidator = "validator"
	ModeSeed      = "seed"

	BlockchainV0 = "v0"
	BlockchainV2 = "v2"
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

// Config defines the top level configuration for a Tendermint node
type Config struct {
	// Top level options use an anonymous struct
	BaseConfig `mapstructure:",squash"`

	// Options for services
	RPC             *RPCConfig             `mapstructure:"rpc"`
	P2P             *P2PConfig             `mapstructure:"p2p"`
	Mempool         *MempoolConfig         `mapstructure:"mempool"`
	StateSync       *StateSyncConfig       `mapstructure:"statesync"`
	FastSync        *FastSyncConfig        `mapstructure:"fastsync"`
	Consensus       *ConsensusConfig       `mapstructure:"consensus"`
	TxIndex         *TxIndexConfig         `mapstructure:"tx-index"`
	Instrumentation *InstrumentationConfig `mapstructure:"instrumentation"`
}

// DefaultConfig returns a default configuration for a Tendermint node
func DefaultConfig() *Config {
	return &Config{
		BaseConfig:      DefaultBaseConfig(),
		RPC:             DefaultRPCConfig(),
		P2P:             DefaultP2PConfig(),
		Mempool:         DefaultMempoolConfig(),
		StateSync:       DefaultStateSyncConfig(),
		FastSync:        DefaultFastSyncConfig(),
		Consensus:       DefaultConsensusConfig(),
		TxIndex:         DefaultTxIndexConfig(),
		Instrumentation: DefaultInstrumentationConfig(),
	}
}

// DefaultValidatorConfig returns default config with mode as validator
func DefaultValidatorConfig() *Config {
	cfg := DefaultConfig()
	cfg.Mode = ModeValidator
	return cfg
}

// TestConfig returns a configuration that can be used for testing
func TestConfig() *Config {
	return &Config{
		BaseConfig:      TestBaseConfig(),
		RPC:             TestRPCConfig(),
		P2P:             TestP2PConfig(),
		Mempool:         TestMempoolConfig(),
		StateSync:       TestStateSyncConfig(),
		FastSync:        TestFastSyncConfig(),
		Consensus:       TestConsensusConfig(),
		TxIndex:         TestTxIndexConfig(),
		Instrumentation: TestInstrumentationConfig(),
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

// ValidateBasic performs basic validation (checking param bounds, etc.) and
// returns an error if any check fails.
func (cfg *Config) ValidateBasic() error {
	if err := cfg.BaseConfig.ValidateBasic(); err != nil {
		return err
	}
	if err := cfg.RPC.ValidateBasic(); err != nil {
		return fmt.Errorf("error in [rpc] section: %w", err)
	}
	if err := cfg.P2P.ValidateBasic(); err != nil {
		return fmt.Errorf("error in [p2p] section: %w", err)
	}
	if err := cfg.Mempool.ValidateBasic(); err != nil {
		return fmt.Errorf("error in [mempool] section: %w", err)
	}
	if err := cfg.StateSync.ValidateBasic(); err != nil {
		return fmt.Errorf("error in [statesync] section: %w", err)
	}
	if err := cfg.FastSync.ValidateBasic(); err != nil {
		return fmt.Errorf("error in [fastsync] section: %w", err)
	}
	if err := cfg.Consensus.ValidateBasic(); err != nil {
		return fmt.Errorf("error in [consensus] section: %w", err)
	}
	if err := cfg.Instrumentation.ValidateBasic(); err != nil {
		return fmt.Errorf("error in [instrumentation] section: %w", err)
	}
	return nil
}

//-----------------------------------------------------------------------------
// BaseConfig

// BaseConfig defines the base configuration for a Tendermint node
type BaseConfig struct { //nolint: maligned
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

	// Path to the JSON file containing the private key to use as a validator in the consensus protocol
	PrivValidatorKey string `mapstructure:"priv-validator-key-file"`

	// Path to the JSON file containing the last sign state of a validator
	PrivValidatorState string `mapstructure:"priv-validator-state-file"`

	// TCP or UNIX socket address for Tendermint to listen on for
	// connections from an external PrivValidator process
	PrivValidatorListenAddr string `mapstructure:"priv-validator-laddr"`

	// Client certificate generated while creating needed files for secure connection.
	// If a remote validator address is provided but no certificate, the connection will be insecure
	PrivValidatorClientCertificate string `mapstructure:"priv-validator-client-certificate-file"`

	// Client key generated while creating certificates for secure connection
	PrivValidatorClientKey string `mapstructure:"priv-validator-client-key-file"`

	// Path Root Certificate Authority used to sign both client and server certificates
	PrivValidatorRootCA string `mapstructure:"priv-validator-root-ca-file"`

	// A JSON file containing the private key to use for p2p authenticated encryption
	NodeKey string `mapstructure:"node-key-file"`

	// Mechanism to connect to the ABCI application: socket | grpc
	ABCI string `mapstructure:"abci"`

	// If true, query the ABCI app on connecting to a new peer
	// so the app can decide if we should keep the connection or not
	FilterPeers bool `mapstructure:"filter-peers"` // false
}

// DefaultBaseConfig returns a default base configuration for a Tendermint node
func DefaultBaseConfig() BaseConfig {
	return BaseConfig{
		Genesis:            defaultGenesisJSONPath,
		PrivValidatorKey:   defaultPrivValKeyPath,
		PrivValidatorState: defaultPrivValStatePath,
		NodeKey:            defaultNodeKeyPath,
		Mode:               defaultMode,
		Moniker:            defaultMoniker,
		ProxyApp:           "tcp://127.0.0.1:26658",
		ABCI:               "socket",
		LogLevel:           DefaultLogLevel,
		LogFormat:          LogFormatPlain,
		FastSyncMode:       true,
		FilterPeers:        false,
		DBBackend:          "goleveldb",
		DBPath:             "data",
	}
}

// TestBaseConfig returns a base configuration for testing a Tendermint node
func TestBaseConfig() BaseConfig {
	cfg := DefaultBaseConfig()
	cfg.chainID = "tendermint_test"
	cfg.Mode = ModeValidator
	cfg.ProxyApp = "kvstore"
	cfg.FastSyncMode = false
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

// PrivValidatorClientKeyFile returns the full path to the priv_validator_key.json file
func (cfg BaseConfig) PrivValidatorClientKeyFile() string {
	return rootify(cfg.PrivValidatorClientKey, cfg.RootDir)
}

// PrivValidatorClientCertificateFile returns the full path to the priv_validator_key.json file
func (cfg BaseConfig) PrivValidatorClientCertificateFile() string {
	return rootify(cfg.PrivValidatorClientCertificate, cfg.RootDir)
}

// PrivValidatorCertificateAuthorityFile returns the full path to the priv_validator_key.json file
func (cfg BaseConfig) PrivValidatorRootCAFile() string {
	return rootify(cfg.PrivValidatorRootCA, cfg.RootDir)
}

// PrivValidatorKeyFile returns the full path to the priv_validator_key.json file
func (cfg BaseConfig) PrivValidatorKeyFile() string {
	return rootify(cfg.PrivValidatorKey, cfg.RootDir)
}

// PrivValidatorFile returns the full path to the priv_validator_state.json file
func (cfg BaseConfig) PrivValidatorStateFile() string {
	return rootify(cfg.PrivValidatorState, cfg.RootDir)
}

// NodeKeyFile returns the full path to the node_key.json file
func (cfg BaseConfig) NodeKeyFile() string {
	return rootify(cfg.NodeKey, cfg.RootDir)
}

// DBDir returns the full path to the database directory
func (cfg BaseConfig) DBDir() string {
	return rootify(cfg.DBPath, cfg.RootDir)
}

func (cfg *BaseConfig) ArePrivValidatorClientSecurityOptionsPresent() bool {
	switch {
	case cfg.PrivValidatorRootCA == "":
		return false
	case cfg.PrivValidatorClientKey == "":
		return false
	case cfg.PrivValidatorClientCertificate == "":
		return false
	default:
		return true
	}
}

// ValidateBasic performs basic validation (checking param bounds, etc.) and
// returns an error if any check fails.
func (cfg BaseConfig) ValidateBasic() error {
	switch cfg.LogFormat {
	case LogFormatPlain, LogFormatJSON:
	default:
		return errors.New("unknown log format (must be 'plain' or 'json')")
	}
	switch cfg.Mode {
	case ModeFull, ModeValidator, ModeSeed:
	case "":
		return errors.New("no mode has been set")
	default:
		return fmt.Errorf("unknown mode: %v", cfg.Mode)
	}
	return nil
}

//-----------------------------------------------------------------------------
// RPCConfig

// RPCConfig defines the configuration options for the Tendermint RPC server
type RPCConfig struct {
	RootDir string `mapstructure:"home"`

	// TCP or UNIX socket address for the RPC server to listen on
	ListenAddress string `mapstructure:"laddr"`

	// A list of origins a cross-domain request can be executed from.
	// If the special '*' value is present in the list, all origins will be allowed.
	// An origin may contain a wildcard (*) to replace 0 or more characters (i.e.: http://*.domain.com).
	// Only one wildcard can be used per origin.
	CORSAllowedOrigins []string `mapstructure:"cors-allowed-origins"`

	// A list of methods the client is allowed to use with cross-domain requests.
	CORSAllowedMethods []string `mapstructure:"cors-allowed-methods"`

	// A list of non simple headers the client is allowed to use with cross-domain requests.
	CORSAllowedHeaders []string `mapstructure:"cors-allowed-headers"`

	// TCP or UNIX socket address for the gRPC server to listen on
	// NOTE: This server only supports /broadcast_tx_commit
	GRPCListenAddress string `mapstructure:"grpc-laddr"`

	// Maximum number of simultaneous connections.
	// Does not include RPC (HTTP&WebSocket) connections. See max-open-connections
	// If you want to accept a larger number than the default, make sure
	// you increase your OS limits.
	// 0 - unlimited.
	GRPCMaxOpenConnections int `mapstructure:"grpc-max-open-connections"`

	// Activate unsafe RPC commands like /dial-persistent-peers and /unsafe-flush-mempool
	Unsafe bool `mapstructure:"unsafe"`

	// Maximum number of simultaneous connections (including WebSocket).
	// Does not include gRPC connections. See grpc-max-open-connections
	// If you want to accept a larger number than the default, make sure
	// you increase your OS limits.
	// 0 - unlimited.
	// Should be < {ulimit -Sn} - {MaxNumInboundPeers} - {MaxNumOutboundPeers} - {N of wal, db and other open files}
	// 1024 - 40 - 10 - 50 = 924 = ~900
	MaxOpenConnections int `mapstructure:"max-open-connections"`

	// Maximum number of unique clientIDs that can /subscribe
	// If you're using /broadcast_tx_commit, set to the estimated maximum number
	// of broadcast_tx_commit calls per block.
	MaxSubscriptionClients int `mapstructure:"max-subscription-clients"`

	// Maximum number of unique queries a given client can /subscribe to
	// If you're using GRPC (or Local RPC client) and /broadcast_tx_commit, set
	// to the estimated maximum number of broadcast_tx_commit calls per block.
	MaxSubscriptionsPerClient int `mapstructure:"max-subscriptions-per-client"`

	// How long to wait for a tx to be committed during /broadcast_tx_commit
	// WARNING: Using a value larger than 10s will result in increasing the
	// global HTTP write timeout, which applies to all connections and endpoints.
	// See https://github.com/tendermint/tendermint/issues/3435
	TimeoutBroadcastTxCommit time.Duration `mapstructure:"timeout-broadcast-tx-commit"`

	// Maximum size of request body, in bytes
	MaxBodyBytes int64 `mapstructure:"max-body-bytes"`

	// Maximum size of request header, in bytes
	MaxHeaderBytes int `mapstructure:"max-header-bytes"`

	// The path to a file containing certificate that is used to create the HTTPS server.
	// Might be either absolute path or path related to Tendermint's config directory.
	//
	// If the certificate is signed by a certificate authority,
	// the certFile should be the concatenation of the server's certificate, any intermediates,
	// and the CA's certificate.
	//
	// NOTE: both tls-cert-file and tls-key-file must be present for Tendermint to create HTTPS server.
	// Otherwise, HTTP server is run.
	TLSCertFile string `mapstructure:"tls-cert-file"`

	// The path to a file containing matching private key that is used to create the HTTPS server.
	// Might be either absolute path or path related to tendermint's config directory.
	//
	// NOTE: both tls-cert-file and tls-key-file must be present for Tendermint to create HTTPS server.
	// Otherwise, HTTP server is run.
	TLSKeyFile string `mapstructure:"tls-key-file"`

	// pprof listen address (https://golang.org/pkg/net/http/pprof)
	PprofListenAddress string `mapstructure:"pprof-laddr"`
}

// DefaultRPCConfig returns a default configuration for the RPC server
func DefaultRPCConfig() *RPCConfig {
	return &RPCConfig{
		ListenAddress:          "tcp://127.0.0.1:26657",
		CORSAllowedOrigins:     []string{},
		CORSAllowedMethods:     []string{http.MethodHead, http.MethodGet, http.MethodPost},
		CORSAllowedHeaders:     []string{"Origin", "Accept", "Content-Type", "X-Requested-With", "X-Server-Time"},
		GRPCListenAddress:      "",
		GRPCMaxOpenConnections: 900,

		Unsafe:             false,
		MaxOpenConnections: 900,

		MaxSubscriptionClients:    100,
		MaxSubscriptionsPerClient: 5,
		TimeoutBroadcastTxCommit:  10 * time.Second,

		MaxBodyBytes:   int64(1000000), // 1MB
		MaxHeaderBytes: 1 << 20,        // same as the net/http default

		TLSCertFile: "",
		TLSKeyFile:  "",
	}
}

// TestRPCConfig returns a configuration for testing the RPC server
func TestRPCConfig() *RPCConfig {
	cfg := DefaultRPCConfig()
	cfg.ListenAddress = "tcp://127.0.0.1:36657"
	cfg.GRPCListenAddress = "tcp://127.0.0.1:36658"
	cfg.Unsafe = true
	return cfg
}

// ValidateBasic performs basic validation (checking param bounds, etc.) and
// returns an error if any check fails.
func (cfg *RPCConfig) ValidateBasic() error {
	if cfg.GRPCMaxOpenConnections < 0 {
		return errors.New("grpc-max-open-connections can't be negative")
	}
	if cfg.MaxOpenConnections < 0 {
		return errors.New("max-open-connections can't be negative")
	}
	if cfg.MaxSubscriptionClients < 0 {
		return errors.New("max-subscription-clients can't be negative")
	}
	if cfg.MaxSubscriptionsPerClient < 0 {
		return errors.New("max-subscriptions-per-client can't be negative")
	}
	if cfg.TimeoutBroadcastTxCommit < 0 {
		return errors.New("timeout-broadcast-tx-commit can't be negative")
	}
	if cfg.MaxBodyBytes < 0 {
		return errors.New("max-body-bytes can't be negative")
	}
	if cfg.MaxHeaderBytes < 0 {
		return errors.New("max-header-bytes can't be negative")
	}
	return nil
}

// IsCorsEnabled returns true if cross-origin resource sharing is enabled.
func (cfg *RPCConfig) IsCorsEnabled() bool {
	return len(cfg.CORSAllowedOrigins) != 0
}

func (cfg RPCConfig) KeyFile() string {
	path := cfg.TLSKeyFile
	if filepath.IsAbs(path) {
		return path
	}
	return rootify(filepath.Join(defaultConfigDir, path), cfg.RootDir)
}

func (cfg RPCConfig) CertFile() string {
	path := cfg.TLSCertFile
	if filepath.IsAbs(path) {
		return path
	}
	return rootify(filepath.Join(defaultConfigDir, path), cfg.RootDir)
}

func (cfg RPCConfig) IsTLSEnabled() bool {
	return cfg.TLSCertFile != "" && cfg.TLSKeyFile != ""
}

//-----------------------------------------------------------------------------
// P2PConfig

// P2PConfig defines the configuration options for the Tendermint peer-to-peer networking layer
type P2PConfig struct { //nolint: maligned
	RootDir string `mapstructure:"home"`

	// Address to listen for incoming connections
	ListenAddress string `mapstructure:"laddr"`

	// Address to advertise to peers for them to dial
	ExternalAddress string `mapstructure:"external-address"`

	// Comma separated list of seed nodes to connect to
	// We only use these if we can’t connect to peers in the addrbook
	Seeds string `mapstructure:"seeds"`

	// Comma separated list of nodes to keep persistent connections to
	PersistentPeers string `mapstructure:"persistent-peers"`

	// UPNP port forwarding
	UPNP bool `mapstructure:"upnp"`

	// Path to address book
	AddrBook string `mapstructure:"addr-book-file"`

	// Set true for strict address routability rules
	// Set false for private or local networks
	AddrBookStrict bool `mapstructure:"addr-book-strict"`

	// Maximum number of inbound peers
	MaxNumInboundPeers int `mapstructure:"max-num-inbound-peers"`

	// Maximum number of outbound peers to connect to, excluding persistent peers
	MaxNumOutboundPeers int `mapstructure:"max-num-outbound-peers"`

	// List of node IDs, to which a connection will be (re)established ignoring any existing limits
	UnconditionalPeerIDs string `mapstructure:"unconditional-peer-ids"`

	// Maximum pause when redialing a persistent peer (if zero, exponential backoff is used)
	PersistentPeersMaxDialPeriod time.Duration `mapstructure:"persistent-peers-max-dial-period"`

	// Time to wait before flushing messages out on the connection
	FlushThrottleTimeout time.Duration `mapstructure:"flush-throttle-timeout"`

	// Maximum size of a message packet payload, in bytes
	MaxPacketMsgPayloadSize int `mapstructure:"max-packet-msg-payload-size"`

	// Rate at which packets can be sent, in bytes/second
	SendRate int64 `mapstructure:"send-rate"`

	// Rate at which packets can be received, in bytes/second
	RecvRate int64 `mapstructure:"recv-rate"`

	// Set true to enable the peer-exchange reactor
	PexReactor bool `mapstructure:"pex"`

	// Comma separated list of peer IDs to keep private (will not be gossiped to
	// other peers)
	PrivatePeerIDs string `mapstructure:"private-peer-ids"`

	// Toggle to disable guard against peers connecting from the same ip.
	AllowDuplicateIP bool `mapstructure:"allow-duplicate-ip"`

	// Peer connection configuration.
	HandshakeTimeout time.Duration `mapstructure:"handshake-timeout"`
	DialTimeout      time.Duration `mapstructure:"dial-timeout"`

	// Testing params.
	// Force dial to fail
	TestDialFail bool `mapstructure:"test-dial-fail"`

	// Mostly for testing, use rather than environment variables
	// to turn on the new P2P stack.
	UseNewP2P bool `mapstructure:"use-new-p2p"`

	// Makes it possible to configure which queue backend the p2p
	// layer uses. Options are: "fifo", "priority" and "wdrr",
	// with the default being "fifo".
	QueueType string `mapstructure:"queue-type"`
}

// DefaultP2PConfig returns a default configuration for the peer-to-peer layer
func DefaultP2PConfig() *P2PConfig {
	return &P2PConfig{
		ListenAddress:                "tcp://0.0.0.0:26656",
		ExternalAddress:              "",
		UPNP:                         false,
		AddrBook:                     defaultAddrBookPath,
		AddrBookStrict:               true,
		MaxNumInboundPeers:           40,
		MaxNumOutboundPeers:          10,
		PersistentPeersMaxDialPeriod: 0 * time.Second,
		FlushThrottleTimeout:         100 * time.Millisecond,
		// The MTU (Maximum Transmission Unit) for Ethernet is 1500 bytes.
		// The IP header and the TCP header take up 20 bytes each at least (unless
		// optional header fields are used) and thus the max for (non-Jumbo frame)
		// Ethernet is 1500 - 20 -20 = 1460
		// Source: https://stackoverflow.com/a/3074427/820520
		MaxPacketMsgPayloadSize: 1400,
		SendRate:                5120000, // 5 mB/s
		RecvRate:                5120000, // 5 mB/s
		PexReactor:              true,
		AllowDuplicateIP:        false,
		HandshakeTimeout:        20 * time.Second,
		DialTimeout:             3 * time.Second,
		TestDialFail:            false,
		QueueType:               "priority",
	}
}

// TestP2PConfig returns a configuration for testing the peer-to-peer layer
func TestP2PConfig() *P2PConfig {
	cfg := DefaultP2PConfig()
	cfg.ListenAddress = "tcp://127.0.0.1:36656"
	cfg.FlushThrottleTimeout = 10 * time.Millisecond
	cfg.AllowDuplicateIP = true
	return cfg
}

// AddrBookFile returns the full path to the address book
func (cfg *P2PConfig) AddrBookFile() string {
	return rootify(cfg.AddrBook, cfg.RootDir)
}

// ValidateBasic performs basic validation (checking param bounds, etc.) and
// returns an error if any check fails.
func (cfg *P2PConfig) ValidateBasic() error {
	if cfg.MaxNumInboundPeers < 0 {
		return errors.New("max-num-inbound-peers can't be negative")
	}
	if cfg.MaxNumOutboundPeers < 0 {
		return errors.New("max-num-outbound-peers can't be negative")
	}
	if cfg.FlushThrottleTimeout < 0 {
		return errors.New("flush-throttle-timeout can't be negative")
	}
	if cfg.PersistentPeersMaxDialPeriod < 0 {
		return errors.New("persistent-peers-max-dial-period can't be negative")
	}
	if cfg.MaxPacketMsgPayloadSize < 0 {
		return errors.New("max-packet-msg-payload-size can't be negative")
	}
	if cfg.SendRate < 0 {
		return errors.New("send-rate can't be negative")
	}
	if cfg.RecvRate < 0 {
		return errors.New("recv-rate can't be negative")
	}
	return nil
}

//-----------------------------------------------------------------------------
// MempoolConfig

// MempoolConfig defines the configuration options for the Tendermint mempool
type MempoolConfig struct {
	RootDir   string `mapstructure:"home"`
	Recheck   bool   `mapstructure:"recheck"`
	Broadcast bool   `mapstructure:"broadcast"`
	WalPath   string `mapstructure:"wal-dir"`
	// Maximum number of transactions in the mempool
	Size int `mapstructure:"size"`
	// Limit the total size of all txs in the mempool.
	// This only accounts for raw transactions (e.g. given 1MB transactions and
	// max-txs-bytes=5MB, mempool will only accept 5 transactions).
	MaxTxsBytes int64 `mapstructure:"max-txs-bytes"`
	// Size of the cache (used to filter transactions we saw earlier) in transactions
	CacheSize int `mapstructure:"cache-size"`
	// Do not remove invalid transactions from the cache (default: false)
	// Set to true if it's not possible for any invalid transaction to become
	// valid again in the future.
	KeepInvalidTxsInCache bool `mapstructure:"keep-invalid-txs-in-cache"`
	// Maximum size of a single transaction
	// NOTE: the max size of a tx transmitted over the network is {max-tx-bytes}.
	MaxTxBytes int `mapstructure:"max-tx-bytes"`
	// Maximum size of a batch of transactions to send to a peer
	// Including space needed by encoding (one varint per transaction).
	// XXX: Unused due to https://github.com/tendermint/tendermint/issues/5796
	MaxBatchBytes int `mapstructure:"max-batch-bytes"`
}

// DefaultMempoolConfig returns a default configuration for the Tendermint mempool
func DefaultMempoolConfig() *MempoolConfig {
	return &MempoolConfig{
		Recheck:   true,
		Broadcast: true,
		WalPath:   "",
		// Each signature verification takes .5ms, Size reduced until we implement
		// ABCI Recheck
		Size:        5000,
		MaxTxsBytes: 1024 * 1024 * 1024, // 1GB
		CacheSize:   10000,
		MaxTxBytes:  1024 * 1024, // 1MB
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

// WalEnabled returns true if the WAL is enabled.
func (cfg *MempoolConfig) WalEnabled() bool {
	return cfg.WalPath != ""
}

// ValidateBasic performs basic validation (checking param bounds, etc.) and
// returns an error if any check fails.
func (cfg *MempoolConfig) ValidateBasic() error {
	if cfg.Size < 0 {
		return errors.New("size can't be negative")
	}
	if cfg.MaxTxsBytes < 0 {
		return errors.New("max-txs-bytes can't be negative")
	}
	if cfg.CacheSize < 0 {
		return errors.New("cache-size can't be negative")
	}
	if cfg.MaxTxBytes < 0 {
		return errors.New("max-tx-bytes can't be negative")
	}
	return nil
}

//-----------------------------------------------------------------------------
// StateSyncConfig

// StateSyncConfig defines the configuration for the Tendermint state sync service
type StateSyncConfig struct {
	Enable        bool          `mapstructure:"enable"`
	TempDir       string        `mapstructure:"temp-dir"`
	RPCServers    []string      `mapstructure:"rpc-servers"`
	TrustPeriod   time.Duration `mapstructure:"trust-period"`
	TrustHeight   int64         `mapstructure:"trust-height"`
	TrustHash     string        `mapstructure:"trust-hash"`
	DiscoveryTime time.Duration `mapstructure:"discovery-time"`
}

func (cfg *StateSyncConfig) TrustHashBytes() []byte {
	// validated in ValidateBasic, so we can safely panic here
	bytes, err := hex.DecodeString(cfg.TrustHash)
	if err != nil {
		panic(err)
	}
	return bytes
}

// DefaultStateSyncConfig returns a default configuration for the state sync service
func DefaultStateSyncConfig() *StateSyncConfig {
	return &StateSyncConfig{
		TrustPeriod:   168 * time.Hour,
		DiscoveryTime: 15 * time.Second,
	}
}

// TestFastSyncConfig returns a default configuration for the state sync service
func TestStateSyncConfig() *StateSyncConfig {
	return DefaultStateSyncConfig()
}

// ValidateBasic performs basic validation.
func (cfg *StateSyncConfig) ValidateBasic() error {
	if cfg.Enable {
		if len(cfg.RPCServers) == 0 {
			return errors.New("rpc-servers is required")
		}
		if len(cfg.RPCServers) < 2 {
			return errors.New("at least two rpc-servers entries is required")
		}
		for _, server := range cfg.RPCServers {
			if len(server) == 0 {
				return errors.New("found empty rpc-servers entry")
			}
		}
		if cfg.TrustPeriod <= 0 {
			return errors.New("trusted-period is required")
		}
		if cfg.TrustHeight <= 0 {
			return errors.New("trusted-height is required")
		}
		if len(cfg.TrustHash) == 0 {
			return errors.New("trusted-hash is required")
		}
		_, err := hex.DecodeString(cfg.TrustHash)
		if err != nil {
			return fmt.Errorf("invalid trusted-hash: %w", err)
		}
	}
	return nil
}

//-----------------------------------------------------------------------------
// FastSyncConfig

// FastSyncConfig defines the configuration for the Tendermint fast sync service
type FastSyncConfig struct {
	Version string `mapstructure:"version"`
}

// DefaultFastSyncConfig returns a default configuration for the fast sync service
func DefaultFastSyncConfig() *FastSyncConfig {
	return &FastSyncConfig{
		Version: BlockchainV0,
	}
}

// TestFastSyncConfig returns a default configuration for the fast sync.
func TestFastSyncConfig() *FastSyncConfig {
	return DefaultFastSyncConfig()
}

// ValidateBasic performs basic validation.
func (cfg *FastSyncConfig) ValidateBasic() error {
	switch cfg.Version {
	case BlockchainV0:
		return nil
	case BlockchainV2:
		return nil
	default:
		return fmt.Errorf("unknown fastsync version %s", cfg.Version)
	}
}

//-----------------------------------------------------------------------------
// ConsensusConfig

// ConsensusConfig defines the configuration for the Tendermint consensus service,
// including timeouts and details about the WAL and the block structure.
type ConsensusConfig struct {
	RootDir string `mapstructure:"home"`
	WalPath string `mapstructure:"wal-file"`
	walFile string // overrides WalPath if set

	// TODO: remove timeout configs, these should be global not local
	// How long we wait for a proposal block before prevoting nil
	TimeoutPropose time.Duration `mapstructure:"timeout-propose"`
	// How much timeout-propose increases with each round
	TimeoutProposeDelta time.Duration `mapstructure:"timeout-propose-delta"`
	// How long we wait after receiving +2/3 prevotes for “anything” (ie. not a single block or nil)
	TimeoutPrevote time.Duration `mapstructure:"timeout-prevote"`
	// How much the timeout-prevote increases with each round
	TimeoutPrevoteDelta time.Duration `mapstructure:"timeout-prevote-delta"`
	// How long we wait after receiving +2/3 precommits for “anything” (ie. not a single block or nil)
	TimeoutPrecommit time.Duration `mapstructure:"timeout-precommit"`
	// How much the timeout-precommit increases with each round
	TimeoutPrecommitDelta time.Duration `mapstructure:"timeout-precommit-delta"`
	// How long we wait after committing a block, before starting on the new
	// height (this gives us a chance to receive some more precommits, even
	// though we already have +2/3).
	TimeoutCommit time.Duration `mapstructure:"timeout-commit"`

	// Make progress as soon as we have all the precommits (as if TimeoutCommit = 0)
	SkipTimeoutCommit bool `mapstructure:"skip-timeout-commit"`

	// EmptyBlocks mode and possible interval between empty blocks
	CreateEmptyBlocks         bool          `mapstructure:"create-empty-blocks"`
	CreateEmptyBlocksInterval time.Duration `mapstructure:"create-empty-blocks-interval"`

	// Reactor sleep duration parameters
	PeerGossipSleepDuration     time.Duration `mapstructure:"peer-gossip-sleep-duration"`
	PeerQueryMaj23SleepDuration time.Duration `mapstructure:"peer-query-maj23-sleep-duration"`

	DoubleSignCheckHeight int64 `mapstructure:"double-sign-check-height"`
}

// DefaultConsensusConfig returns a default configuration for the consensus service
func DefaultConsensusConfig() *ConsensusConfig {
	return &ConsensusConfig{
		WalPath:                     filepath.Join(defaultDataDir, "cs.wal", "wal"),
		TimeoutPropose:              3000 * time.Millisecond,
		TimeoutProposeDelta:         500 * time.Millisecond,
		TimeoutPrevote:              1000 * time.Millisecond,
		TimeoutPrevoteDelta:         500 * time.Millisecond,
		TimeoutPrecommit:            1000 * time.Millisecond,
		TimeoutPrecommitDelta:       500 * time.Millisecond,
		TimeoutCommit:               1000 * time.Millisecond,
		SkipTimeoutCommit:           false,
		CreateEmptyBlocks:           true,
		CreateEmptyBlocksInterval:   0 * time.Second,
		PeerGossipSleepDuration:     100 * time.Millisecond,
		PeerQueryMaj23SleepDuration: 2000 * time.Millisecond,
		DoubleSignCheckHeight:       int64(0),
	}
}

// TestConsensusConfig returns a configuration for testing the consensus service
func TestConsensusConfig() *ConsensusConfig {
	cfg := DefaultConsensusConfig()
	cfg.TimeoutPropose = 40 * time.Millisecond
	cfg.TimeoutProposeDelta = 1 * time.Millisecond
	cfg.TimeoutPrevote = 10 * time.Millisecond
	cfg.TimeoutPrevoteDelta = 1 * time.Millisecond
	cfg.TimeoutPrecommit = 10 * time.Millisecond
	cfg.TimeoutPrecommitDelta = 1 * time.Millisecond
	cfg.TimeoutCommit = 10 * time.Millisecond
	cfg.SkipTimeoutCommit = true
	cfg.PeerGossipSleepDuration = 5 * time.Millisecond
	cfg.PeerQueryMaj23SleepDuration = 250 * time.Millisecond
	cfg.DoubleSignCheckHeight = int64(0)
	return cfg
}

// WaitForTxs returns true if the consensus should wait for transactions before entering the propose step
func (cfg *ConsensusConfig) WaitForTxs() bool {
	return !cfg.CreateEmptyBlocks || cfg.CreateEmptyBlocksInterval > 0
}

// Propose returns the amount of time to wait for a proposal
func (cfg *ConsensusConfig) Propose(round int32) time.Duration {
	return time.Duration(
		cfg.TimeoutPropose.Nanoseconds()+cfg.TimeoutProposeDelta.Nanoseconds()*int64(round),
	) * time.Nanosecond
}

// Prevote returns the amount of time to wait for straggler votes after receiving any +2/3 prevotes
func (cfg *ConsensusConfig) Prevote(round int32) time.Duration {
	return time.Duration(
		cfg.TimeoutPrevote.Nanoseconds()+cfg.TimeoutPrevoteDelta.Nanoseconds()*int64(round),
	) * time.Nanosecond
}

// Precommit returns the amount of time to wait for straggler votes after receiving any +2/3 precommits
func (cfg *ConsensusConfig) Precommit(round int32) time.Duration {
	return time.Duration(
		cfg.TimeoutPrecommit.Nanoseconds()+cfg.TimeoutPrecommitDelta.Nanoseconds()*int64(round),
	) * time.Nanosecond
}

// Commit returns the amount of time to wait for straggler votes after receiving +2/3 precommits
// for a single block (ie. a commit).
func (cfg *ConsensusConfig) Commit(t time.Time) time.Time {
	return t.Add(cfg.TimeoutCommit)
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

// ValidateBasic performs basic validation (checking param bounds, etc.) and
// returns an error if any check fails.
func (cfg *ConsensusConfig) ValidateBasic() error {
	if cfg.TimeoutPropose < 0 {
		return errors.New("timeout-propose can't be negative")
	}
	if cfg.TimeoutProposeDelta < 0 {
		return errors.New("timeout-propose-delta can't be negative")
	}
	if cfg.TimeoutPrevote < 0 {
		return errors.New("timeout-prevote can't be negative")
	}
	if cfg.TimeoutPrevoteDelta < 0 {
		return errors.New("timeout-prevote-delta can't be negative")
	}
	if cfg.TimeoutPrecommit < 0 {
		return errors.New("timeout-precommit can't be negative")
	}
	if cfg.TimeoutPrecommitDelta < 0 {
		return errors.New("timeout-precommit-delta can't be negative")
	}
	if cfg.TimeoutCommit < 0 {
		return errors.New("timeout-commit can't be negative")
	}
	if cfg.CreateEmptyBlocksInterval < 0 {
		return errors.New("create-empty-blocks-interval can't be negative")
	}
	if cfg.PeerGossipSleepDuration < 0 {
		return errors.New("peer-gossip-sleep-duration can't be negative")
	}
	if cfg.PeerQueryMaj23SleepDuration < 0 {
		return errors.New("peer-query-maj23-sleep-duration can't be negative")
	}
	if cfg.DoubleSignCheckHeight < 0 {
		return errors.New("double-sign-check-height can't be negative")
	}
	return nil
}

//-----------------------------------------------------------------------------
// TxIndexConfig
// Remember that Event has the following structure:
// type: [
//  key: value,
//  ...
// ]
//
// CompositeKeys are constructed by `type.key`
// TxIndexConfig defines the configuration for the transaction indexer,
// including composite keys to index.
type TxIndexConfig struct {
	// What indexer to use for transactions
	//
	// Options:
	//   1) "null"
	//   2) "kv" (default) - the simplest possible indexer,
	//      backed by key-value storage (defaults to levelDB; see DBBackend).
	Indexer string `mapstructure:"indexer"`
}

// DefaultTxIndexConfig returns a default configuration for the transaction indexer.
func DefaultTxIndexConfig() *TxIndexConfig {
	return &TxIndexConfig{
		Indexer: "kv",
	}
}

// TestTxIndexConfig returns a default configuration for the transaction indexer.
func TestTxIndexConfig() *TxIndexConfig {
	return DefaultTxIndexConfig()
}

//-----------------------------------------------------------------------------
// InstrumentationConfig

// InstrumentationConfig defines the configuration for metrics reporting.
type InstrumentationConfig struct {
	// When true, Prometheus metrics are served under /metrics on
	// PrometheusListenAddr.
	// Check out the documentation for the list of available metrics.
	Prometheus bool `mapstructure:"prometheus"`

	// Address to listen for Prometheus collector(s) connections.
	PrometheusListenAddr string `mapstructure:"prometheus-listen-addr"`

	// Maximum number of simultaneous connections.
	// If you want to accept a larger number than the default, make sure
	// you increase your OS limits.
	// 0 - unlimited.
	MaxOpenConnections int `mapstructure:"max-open-connections"`

	// Instrumentation namespace.
	Namespace string `mapstructure:"namespace"`
}

// DefaultInstrumentationConfig returns a default configuration for metrics
// reporting.
func DefaultInstrumentationConfig() *InstrumentationConfig {
	return &InstrumentationConfig{
		Prometheus:           false,
		PrometheusListenAddr: ":26660",
		MaxOpenConnections:   3,
		Namespace:            "tendermint",
	}
}

// TestInstrumentationConfig returns a default configuration for metrics
// reporting.
func TestInstrumentationConfig() *InstrumentationConfig {
	return DefaultInstrumentationConfig()
}

// ValidateBasic performs basic validation (checking param bounds, etc.) and
// returns an error if any check fails.
func (cfg *InstrumentationConfig) ValidateBasic() error {
	if cfg.MaxOpenConnections < 0 {
		return errors.New("max-open-connections can't be negative")
	}
	return nil
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
