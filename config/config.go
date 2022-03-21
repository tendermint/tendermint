package config

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	"github.com/tendermint/tendermint/types"
)

const (
	// FuzzModeDrop is a mode in which we randomly drop reads/writes, connections or sleep
	FuzzModeDrop = iota
	// FuzzModeDelay is a mode in which we randomly sleep
	FuzzModeDelay

	// DefaultLogLevel defines a default log level as INFO.
	DefaultLogLevel = "info"

	ModeFull      = "full"
	ModeValidator = "validator"
	ModeSeed      = "seed"
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

	defaultNodeKeyName = "node_key.json"

	defaultConfigFilePath   = filepath.Join(defaultConfigDir, defaultConfigFileName)
	defaultGenesisJSONPath  = filepath.Join(defaultConfigDir, defaultGenesisJSONName)
	defaultPrivValKeyPath   = filepath.Join(defaultConfigDir, defaultPrivValKeyName)
	defaultPrivValStatePath = filepath.Join(defaultDataDir, defaultPrivValStateName)

	defaultNodeKeyPath = filepath.Join(defaultConfigDir, defaultNodeKeyName)
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
	Consensus       *ConsensusConfig       `mapstructure:"consensus"`
	TxIndex         *TxIndexConfig         `mapstructure:"tx-index"`
	Instrumentation *InstrumentationConfig `mapstructure:"instrumentation"`
	PrivValidator   *PrivValidatorConfig   `mapstructure:"priv-validator"`
}

// DefaultConfig returns a default configuration for a Tendermint node
func DefaultConfig() *Config {
	return &Config{
		BaseConfig:      DefaultBaseConfig(),
		RPC:             DefaultRPCConfig(),
		P2P:             DefaultP2PConfig(),
		Mempool:         DefaultMempoolConfig(),
		StateSync:       DefaultStateSyncConfig(),
		Consensus:       DefaultConsensusConfig(),
		TxIndex:         DefaultTxIndexConfig(),
		Instrumentation: DefaultInstrumentationConfig(),
		PrivValidator:   DefaultPrivValidatorConfig(),
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
		Consensus:       TestConsensusConfig(),
		TxIndex:         TestTxIndexConfig(),
		Instrumentation: TestInstrumentationConfig(),
		PrivValidator:   DefaultPrivValidatorConfig(),
	}
}

// SetRoot sets the RootDir for all Config structs
func (cfg *Config) SetRoot(root string) *Config {
	cfg.BaseConfig.RootDir = root
	cfg.RPC.RootDir = root
	cfg.P2P.RootDir = root
	cfg.Mempool.RootDir = root
	cfg.Consensus.RootDir = root
	cfg.PrivValidator.RootDir = root
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
	if err := cfg.Mempool.ValidateBasic(); err != nil {
		return fmt.Errorf("error in [mempool] section: %w", err)
	}
	if err := cfg.StateSync.ValidateBasic(); err != nil {
		return fmt.Errorf("error in [statesync] section: %w", err)
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

	Other map[string]interface{} `mapstructure:",remain"`
}

// DefaultBaseConfig returns a default base configuration for a Tendermint node
func DefaultBaseConfig() BaseConfig {
	return BaseConfig{
		Genesis:     defaultGenesisJSONPath,
		NodeKey:     defaultNodeKeyPath,
		Mode:        defaultMode,
		Moniker:     defaultMoniker,
		ProxyApp:    "tcp://127.0.0.1:26658",
		ABCI:        "socket",
		LogLevel:    DefaultLogLevel,
		LogFormat:   log.LogFormatPlain,
		FilterPeers: false,
		DBBackend:   "goleveldb",
		DBPath:      "data",
	}
}

// TestBaseConfig returns a base configuration for testing a Tendermint node
func TestBaseConfig() BaseConfig {
	cfg := DefaultBaseConfig()
	cfg.chainID = "tendermint_test"
	cfg.Mode = ModeValidator
	cfg.ProxyApp = "kvstore"
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

// NodeKeyFile returns the full path to the node_key.json file
func (cfg BaseConfig) NodeKeyFile() string {
	return rootify(cfg.NodeKey, cfg.RootDir)
}

// LoadNodeKey loads NodeKey located in filePath.
func (cfg BaseConfig) LoadNodeKeyID() (types.NodeID, error) {
	jsonBytes, err := os.ReadFile(cfg.NodeKeyFile())
	if err != nil {
		return "", err
	}
	nodeKey := types.NodeKey{}
	err = json.Unmarshal(jsonBytes, &nodeKey)
	if err != nil {
		return "", err
	}
	nodeKey.ID = types.NodeIDFromPubKey(nodeKey.PubKey())
	return nodeKey.ID, nil
}

// LoadOrGenNodeKey attempts to load the NodeKey from the given filePath. If
// the file does not exist, it generates and saves a new NodeKey.
func (cfg BaseConfig) LoadOrGenNodeKeyID() (types.NodeID, error) {
	if tmos.FileExists(cfg.NodeKeyFile()) {
		nodeKey, err := cfg.LoadNodeKeyID()
		if err != nil {
			return "", err
		}
		return nodeKey, nil
	}

	nodeKey := types.GenNodeKey()

	if err := nodeKey.SaveAs(cfg.NodeKeyFile()); err != nil {
		return "", err
	}

	return nodeKey.ID, nil
}

// DBDir returns the full path to the database directory
func (cfg BaseConfig) DBDir() string {
	return rootify(cfg.DBPath, cfg.RootDir)
}

// ValidateBasic performs basic validation (checking param bounds, etc.) and
// returns an error if any check fails.
func (cfg BaseConfig) ValidateBasic() error {
	switch cfg.LogFormat {
	case log.LogFormatJSON, log.LogFormatText, log.LogFormatPlain:
	default:
		return errors.New("unknown log format (must be 'plain', 'text' or 'json')")
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
// PrivValidatorConfig

// PrivValidatorConfig defines the configuration parameters for running a validator
type PrivValidatorConfig struct {
	RootDir string `mapstructure:"home"`

	// Path to the JSON file containing the private key to use as a validator in the consensus protocol
	Key string `mapstructure:"key-file"`

	// Path to the JSON file containing the last sign state of a validator
	State string `mapstructure:"state-file"`

	// TCP or UNIX socket address for Tendermint to listen on for
	// connections from an external PrivValidator process
	ListenAddr string `mapstructure:"laddr"`

	// Client certificate generated while creating needed files for secure connection.
	// If a remote validator address is provided but no certificate, the connection will be insecure
	ClientCertificate string `mapstructure:"client-certificate-file"`

	// Client key generated while creating certificates for secure connection
	ClientKey string `mapstructure:"client-key-file"`

	// Path Root Certificate Authority used to sign both client and server certificates
	RootCA string `mapstructure:"root-ca-file"`
}

// DefaultBaseConfig returns a default private validator configuration
// for a Tendermint node.
func DefaultPrivValidatorConfig() *PrivValidatorConfig {
	return &PrivValidatorConfig{
		Key:   defaultPrivValKeyPath,
		State: defaultPrivValStatePath,
	}
}

// ClientKeyFile returns the full path to the priv_validator_key.json file
func (cfg *PrivValidatorConfig) ClientKeyFile() string {
	return rootify(cfg.ClientKey, cfg.RootDir)
}

// ClientCertificateFile returns the full path to the priv_validator_key.json file
func (cfg *PrivValidatorConfig) ClientCertificateFile() string {
	return rootify(cfg.ClientCertificate, cfg.RootDir)
}

// CertificateAuthorityFile returns the full path to the priv_validator_key.json file
func (cfg *PrivValidatorConfig) RootCAFile() string {
	return rootify(cfg.RootCA, cfg.RootDir)
}

// KeyFile returns the full path to the priv_validator_key.json file
func (cfg *PrivValidatorConfig) KeyFile() string {
	return rootify(cfg.Key, cfg.RootDir)
}

// StateFile returns the full path to the priv_validator_state.json file
func (cfg *PrivValidatorConfig) StateFile() string {
	return rootify(cfg.State, cfg.RootDir)
}

func (cfg *PrivValidatorConfig) AreSecurityOptionsPresent() bool {
	switch {
	case cfg.RootCA == "":
		return false
	case cfg.ClientKey == "":
		return false
	case cfg.ClientCertificate == "":
		return false
	default:
		return true
	}
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

	// Activate unsafe RPC commands like /dial-persistent-peers and /unsafe-flush-mempool
	Unsafe bool `mapstructure:"unsafe"`

	// Maximum number of simultaneous connections (including WebSocket).
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
	// If you're using a Local RPC client and /broadcast_tx_commit, set this
	// to the estimated maximum number of broadcast_tx_commit calls per block.
	MaxSubscriptionsPerClient int `mapstructure:"max-subscriptions-per-client"`

	// If true, disable the websocket interface to the RPC service.  This has
	// the effect of disabling the /subscribe, /unsubscribe, and /unsubscribe_all
	// methods for event subscription.
	//
	// EXPERIMENTAL: This setting will be removed in Tendermint v0.37.
	ExperimentalDisableWebsocket bool `mapstructure:"experimental-disable-websocket"`

	// The time window size for the event log. All events up to this long before
	// the latest (up to EventLogMaxItems) will be available for subscribers to
	// fetch via the /events method.  If 0 (the default) the event log and the
	// /events RPC method are disabled.
	EventLogWindowSize time.Duration `mapstructure:"event-log-window-size"`

	// The maxiumum number of events that may be retained by the event log.  If
	// this value is 0, no upper limit is set. Otherwise, items in excess of
	// this number will be discarded from the event log.
	//
	// Warning: This setting is a safety valve. Setting it too low may cause
	// subscribers to miss events.  Try to choose a value higher than the
	// maximum worst-case expected event load within the chosen window size in
	// ordinary operation.
	//
	// For example, if the window size is 10 minutes and the node typically
	// averages 1000 events per ten minutes, but with occasional known spikes of
	// up to 2000, choose a value > 2000.
	EventLogMaxItems int `mapstructure:"event-log-max-items"`

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
		ListenAddress:      "tcp://127.0.0.1:26657",
		CORSAllowedOrigins: []string{},
		CORSAllowedMethods: []string{http.MethodHead, http.MethodGet, http.MethodPost},
		CORSAllowedHeaders: []string{"Origin", "Accept", "Content-Type", "X-Requested-With", "X-Server-Time"},

		Unsafe:             false,
		MaxOpenConnections: 900,

		// Settings for event subscription.
		MaxSubscriptionClients:       100,
		MaxSubscriptionsPerClient:    5,
		ExperimentalDisableWebsocket: false, // compatible with TM v0.35 and earlier
		EventLogWindowSize:           0,     // disables /events RPC by default
		EventLogMaxItems:             0,

		TimeoutBroadcastTxCommit: 10 * time.Second,

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
	cfg.Unsafe = true
	return cfg
}

// ValidateBasic performs basic validation (checking param bounds, etc.) and
// returns an error if any check fails.
func (cfg *RPCConfig) ValidateBasic() error {
	if cfg.MaxOpenConnections < 0 {
		return errors.New("max-open-connections can't be negative")
	}
	if cfg.MaxSubscriptionClients < 0 {
		return errors.New("max-subscription-clients can't be negative")
	}
	if cfg.MaxSubscriptionsPerClient < 0 {
		return errors.New("max-subscriptions-per-client can't be negative")
	}
	if cfg.EventLogWindowSize < 0 {
		return errors.New("event-log-window-size must not be negative")
	}
	if cfg.EventLogMaxItems < 0 {
		return errors.New("event-log-max-items must not be negative")
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
	//
	// Deprecated: This value is not used by the new PEX reactor. Use
	// BootstrapPeers instead.
	//
	// TODO(#5670): Remove once the p2p refactor is complete.
	Seeds string `mapstructure:"seeds"`

	// Comma separated list of peers to be added to the peer store
	// on startup. Either BootstrapPeers or PersistentPeers are
	// needed for peer discovery
	BootstrapPeers string `mapstructure:"bootstrap-peers"`

	// Comma separated list of nodes to keep persistent connections to
	PersistentPeers string `mapstructure:"persistent-peers"`

	// UPNP port forwarding
	UPNP bool `mapstructure:"upnp"`

	// MaxConnections defines the maximum number of connected peers (inbound and
	// outbound).
	MaxConnections uint16 `mapstructure:"max-connections"`

	// MaxIncomingConnectionAttempts rate limits the number of incoming connection
	// attempts per IP address.
	MaxIncomingConnectionAttempts uint `mapstructure:"max-incoming-connection-attempts"`

	// Set true to enable the peer-exchange reactor
	PexReactor bool `mapstructure:"pex"`

	// Comma separated list of peer IDs to keep private (will not be gossiped to
	// other peers)
	PrivatePeerIDs string `mapstructure:"private-peer-ids"`

	// Toggle to disable guard against peers connecting from the same ip.
	AllowDuplicateIP bool `mapstructure:"allow-duplicate-ip"`

	// Time to wait before flushing messages out on the connection
	FlushThrottleTimeout time.Duration `mapstructure:"flush-throttle-timeout"`

	// Maximum size of a message packet payload, in bytes
	MaxPacketMsgPayloadSize int `mapstructure:"max-packet-msg-payload-size"`

	// Rate at which packets can be sent, in bytes/second
	SendRate int64 `mapstructure:"send-rate"`

	// Rate at which packets can be received, in bytes/second
	RecvRate int64 `mapstructure:"recv-rate"`

	// Peer connection configuration.
	HandshakeTimeout time.Duration `mapstructure:"handshake-timeout"`
	DialTimeout      time.Duration `mapstructure:"dial-timeout"`

	// Testing params.
	// Force dial to fail
	TestDialFail bool `mapstructure:"test-dial-fail"`

	// Makes it possible to configure which queue backend the p2p
	// layer uses. Options are: "fifo" and "priority",
	// with the default being "priority".
	QueueType string `mapstructure:"queue-type"`
}

// DefaultP2PConfig returns a default configuration for the peer-to-peer layer
func DefaultP2PConfig() *P2PConfig {
	return &P2PConfig{
		ListenAddress:                 "tcp://0.0.0.0:26656",
		ExternalAddress:               "",
		UPNP:                          false,
		MaxConnections:                64,
		MaxIncomingConnectionAttempts: 100,
		FlushThrottleTimeout:          100 * time.Millisecond,
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

// ValidateBasic performs basic validation (checking param bounds, etc.) and
// returns an error if any check fails.
func (cfg *P2PConfig) ValidateBasic() error {
	if cfg.FlushThrottleTimeout < 0 {
		return errors.New("flush-throttle-timeout can't be negative")
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

// TestP2PConfig returns a configuration for testing the peer-to-peer layer
func TestP2PConfig() *P2PConfig {
	cfg := DefaultP2PConfig()
	cfg.ListenAddress = "tcp://127.0.0.1:36656"
	cfg.AllowDuplicateIP = true
	cfg.FlushThrottleTimeout = 10 * time.Millisecond
	return cfg
}

//-----------------------------------------------------------------------------
// MempoolConfig

// MempoolConfig defines the configuration options for the Tendermint mempool.
type MempoolConfig struct {
	RootDir   string `mapstructure:"home"`
	Recheck   bool   `mapstructure:"recheck"`
	Broadcast bool   `mapstructure:"broadcast"`

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

	// TTLDuration, if non-zero, defines the maximum amount of time a transaction
	// can exist for in the mempool.
	//
	// Note, if TTLNumBlocks is also defined, a transaction will be removed if it
	// has existed in the mempool at least TTLNumBlocks number of blocks or if it's
	// insertion time into the mempool is beyond TTLDuration.
	TTLDuration time.Duration `mapstructure:"ttl-duration"`

	// TTLNumBlocks, if non-zero, defines the maximum number of blocks a transaction
	// can exist for in the mempool.
	//
	// Note, if TTLDuration is also defined, a transaction will be removed if it
	// has existed in the mempool at least TTLNumBlocks number of blocks or if
	// it's insertion time into the mempool is beyond TTLDuration.
	TTLNumBlocks int64 `mapstructure:"ttl-num-blocks"`
}

// DefaultMempoolConfig returns a default configuration for the Tendermint mempool.
func DefaultMempoolConfig() *MempoolConfig {
	return &MempoolConfig{
		Recheck:   true,
		Broadcast: true,
		// Each signature verification takes .5ms, Size reduced until we implement
		// ABCI Recheck
		Size:         5000,
		MaxTxsBytes:  1024 * 1024 * 1024, // 1GB
		CacheSize:    10000,
		MaxTxBytes:   1024 * 1024, // 1MB
		TTLDuration:  0 * time.Second,
		TTLNumBlocks: 0,
	}
}

// TestMempoolConfig returns a configuration for testing the Tendermint mempool
func TestMempoolConfig() *MempoolConfig {
	cfg := DefaultMempoolConfig()
	cfg.CacheSize = 1000
	return cfg
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
	if cfg.TTLDuration < 0 {
		return errors.New("ttl-duration can't be negative")
	}
	if cfg.TTLNumBlocks < 0 {
		return errors.New("ttl-num-blocks can't be negative")
	}

	return nil
}

//-----------------------------------------------------------------------------
// StateSyncConfig

// StateSyncConfig defines the configuration for the Tendermint state sync service
type StateSyncConfig struct {
	// State sync rapidly bootstraps a new node by discovering, fetching, and restoring a
	// state machine snapshot from peers instead of fetching and replaying historical
	// blocks. Requires some peers in the network to take and serve state machine
	// snapshots. State sync is not attempted if the node has any local state
	// (LastBlockHeight > 0). The node will have a truncated block history, starting from
	// the height of the snapshot.
	Enable bool `mapstructure:"enable"`

	// State sync uses light client verification to verify state. This can be done either
	// through the P2P layer or the RPC layer. Set this to true to use the P2P layer. If
	// false (default), the RPC layer will be used.
	UseP2P bool `mapstructure:"use-p2p"`

	// If using RPC, at least two addresses need to be provided. They should be compatible
	// with net.Dial, for example: "host.example.com:2125".
	RPCServers []string `mapstructure:"rpc-servers"`

	// The hash and height of a trusted block. Must be within the trust-period.
	TrustHeight int64  `mapstructure:"trust-height"`
	TrustHash   string `mapstructure:"trust-hash"`

	// The trust period should be set so that Tendermint can detect and gossip
	// misbehavior before it is considered expired. For chains based on the Cosmos SDK,
	// one day less than the unbonding period should suffice.
	TrustPeriod time.Duration `mapstructure:"trust-period"`

	// Time to spend discovering snapshots before initiating a restore.
	DiscoveryTime time.Duration `mapstructure:"discovery-time"`

	// Temporary directory for state sync snapshot chunks, defaults to os.TempDir().
	// The synchronizer will create a new, randomly named directory within this directory
	// and remove it when the sync is complete.
	TempDir string `mapstructure:"temp-dir"`

	// The timeout duration before re-requesting a chunk, possibly from a different
	// peer (default: 15 seconds).
	ChunkRequestTimeout time.Duration `mapstructure:"chunk-request-timeout"`

	// The number of concurrent chunk and block fetchers to run (default: 4).
	Fetchers int32 `mapstructure:"fetchers"`
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
		TrustPeriod:         168 * time.Hour,
		DiscoveryTime:       15 * time.Second,
		ChunkRequestTimeout: 15 * time.Second,
		Fetchers:            4,
	}
}

// TestStateSyncConfig returns a default configuration for the state sync service
func TestStateSyncConfig() *StateSyncConfig {
	return DefaultStateSyncConfig()
}

// ValidateBasic performs basic validation.
func (cfg *StateSyncConfig) ValidateBasic() error {
	if !cfg.Enable {
		return nil
	}

	// If we're not using the P2P stack then we need to validate the
	// RPCServers
	if !cfg.UseP2P {
		if len(cfg.RPCServers) < 2 {
			return errors.New("at least two rpc-servers must be specified")
		}

		for _, server := range cfg.RPCServers {
			if server == "" {
				return errors.New("found empty rpc-servers entry")
			}
		}
	}

	if cfg.DiscoveryTime != 0 && cfg.DiscoveryTime < 5*time.Second {
		return errors.New("discovery time must be 0s or greater than five seconds")
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

	if cfg.ChunkRequestTimeout < 5*time.Second {
		return errors.New("chunk-request-timeout must be at least 5 seconds")
	}

	if cfg.Fetchers <= 0 {
		return errors.New("fetchers is required")
	}

	return nil
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
	// The backend database list to back the indexer.
	// If list contains `null`, meaning no indexer service will be used.
	//
	// Options:
	//   1) "null" - no indexer services.
	//   2) "kv" (default) - the simplest possible indexer,
	//      backed by key-value storage (defaults to levelDB; see DBBackend).
	//   3) "psql" - the indexer services backed by PostgreSQL.
	Indexer []string `mapstructure:"indexer"`

	// The PostgreSQL connection configuration, the connection format:
	// postgresql://<user>:<password>@<host>:<port>/<db>?<opts>
	PsqlConn string `mapstructure:"psql-conn"`
}

// DefaultTxIndexConfig returns a default configuration for the transaction indexer.
func DefaultTxIndexConfig() *TxIndexConfig {
	return &TxIndexConfig{
		Indexer: []string{"kv"},
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
