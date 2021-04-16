package config

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"text/template"

	tmos "github.com/tendermint/tendermint/libs/os"
)

// DefaultDirPerm is the default permissions used when creating directories.
const DefaultDirPerm = 0700

var configTemplate *template.Template

func init() {
	var err error
	tmpl := template.New("configFileTemplate").Funcs(template.FuncMap{
		"StringsJoin": strings.Join,
	})
	if configTemplate, err = tmpl.Parse(defaultConfigTemplate); err != nil {
		panic(err)
	}
}

/****** these are for production settings ***********/

// EnsureRoot creates the root, config, and data directories if they don't exist,
// and panics if it fails.
func EnsureRoot(rootDir string) {
	if err := tmos.EnsureDir(rootDir, DefaultDirPerm); err != nil {
		panic(err.Error())
	}
	if err := tmos.EnsureDir(filepath.Join(rootDir, defaultConfigDir), DefaultDirPerm); err != nil {
		panic(err.Error())
	}
	if err := tmos.EnsureDir(filepath.Join(rootDir, defaultDataDir), DefaultDirPerm); err != nil {
		panic(err.Error())
	}

	configFilePath := filepath.Join(rootDir, defaultConfigFilePath)

	// Write default config file if missing.
	if !tmos.FileExists(configFilePath) {
		writeDefaultConfigFile(configFilePath)
	}
}

// XXX: this func should probably be called by cmd/tendermint/commands/init.go
// alongside the writing of the genesis.json and priv_validator.json
func writeDefaultConfigFile(configFilePath string) {
	WriteConfigFile(configFilePath, DefaultConfig())
}

// WriteConfigFile renders config using the template and writes it to configFilePath.
func WriteConfigFile(configFilePath string, config *Config) {
	var buffer bytes.Buffer

	if err := configTemplate.Execute(&buffer, config); err != nil {
		panic(err)
	}

	tmos.MustWriteFile(configFilePath, buffer.Bytes(), 0644)
}

// Note: any changes to the comments/variables/mapstructure
// must be reflected in the appropriate struct in config/config.go
const defaultConfigTemplate = `# This is a TOML config file.
# For more information, see https://github.com/toml-lang/toml

# NOTE: Any path below can be absolute (e.g. "/var/myawesomeapp/data") or
# relative to the home directory (e.g. "data"). The home directory is
# "$HOME/.tendermint" by default, but could be changed via $TMHOME env variable
# or --home cmd flag.

#######################################################################
###                   Main Base Config Options                      ###
#######################################################################

# TCP or UNIX socket address of the ABCI application,
# or the name of an ABCI application compiled in with the Tendermint binary
proxy_app = "{{ .BaseConfig.ProxyApp }}"

# A custom human readable name for this node
moniker = "{{ .BaseConfig.Moniker }}"

# If this node is many blocks behind the tip of the chain, FastSync
# allows them to catchup quickly by downloading blocks in parallel
# and verifying their commits
fast_sync = {{ .BaseConfig.FastSyncMode }}

# Database backend: goleveldb | cleveldb | boltdb | rocksdb | badgerdb
# * goleveldb (github.com/syndtr/goleveldb - most popular implementation)
#   - pure go
#   - stable
# * cleveldb (uses levigo wrapper)
#   - fast
#   - requires gcc
#   - use cleveldb build tag (go build -tags cleveldb)
# * boltdb (uses etcd's fork of bolt - github.com/etcd-io/bbolt)
#   - EXPERIMENTAL
#   - may be faster is some use-cases (random reads - indexer)
#   - use boltdb build tag (go build -tags boltdb)
# * rocksdb (uses github.com/tecbot/gorocksdb)
#   - EXPERIMENTAL
#   - requires gcc
#   - use rocksdb build tag (go build -tags rocksdb)
# * badgerdb (uses github.com/dgraph-io/badger)
#   - EXPERIMENTAL
#   - use badgerdb build tag (go build -tags badgerdb)
db_backend = "{{ .BaseConfig.DBBackend }}"

# Database directory
db_dir = "{{ js .BaseConfig.DBPath }}"

# Output level for logging, including package level options
log_level = "{{ .BaseConfig.LogLevel }}"

# Output format: 'plain' (colored text) or 'json'
log_format = "{{ .BaseConfig.LogFormat }}"

##### additional base config options #####

# Path to the JSON file containing the initial validator set and other meta data
genesis_file = "{{ js .BaseConfig.Genesis }}"

# Path to the JSON file containing the private key to use as a validator in the consensus protocol
priv_validator_key_file = "{{ js .BaseConfig.PrivValidatorKey }}"

# Path to the JSON file containing the last sign state of a validator
priv_validator_state_file = "{{ js .BaseConfig.PrivValidatorState }}"

# TCP or UNIX socket address for Tendermint to listen on for
# connections from an external PrivValidator process
priv_validator_laddr = "{{ .BaseConfig.PrivValidatorListenAddr }}"

# Local Dash Core Host to connect to
# If this is set, the node follows a Dash Core PrivValidator process
priv_validator_core_rpc_host = "{{ .BaseConfig.PrivValidatorCoreRPCHost }}"

# Local Dash Core RPC Username
priv_validator_core_rpc_username = "{{ .BaseConfig.PrivValidatorCoreRPCUsername }}"

# Local Dash Core RPC Password
priv_validator_core_rpc_password = "{{ .BaseConfig.PrivValidatorCoreRPCPassword }}"

# LLMQ Type used
llmq_type_used = "{{ .BaseConfig.LLMQTypeUsed }}"

# Path to the JSON file containing the private key to use for node authentication in the p2p protocol
node_key_file = "{{ js .BaseConfig.NodeKey }}"

# Mechanism to connect to the ABCI application: socket | grpc
abci = "{{ .BaseConfig.ABCI }}"

# If true, query the ABCI app on connecting to a new peer
# so the app can decide if we should keep the connection or not
filter_peers = {{ .BaseConfig.FilterPeers }}


#######################################################################
###                 Advanced Configuration Options                  ###
#######################################################################

#######################################################
###       RPC Server Configuration Options          ###
#######################################################
[rpc]

# TCP or UNIX socket address for the RPC server to listen on
laddr = "{{ .RPC.ListenAddress }}"

# A list of origins a cross-domain request can be executed from
# Default value '[]' disables cors support
# Use '["*"]' to allow any origin
cors_allowed_origins = [{{ range .RPC.CORSAllowedOrigins }}{{ printf "%q, " . }}{{end}}]

# A list of methods the client is allowed to use with cross-domain requests
cors_allowed_methods = [{{ range .RPC.CORSAllowedMethods }}{{ printf "%q, " . }}{{end}}]

# A list of non simple headers the client is allowed to use with cross-domain requests
cors_allowed_headers = [{{ range .RPC.CORSAllowedHeaders }}{{ printf "%q, " . }}{{end}}]

# TCP or UNIX socket address for the gRPC server to listen on
# NOTE: This server only supports /broadcast_tx_commit
grpc_laddr = "{{ .RPC.GRPCListenAddress }}"

# Maximum number of simultaneous connections.
# Does not include RPC (HTTP&WebSocket) connections. See max_open_connections
# If you want to accept a larger number than the default, make sure
# you increase your OS limits.
# 0 - unlimited.
# Should be < {ulimit -Sn} - {MaxNumInboundPeers} - {MaxNumOutboundPeers} - {N of wal, db and other open files}
# 1024 - 40 - 10 - 50 = 924 = ~900
grpc_max_open_connections = {{ .RPC.GRPCMaxOpenConnections }}

# Activate unsafe RPC commands like /dial_seeds and /unsafe_flush_mempool
unsafe = {{ .RPC.Unsafe }}

# Maximum number of simultaneous connections (including WebSocket).
# Does not include gRPC connections. See grpc_max_open_connections
# If you want to accept a larger number than the default, make sure
# you increase your OS limits.
# 0 - unlimited.
# Should be < {ulimit -Sn} - {MaxNumInboundPeers} - {MaxNumOutboundPeers} - {N of wal, db and other open files}
# 1024 - 40 - 10 - 50 = 924 = ~900
max_open_connections = {{ .RPC.MaxOpenConnections }}

# Maximum number of unique clientIDs that can /subscribe
# If you're using /broadcast_tx_commit, set to the estimated maximum number
# of broadcast_tx_commit calls per block.
max_subscription_clients = {{ .RPC.MaxSubscriptionClients }}

# Maximum number of unique queries a given client can /subscribe to
# If you're using GRPC (or Local RPC client) and /broadcast_tx_commit, set to
# the estimated # maximum number of broadcast_tx_commit calls per block.
max_subscriptions_per_client = {{ .RPC.MaxSubscriptionsPerClient }}

# How long to wait for a tx to be committed during /broadcast_tx_commit.
# WARNING: Using a value larger than 10s will result in increasing the
# global HTTP write timeout, which applies to all connections and endpoints.
# See https://github.com/tendermint/tendermint/issues/3435
timeout_broadcast_tx_commit = "{{ .RPC.TimeoutBroadcastTxCommit }}"

# Maximum size of request body, in bytes
max_body_bytes = {{ .RPC.MaxBodyBytes }}

# Maximum size of request header, in bytes
max_header_bytes = {{ .RPC.MaxHeaderBytes }}

# The path to a file containing certificate that is used to create the HTTPS server.
# Might be either absolute path or path related to Tendermint's config directory.
# If the certificate is signed by a certificate authority,
# the certFile should be the concatenation of the server's certificate, any intermediates,
# and the CA's certificate.
# NOTE: both tls_cert_file and tls_key_file must be present for Tendermint to create HTTPS server.
# Otherwise, HTTP server is run.
tls_cert_file = "{{ .RPC.TLSCertFile }}"

# The path to a file containing matching private key that is used to create the HTTPS server.
# Might be either absolute path or path related to Tendermint's config directory.
# NOTE: both tls-cert-file and tls-key-file must be present for Tendermint to create HTTPS server.
# Otherwise, HTTP server is run.
tls_key_file = "{{ .RPC.TLSKeyFile }}"

# pprof listen address (https://golang.org/pkg/net/http/pprof)
pprof_laddr = "{{ .RPC.PprofListenAddress }}"

#######################################################
###           P2P Configuration Options             ###
#######################################################
[p2p]

# Address to listen for incoming connections
laddr = "{{ .P2P.ListenAddress }}"

# Address to advertise to peers for them to dial
# If empty, will use the same port as the laddr,
# and will introspect on the listener or use UPnP
# to figure out the address.
external_address = "{{ .P2P.ExternalAddress }}"

# Comma separated list of seed nodes to connect to
seeds = "{{ .P2P.Seeds }}"

# Comma separated list of nodes to keep persistent connections to
persistent_peers = "{{ .P2P.PersistentPeers }}"

# UPNP port forwarding
upnp = {{ .P2P.UPNP }}

# Path to address book
addr_book_file = "{{ js .P2P.AddrBook }}"

# Set true for strict address routability rules
# Set false for private or local networks
addr_book_strict = {{ .P2P.AddrBookStrict }}

# Maximum number of inbound peers
max_num_inbound_peers = {{ .P2P.MaxNumInboundPeers }}

# Maximum number of outbound peers to connect to, excluding persistent peers
max_num_outbound_peers = {{ .P2P.MaxNumOutboundPeers }}

# List of node IDs, to which a connection will be (re)established ignoring any existing limits
unconditional_peer_ids = "{{ .P2P.UnconditionalPeerIDs }}"

# Maximum pause when redialing a persistent peer (if zero, exponential backoff is used)
persistent_peers_max_dial_period = "{{ .P2P.PersistentPeersMaxDialPeriod }}"

# Time to wait before flushing messages out on the connection
flush_throttle_timeout = "{{ .P2P.FlushThrottleTimeout }}"

# Maximum size of a message packet payload, in bytes
max_packet_msg_payload_size = {{ .P2P.MaxPacketMsgPayloadSize }}

# Rate at which packets can be sent, in bytes/second
send_rate = {{ .P2P.SendRate }}

# Rate at which packets can be received, in bytes/second
recv_rate = {{ .P2P.RecvRate }}

# Set true to enable the peer-exchange reactor
pex = {{ .P2P.PexReactor }}

# Seed mode, in which node constantly crawls the network and looks for
# peers. If another node asks it for addresses, it responds and disconnects.
#
# Does not work if the peer-exchange reactor is disabled.
seed_mode = {{ .P2P.SeedMode }}

# Comma separated list of peer IDs to keep private (will not be gossiped to other peers)
private_peer_ids = "{{ .P2P.PrivatePeerIDs }}"

# Toggle to disable guard against peers connecting from the same ip.
allow_duplicate_ip = {{ .P2P.AllowDuplicateIP }}

# Peer connection configuration.
handshake_timeout = "{{ .P2P.HandshakeTimeout }}"
dial_timeout = "{{ .P2P.DialTimeout }}"

#######################################################
###          Mempool Configuration Option          ###
#######################################################
[mempool]

recheck = {{ .Mempool.Recheck }}
broadcast = {{ .Mempool.Broadcast }}
wal_dir = "{{ js .Mempool.WalPath }}"

# Maximum number of transactions in the mempool
size = {{ .Mempool.Size }}

# Limit the total size of all txs in the mempool.
# This only accounts for raw transactions (e.g. given 1MB transactions and
# max_txs_bytes=5MB, mempool will only accept 5 transactions).
max_txs_bytes = {{ .Mempool.MaxTxsBytes }}

# Size of the cache (used to filter transactions we saw earlier) in transactions
cache_size = {{ .Mempool.CacheSize }}

# Do not remove invalid transactions from the cache (default: false)
# Set to true if it's not possible for any invalid transaction to become valid
# again in the future.
keep-invalid-txs-in-cache = {{ .Mempool.KeepInvalidTxsInCache }}

# Maximum size of a single transaction.
# NOTE: the max size of a tx transmitted over the network is {max_tx_bytes}.
max_tx_bytes = {{ .Mempool.MaxTxBytes }}

# Maximum size of a batch of transactions to send to a peer
# Including space needed by encoding (one varint per transaction).
# XXX: Unused due to https://github.com/tendermint/tendermint/issues/5796
max_batch_bytes = {{ .Mempool.MaxBatchBytes }}

#######################################################
###         State Sync Configuration Options        ###
#######################################################
[statesync]
# State sync rapidly bootstraps a new node by discovering, fetching, and restoring a state machine
# snapshot from peers instead of fetching and replaying historical blocks. Requires some peers in
# the network to take and serve state machine snapshots. State sync is not attempted if the node
# has any local state (LastBlockHeight > 0). The node will have a truncated block history,
# starting from the height of the snapshot.
enable = {{ .StateSync.Enable }}

# RPC servers (comma-separated) for light client verification of the synced state machine and
# retrieval of state data for node bootstrapping. Also needs a trusted height and corresponding
# header hash obtained from a trusted source, and a period during which validators can be trusted.
#
# For Cosmos SDK-based chains, trust_period should usually be about 2/3 of the unbonding time (~2
# weeks) during which they can be financially punished (slashed) for misbehavior.
rpc_servers = "{{ StringsJoin .StateSync.RPCServers "," }}"
trust_height = {{ .StateSync.TrustHeight }}
trust_hash = "{{ .StateSync.TrustHash }}"
trust_period = "{{ .StateSync.TrustPeriod }}"

# Time to spend discovering snapshots before initiating a restore.
discovery_time = "{{ .StateSync.DiscoveryTime }}"

# Temporary directory for state sync snapshot chunks, defaults to the OS tempdir (typically /tmp).
# Will create a new, randomly named directory within, and remove it when done.
temp_dir = "{{ .StateSync.TempDir }}"

#######################################################
###       Fast Sync Configuration Connections       ###
#######################################################
[fastsync]

# Fast Sync version to use:
#   1) "v0" (default) - the legacy fast sync implementation
#   2) "v1" - refactor of v0 version for better testability
#   2) "v2" - complete redesign of v0, optimized for testability & readability
version = "{{ .FastSync.Version }}"

#######################################################
###         Consensus Configuration Options         ###
#######################################################
[consensus]

wal_file = "{{ js .Consensus.WalPath }}"

# How long we wait for a proposal block before prevoting nil
timeout_propose = "{{ .Consensus.TimeoutPropose }}"
# How much timeout_propose increases with each round
timeout_propose_delta = "{{ .Consensus.TimeoutProposeDelta }}"
# How long we wait after receiving +2/3 prevotes for “anything” (ie. not a single block or nil)
timeout_prevote = "{{ .Consensus.TimeoutPrevote }}"
# How much the timeout_prevote increases with each round
timeout_prevote_delta = "{{ .Consensus.TimeoutPrevoteDelta }}"
# How long we wait after receiving +2/3 precommits for “anything” (ie. not a single block or nil)
timeout_precommit = "{{ .Consensus.TimeoutPrecommit }}"
# How much the timeout_precommit increases with each round
timeout_precommit_delta = "{{ .Consensus.TimeoutPrecommitDelta }}"
# How long we wait after committing a block, before starting on the new
# height (this gives us a chance to receive some more precommits, even
# though we already have +2/3).
timeout_commit = "{{ .Consensus.TimeoutCommit }}"

# How many blocks to look back to check existence of the node's consensus votes before joining consensus
# When non-zero, the node will panic upon restart
# if the same consensus key was used to sign {double_sign_check_height} last blocks.
# So, validators should stop the state machine, wait for some blocks, and then restart the state machine to avoid panic.
double_sign_check_height = {{ .Consensus.DoubleSignCheckHeight }}

# Make progress as soon as we have all the precommits (as if TimeoutCommit = 0)
skip_timeout_commit = {{ .Consensus.SkipTimeoutCommit }}

# EmptyBlocks mode and possible interval between empty blocks
create_empty_blocks = {{ .Consensus.CreateEmptyBlocks }}
create_empty_blocks_interval = "{{ .Consensus.CreateEmptyBlocksInterval }}"

# Reactor sleep duration parameters
peer_gossip_sleep_duration = "{{ .Consensus.PeerGossipSleepDuration }}"
peer_query_maj23_sleep_duration = "{{ .Consensus.PeerQueryMaj23SleepDuration }}"

#######################################################
###   Transaction Indexer Configuration Options     ###
#######################################################
[tx_index]

# What indexer to use for transactions
#
# The application will set which txs to index. In some cases a node operator will be able
# to decide which txs to index based on configuration set in the application.
#
# Options:
#   1) "null"
#   2) "kv" (default) - the simplest possible indexer, backed by key-value storage (defaults to levelDB; see DBBackend).
# 		- When "kv" is chosen "tx.height" and "tx.hash" will always be indexed.
indexer = "{{ .TxIndex.Indexer }}"

#######################################################
###       Instrumentation Configuration Options     ###
#######################################################
[instrumentation]

# When true, Prometheus metrics are served under /metrics on
# PrometheusListenAddr.
# Check out the documentation for the list of available metrics.
prometheus = {{ .Instrumentation.Prometheus }}

# Address to listen for Prometheus collector(s) connections
prometheus_listen_addr = "{{ .Instrumentation.PrometheusListenAddr }}"

# Maximum number of simultaneous connections.
# If you want to accept a larger number than the default, make sure
# you increase your OS limits.
# 0 - unlimited.
max_open_connections = {{ .Instrumentation.MaxOpenConnections }}

# Instrumentation namespace
namespace = "{{ .Instrumentation.Namespace }}"
`

/****** these are for test settings ***********/

func ResetTestRoot(testName string) *Config {
	return ResetTestRootWithChainID(testName, "")
}

func ResetTestRootWithChainID(testName string, chainID string) *Config {
	// create a unique, concurrency-safe test directory under os.TempDir()
	rootDir, err := ioutil.TempDir("", fmt.Sprintf("%s-%s_", chainID, testName))
	if err != nil {
		panic(err)
	}
	// ensure config and data subdirs are created
	if err := tmos.EnsureDir(filepath.Join(rootDir, defaultConfigDir), DefaultDirPerm); err != nil {
		panic(err)
	}
	if err := tmos.EnsureDir(filepath.Join(rootDir, defaultDataDir), DefaultDirPerm); err != nil {
		panic(err)
	}

	baseConfig := DefaultBaseConfig()
	configFilePath := filepath.Join(rootDir, defaultConfigFilePath)
	genesisFilePath := filepath.Join(rootDir, baseConfig.Genesis)
	privKeyFilePath := filepath.Join(rootDir, baseConfig.PrivValidatorKey)
	privStateFilePath := filepath.Join(rootDir, baseConfig.PrivValidatorState)

	// Write default config file if missing.
	if !tmos.FileExists(configFilePath) {
		writeDefaultConfigFile(configFilePath)
	}
	if !tmos.FileExists(genesisFilePath) {
		if chainID == "" {
			chainID = "tendermint_test"
		}
		testGenesis := fmt.Sprintf(testGenesisFmt, chainID)
		tmos.MustWriteFile(genesisFilePath, []byte(testGenesis), 0644)
	}
	// we always overwrite the priv val
	tmos.MustWriteFile(privKeyFilePath, []byte(testPrivValidatorKey), 0644)
	tmos.MustWriteFile(privStateFilePath, []byte(testPrivValidatorState), 0644)

	config := TestConfig().SetRoot(rootDir)
	return config
}

var testGenesisFmt = `{
  "genesis_time": "2018-10-10T08:20:13.695936996Z",
  "chain_id": "%s",
  "initial_height": "1",
  "initial_core_chain_locked_height": 1,
	"consensus_params": {
		"block": {
			"max_bytes": "22020096",
			"max_gas": "-1",
			"time_iota_ms": "10"
		},
		"evidence": {
			"max_age_num_blocks": "100000",
			"max_age_duration": "172800000000000",
			"max_bytes": "1048576"
		},
		"validator": {
			"pub_key_types": [
				"bls12381"
			]
		},
		"version": {}
	},
  "validators": [
    {
      "pub_key": {
        "type": "tendermint/PubKeyBLS12381",
        "value":"F5BjXeh0DppqaxX7a3LzoWr6CXPZcZeba6VHYdbiUCxQ23b00mFD8FRZpCz9Ug1E"
      },
      "power": "100",
      "name": "",
      "pro_tx_hash": "51BF39CC1F41B9FC63DFA5B1EDF3F0CA3AD5CAFAE4B12B4FE9263B08BB50C45F"
    }
  ],
  "quorum_hash": "444F39CC1F41B9FC63DFA5B1EDF3F0CA3AD5CAFAE4B12B4FE9263B08BB50C433",
  "threshold_public_key": {
    "type": "tendermint/PubKeyBLS12381",
	"value": "F5BjXeh0DppqaxX7a3LzoWr6CXPZcZeba6VHYdbiUCxQ23b00mFD8FRZpCz9Ug1E"
  },
  "app_hash": ""
}`

var testPrivValidatorKey = `{
  "address": "DDAD59BB10A10088C5A9CA219C3CF5BB4599B54E",
  "pub_key": {
    "type": "tendermint/PubKeyBLS12381",
    "value": "F5BjXeh0DppqaxX7a3LzoWr6CXPZcZeba6VHYdbiUCxQ23b00mFD8FRZpCz9Ug1E"
  },
  "priv_key": {
    "type": "tendermint/PrivKeyBLS12381",
    "value": "RokcLOxJWTyBkh5HPbdIACng/B65M8a5PYH1Nw6xn70="
  },
  "pro_tx_hash": "51BF39CC1F41B9FC63DFA5B1EDF3F0CA3AD5CAFAE4B12B4FE9263B08BB50C45F"
}`

var testPrivValidatorState = `{
  "height": "0",
  "round": 0,
  "step": 0
}`
