package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	tmos "github.com/tendermint/tendermint/libs/os"
	tmrand "github.com/tendermint/tendermint/libs/rand"
)

// defaultDirPerm is the default permissions used when creating directories.
const defaultDirPerm = 0700

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
	if err := tmos.EnsureDir(rootDir, defaultDirPerm); err != nil {
		panic(err.Error())
	}
	if err := tmos.EnsureDir(filepath.Join(rootDir, defaultConfigDir), defaultDirPerm); err != nil {
		panic(err.Error())
	}
	if err := tmos.EnsureDir(filepath.Join(rootDir, defaultDataDir), defaultDirPerm); err != nil {
		panic(err.Error())
	}
}

// WriteConfigFile renders config using the template and writes it to configFilePath.
// This function is called by cmd/tendermint/commands/init.go
func WriteConfigFile(rootDir string, config *Config) error {
	return config.WriteToTemplate(filepath.Join(rootDir, defaultConfigFilePath))
}

// WriteToTemplate writes the config to the exact file specified by
// the path, in the default toml template and does not mangle the path
// or filename at all.
func (cfg *Config) WriteToTemplate(path string) error {
	var buffer bytes.Buffer

	if err := configTemplate.Execute(&buffer, cfg); err != nil {
		return err
	}

	return writeFile(path, buffer.Bytes(), 0644)
}

func writeDefaultConfigFileIfNone(rootDir string) error {
	configFilePath := filepath.Join(rootDir, defaultConfigFilePath)
	if !tmos.FileExists(configFilePath) {
		return WriteConfigFile(rootDir, DefaultConfig())
	}
	return nil
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
proxy-app = "{{ .BaseConfig.ProxyApp }}"

# A custom human readable name for this node
moniker = "{{ .BaseConfig.Moniker }}"

# Mode of Node: full | validator | seed
# * validator node
#   - all reactors
#   - with priv_validator_key.json, priv_validator_state.json
# * full node
#   - all reactors
#   - No priv_validator_key.json, priv_validator_state.json
# * seed node
#   - only P2P, PEX Reactor
#   - No priv_validator_key.json, priv_validator_state.json
mode = "{{ .BaseConfig.Mode }}"

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
db-backend = "{{ .BaseConfig.DBBackend }}"

# Database directory
db-dir = "{{ js .BaseConfig.DBPath }}"

# Output level for logging, including package level options
log-level = "{{ .BaseConfig.LogLevel }}"

# Output format: 'plain' (colored text) or 'json'
log-format = "{{ .BaseConfig.LogFormat }}"

##### additional base config options #####

# Path to the JSON file containing the initial validator set and other meta data
genesis-file = "{{ js .BaseConfig.Genesis }}"

# Path to the JSON file containing the private key to use for node authentication in the p2p protocol
node-key-file = "{{ js .BaseConfig.NodeKey }}"

# Mechanism to connect to the ABCI application: socket | grpc
abci = "{{ .BaseConfig.ABCI }}"

# If true, query the ABCI app on connecting to a new peer
# so the app can decide if we should keep the connection or not
filter-peers = {{ .BaseConfig.FilterPeers }}


#######################################################
###       Priv Validator Configuration              ###
#######################################################
[priv-validator]

# Path to the JSON file containing the private key to use as a validator in the consensus protocol
key-file = "{{ js .PrivValidator.Key }}"

# Path to the JSON file containing the last sign state of a validator
state-file = "{{ js .PrivValidator.State }}"

# TCP or UNIX socket address for Tendermint to listen on for
# connections from an external PrivValidator process
# when the listenAddr is prefixed with grpc instead of tcp it will use the gRPC Client
laddr = "{{ .PrivValidator.ListenAddr }}"

# Path to the client certificate generated while creating needed files for secure connection.
# If a remote validator address is provided but no certificate, the connection will be insecure
client-certificate-file = "{{ js .PrivValidator.ClientCertificate }}"

# Client key generated while creating certificates for secure connection
client-key-file = "{{ js .PrivValidator.ClientKey }}"

# Path to the Root Certificate Authority used to sign both client and server certificates
root-ca-file = "{{ js .PrivValidator.RootCA }}"


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
cors-allowed-origins = [{{ range .RPC.CORSAllowedOrigins }}{{ printf "%q, " . }}{{end}}]

# A list of methods the client is allowed to use with cross-domain requests
cors-allowed-methods = [{{ range .RPC.CORSAllowedMethods }}{{ printf "%q, " . }}{{end}}]

# A list of non simple headers the client is allowed to use with cross-domain requests
cors-allowed-headers = [{{ range .RPC.CORSAllowedHeaders }}{{ printf "%q, " . }}{{end}}]

# Activate unsafe RPC commands like /dial-seeds and /unsafe-flush-mempool
unsafe = {{ .RPC.Unsafe }}

# Maximum number of simultaneous connections (including WebSocket).
# If you want to accept a larger number than the default, make sure
# you increase your OS limits.
# 0 - unlimited.
# Should be < {ulimit -Sn} - {MaxNumInboundPeers} - {MaxNumOutboundPeers} - {N of wal, db and other open files}
# 1024 - 40 - 10 - 50 = 924 = ~900
max-open-connections = {{ .RPC.MaxOpenConnections }}

# Maximum number of unique clientIDs that can /subscribe
# If you're using /broadcast_tx_commit, set to the estimated maximum number
# of broadcast_tx_commit calls per block.
max-subscription-clients = {{ .RPC.MaxSubscriptionClients }}

# Maximum number of unique queries a given client can /subscribe to
# If you're using a Local RPC client and /broadcast_tx_commit, set this
# to the estimated maximum number of broadcast_tx_commit calls per block.
max-subscriptions-per-client = {{ .RPC.MaxSubscriptionsPerClient }}

# If true, disable the websocket interface to the RPC service.  This has
# the effect of disabling the /subscribe, /unsubscribe, and /unsubscribe_all
# methods for event subscription.
#
# EXPERIMENTAL: This setting will be removed in Tendermint v0.37.
experimental-disable-websocket = {{ .RPC.ExperimentalDisableWebsocket }}

# The time window size for the event log. All events up to this long before
# the latest (up to EventLogMaxItems) will be available for subscribers to
# fetch via the /events method.  If 0 (the default) the event log and the
# /events RPC method are disabled.
event-log-window-size = "{{ .RPC.EventLogWindowSize }}"

# The maxiumum number of events that may be retained by the event log.  If
# this value is 0, no upper limit is set. Otherwise, items in excess of
# this number will be discarded from the event log.
#
# Warning: This setting is a safety valve. Setting it too low may cause
# subscribers to miss events.  Try to choose a value higher than the
# maximum worst-case expected event load within the chosen window size in
# ordinary operation.
#
# For example, if the window size is 10 minutes and the node typically
# averages 1000 events per ten minutes, but with occasional known spikes of
# up to 2000, choose a value > 2000.
event-log-max-items = {{ .RPC.EventLogMaxItems }}

# How long to wait for a tx to be committed during /broadcast_tx_commit.
# WARNING: Using a value larger than 10s will result in increasing the
# global HTTP write timeout, which applies to all connections and endpoints.
# See https://github.com/tendermint/tendermint/issues/3435
timeout-broadcast-tx-commit = "{{ .RPC.TimeoutBroadcastTxCommit }}"

# Maximum size of request body, in bytes
max-body-bytes = {{ .RPC.MaxBodyBytes }}

# Maximum size of request header, in bytes
max-header-bytes = {{ .RPC.MaxHeaderBytes }}

# The path to a file containing certificate that is used to create the HTTPS server.
# Might be either absolute path or path related to Tendermint's config directory.
# If the certificate is signed by a certificate authority,
# the certFile should be the concatenation of the server's certificate, any intermediates,
# and the CA's certificate.
# NOTE: both tls-cert-file and tls-key-file must be present for Tendermint to create HTTPS server.
# Otherwise, HTTP server is run.
tls-cert-file = "{{ .RPC.TLSCertFile }}"

# The path to a file containing matching private key that is used to create the HTTPS server.
# Might be either absolute path or path related to Tendermint's config directory.
# NOTE: both tls-cert-file and tls-key-file must be present for Tendermint to create HTTPS server.
# Otherwise, HTTP server is run.
tls-key-file = "{{ .RPC.TLSKeyFile }}"

# pprof listen address (https://golang.org/pkg/net/http/pprof)
pprof-laddr = "{{ .RPC.PprofListenAddress }}"

#######################################################
###           P2P Configuration Options             ###
#######################################################
[p2p]

# Select the p2p internal queue.
# Options are: "fifo" and "simple-priority", and "priority",
# with the default being "simple-priority".
queue-type = "{{ .P2P.QueueType }}"

# Address to listen for incoming connections
laddr = "{{ .P2P.ListenAddress }}"

# Address to advertise to peers for them to dial
# If empty, will use the same port as the laddr,
# and will introspect on the listener or use UPnP
# to figure out the address. ip and port are required
# example: 159.89.10.97:26656
external-address = "{{ .P2P.ExternalAddress }}"

# Comma separated list of peers to be added to the peer store
# on startup. Either BootstrapPeers or PersistentPeers are
# needed for peer discovery
bootstrap-peers = "{{ .P2P.BootstrapPeers }}"

# Comma separated list of nodes to keep persistent connections to
persistent-peers = "{{ .P2P.PersistentPeers }}"

# UPNP port forwarding
upnp = {{ .P2P.UPNP }}

# Maximum number of connections (inbound and outbound).
max-connections = {{ .P2P.MaxConnections }}

# Maximum number of connections reserved for outgoing
# connections. Must be less than max-connections
max-outgoing-connections = {{ .P2P.MaxOutgoingConnections }}

# Rate limits the number of incoming connection attempts per IP address.
max-incoming-connection-attempts = {{ .P2P.MaxIncomingConnectionAttempts }}

# Set true to enable the peer-exchange reactor
pex = {{ .P2P.PexReactor }}

# Comma separated list of peer IDs to keep private (will not be gossiped to other peers)
# Warning: IPs will be exposed at /net_info, for more information https://github.com/tendermint/tendermint/issues/3055
private-peer-ids = "{{ .P2P.PrivatePeerIDs }}"

# Peer connection configuration.
handshake-timeout = "{{ .P2P.HandshakeTimeout }}"
dial-timeout = "{{ .P2P.DialTimeout }}"

# Time to wait before flushing messages out on the connection
# TODO: Remove once MConnConnection is removed.
flush-throttle-timeout = "{{ .P2P.FlushThrottleTimeout }}"

# Maximum size of a message packet payload, in bytes
# TODO: Remove once MConnConnection is removed.
max-packet-msg-payload-size = {{ .P2P.MaxPacketMsgPayloadSize }}

# Rate at which packets can be sent, in bytes/second
# TODO: Remove once MConnConnection is removed.
send-rate = {{ .P2P.SendRate }}

# Rate at which packets can be received, in bytes/second
# TODO: Remove once MConnConnection is removed.
recv-rate = {{ .P2P.RecvRate }}


#######################################################
###          Mempool Configuration Option          ###
#######################################################
[mempool]

# recheck has been moved from a config option to a global
# consensus param in v0.36
# See https://github.com/tendermint/tendermint/issues/8244 for more information.

# Set true to broadcast transactions in the mempool to other nodes
broadcast = {{ .Mempool.Broadcast }}

# Maximum number of transactions in the mempool
size = {{ .Mempool.Size }}

# Limit the total size of all txs in the mempool.
# This only accounts for raw transactions (e.g. given 1MB transactions and
# max-txs-bytes=5MB, mempool will only accept 5 transactions).
max-txs-bytes = {{ .Mempool.MaxTxsBytes }}

# Size of the cache (used to filter transactions we saw earlier) in transactions
cache-size = {{ .Mempool.CacheSize }}

# Do not remove invalid transactions from the cache (default: false)
# Set to true if it's not possible for any invalid transaction to become valid
# again in the future.
keep-invalid-txs-in-cache = {{ .Mempool.KeepInvalidTxsInCache }}

# Maximum size of a single transaction.
# NOTE: the max size of a tx transmitted over the network is {max-tx-bytes}.
max-tx-bytes = {{ .Mempool.MaxTxBytes }}

# Maximum size of a batch of transactions to send to a peer
# Including space needed by encoding (one varint per transaction).
# XXX: Unused due to https://github.com/tendermint/tendermint/issues/5796
max-batch-bytes = {{ .Mempool.MaxBatchBytes }}

# ttl-duration, if non-zero, defines the maximum amount of time a transaction
# can exist for in the mempool.
#
# Note, if ttl-num-blocks is also defined, a transaction will be removed if it
# has existed in the mempool at least ttl-num-blocks number of blocks or if it's
# insertion time into the mempool is beyond ttl-duration.
ttl-duration = "{{ .Mempool.TTLDuration }}"

# ttl-num-blocks, if non-zero, defines the maximum number of blocks a transaction
# can exist for in the mempool.
#
# Note, if ttl-duration is also defined, a transaction will be removed if it
# has existed in the mempool at least ttl-num-blocks number of blocks or if
# it's insertion time into the mempool is beyond ttl-duration.
ttl-num-blocks = {{ .Mempool.TTLNumBlocks }}

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

# State sync uses light client verification to verify state. This can be done either through the
# P2P layer or RPC layer. Set this to true to use the P2P layer. If false (default), RPC layer
# will be used.
use-p2p = {{ .StateSync.UseP2P }}

# If using RPC, at least two addresses need to be provided. They should be compatible with net.Dial,
# for example: "host.example.com:2125"
rpc-servers = "{{ StringsJoin .StateSync.RPCServers "," }}"

# The hash and height of a trusted block. Must be within the trust-period.
trust-height = {{ .StateSync.TrustHeight }}
trust-hash = "{{ .StateSync.TrustHash }}"

# The trust period should be set so that Tendermint can detect and gossip misbehavior before
# it is considered expired. For chains based on the Cosmos SDK, one day less than the unbonding
# period should suffice.
trust-period = "{{ .StateSync.TrustPeriod }}"

# Time to spend discovering snapshots before initiating a restore.
discovery-time = "{{ .StateSync.DiscoveryTime }}"

# Temporary directory for state sync snapshot chunks, defaults to os.TempDir().
# The synchronizer will create a new, randomly named directory within this directory
# and remove it when the sync is complete.
temp-dir = "{{ .StateSync.TempDir }}"

# The timeout duration before re-requesting a chunk, possibly from a different
# peer (default: 15 seconds).
chunk-request-timeout = "{{ .StateSync.ChunkRequestTimeout }}"

# The number of concurrent chunk and block fetchers to run (default: 4).
fetchers = "{{ .StateSync.Fetchers }}"

#######################################################
###         Consensus Configuration Options         ###
#######################################################
[consensus]

wal-file = "{{ js .Consensus.WalPath }}"

# How many blocks to look back to check existence of the node's consensus votes before joining consensus
# When non-zero, the node will panic upon restart
# if the same consensus key was used to sign {double-sign-check-height} last blocks.
# So, validators should stop the state machine, wait for some blocks, and then restart the state machine to avoid panic.
double-sign-check-height = {{ .Consensus.DoubleSignCheckHeight }}

# EmptyBlocks mode and possible interval between empty blocks
create-empty-blocks = {{ .Consensus.CreateEmptyBlocks }}
create-empty-blocks-interval = "{{ .Consensus.CreateEmptyBlocksInterval }}"

# Reactor sleep duration parameters
peer-gossip-sleep-duration = "{{ .Consensus.PeerGossipSleepDuration }}"
peer-query-maj23-sleep-duration = "{{ .Consensus.PeerQueryMaj23SleepDuration }}"

### Unsafe Timeout Overrides ###

# These fields provide temporary overrides for the Timeout consensus parameters.
# Use of these parameters is strongly discouraged. Using these parameters may have serious
# liveness implications for the validator and for the chain.
#
# These fields will be removed from the configuration file in the v0.37 release of Tendermint.
# For additional information, see ADR-74:
# https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-074-timeout-params.md

# This field provides an unsafe override of the Propose timeout consensus parameter.
# This field configures how long the consensus engine will wait for a proposal block before prevoting nil.
# If this field is set to a value greater than 0, it will take effect.
# unsafe-propose-timeout-override = {{ .Consensus.UnsafeProposeTimeoutOverride }}

# This field provides an unsafe override of the ProposeDelta timeout consensus parameter.
# This field configures how much the propose timeout increases with each round.
# If this field is set to a value greater than 0, it will take effect.
# unsafe-propose-timeout-delta-override = {{ .Consensus.UnsafeProposeTimeoutDeltaOverride }}

# This field provides an unsafe override of the Vote timeout consensus parameter.
# This field configures how long the consensus engine will wait after
# receiving +2/3 votes in a round.
# If this field is set to a value greater than 0, it will take effect.
# unsafe-vote-timeout-override = {{ .Consensus.UnsafeVoteTimeoutOverride }}

# This field provides an unsafe override of the VoteDelta timeout consensus parameter.
# This field configures how much the vote timeout increases with each round.
# If this field is set to a value greater than 0, it will take effect.
# unsafe-vote-timeout-delta-override = {{ .Consensus.UnsafeVoteTimeoutDeltaOverride }}

# This field provides an unsafe override of the Commit timeout consensus parameter.
# This field configures how long the consensus engine will wait after receiving
# +2/3 precommits before beginning the next height.
# If this field is set to a value greater than 0, it will take effect.
# unsafe-commit-timeout-override = {{ .Consensus.UnsafeCommitTimeoutOverride }}

# This field provides an unsafe override of the BypassCommitTimeout consensus parameter.
# This field configures if the consensus engine will wait for the full Commit timeout
# before proceeding to the next height.
# If this field is set to true, the consensus engine will proceed to the next height
# as soon as the node has gathered votes from all of the validators on the network.
# unsafe-bypass-commit-timeout-override =

#######################################################
###   Transaction Indexer Configuration Options     ###
#######################################################
[tx-index]

# The backend database list to back the indexer.
# If list contains "null" or "", meaning no indexer service will be used.
#
# The application will set which txs to index. In some cases a node operator will be able
# to decide which txs to index based on configuration set in the application.
#
# Options:
#   1) "null" (default) - no indexer services.
#   2) "kv" - a simple indexer backed by key-value storage (see DBBackend)
#   3) "psql" - the indexer services backed by PostgreSQL.
# When "kv" or "psql" is chosen "tx.height" and "tx.hash" will always be indexed.
indexer = [{{ range $i, $e := .TxIndex.Indexer }}{{if $i}}, {{end}}{{ printf "%q" $e}}{{end}}]

# The PostgreSQL connection configuration, the connection format:
#   postgresql://<user>:<password>@<host>:<port>/<db>?<opts>
psql-conn = "{{ .TxIndex.PsqlConn }}"

#######################################################
###       Instrumentation Configuration Options     ###
#######################################################
[instrumentation]

# When true, Prometheus metrics are served under /metrics on
# PrometheusListenAddr.
# Check out the documentation for the list of available metrics.
prometheus = {{ .Instrumentation.Prometheus }}

# Address to listen for Prometheus collector(s) connections
prometheus-listen-addr = "{{ .Instrumentation.PrometheusListenAddr }}"

# Maximum number of simultaneous connections.
# If you want to accept a larger number than the default, make sure
# you increase your OS limits.
# 0 - unlimited.
max-open-connections = {{ .Instrumentation.MaxOpenConnections }}

# Instrumentation namespace
namespace = "{{ .Instrumentation.Namespace }}"
`

/****** these are for test settings ***********/

func ResetTestRoot(dir, testName string) (*Config, error) {
	return ResetTestRootWithChainID(dir, testName, "")
}

func ResetTestRootWithChainID(dir, testName string, chainID string) (*Config, error) {
	// create a unique, concurrency-safe test directory under os.TempDir()
	rootDir, err := os.MkdirTemp(dir, fmt.Sprintf("%s-%s_", chainID, testName))
	if err != nil {
		return nil, err
	}
	// ensure config and data subdirs are created
	if err := tmos.EnsureDir(filepath.Join(rootDir, defaultConfigDir), defaultDirPerm); err != nil {
		return nil, err
	}
	if err := tmos.EnsureDir(filepath.Join(rootDir, defaultDataDir), defaultDirPerm); err != nil {
		return nil, err
	}

	conf := DefaultConfig()
	genesisFilePath := filepath.Join(rootDir, conf.Genesis)
	privKeyFilePath := filepath.Join(rootDir, conf.PrivValidator.Key)
	privStateFilePath := filepath.Join(rootDir, conf.PrivValidator.State)

	// Write default config file if missing.
	if err := writeDefaultConfigFileIfNone(rootDir); err != nil {
		return nil, err
	}

	if !tmos.FileExists(genesisFilePath) {
		if chainID == "" {
			chainID = "tendermint_test"
		}
		testGenesis := fmt.Sprintf(testGenesisFmt, chainID)
		if err := writeFile(genesisFilePath, []byte(testGenesis), 0644); err != nil {
			return nil, err
		}
	}
	// we always overwrite the priv val
	if err := writeFile(privKeyFilePath, []byte(testPrivValidatorKey), 0644); err != nil {
		return nil, err
	}
	if err := writeFile(privStateFilePath, []byte(testPrivValidatorState), 0644); err != nil {
		return nil, err
	}

	config := TestConfig().SetRoot(rootDir)
	config.Instrumentation.Namespace = fmt.Sprintf("%s_%s_%s", testName, chainID, tmrand.Str(16))
	return config, nil
}

func writeFile(filePath string, contents []byte, mode os.FileMode) error {
	if err := os.WriteFile(filePath, contents, mode); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return nil
}

const testGenesisFmt = `{
  "genesis_time": "2018-10-10T08:20:13.695936996Z",
  "chain_id": "%s",
  "initial_height": "1",
	"consensus_params": {
		"block": {
			"max_bytes": "22020096",
			"max_gas": "-1",
			"time_iota_ms": "10"
		},
		"synchrony": {
			"message_delay": "500000000",
			"precision": "10000000"
		},
		"timeout": {
			"propose": "30000000",
			"propose_delta": "50000",
			"vote": "30000000",
			"vote_delta": "50000",
			"commit": "10000000",
			"bypass_timeout_commit": true
		},
		"evidence": {
			"max_age_num_blocks": "100000",
			"max_age_duration": "172800000000000",
			"max_bytes": "1048576"
		},
		"validator": {
			"pub_key_types": [
				"ed25519"
			]
		},
		"version": {}
	},
  "validators": [
    {
      "pub_key": {
        "type": "tendermint/PubKeyEd25519",
        "value":"AT/+aaL1eB0477Mud9JMm8Sh8BIvOYlPGC9KkIUmFaE="
      },
      "power": "10",
      "name": ""
    }
  ],
  "app_hash": ""
}`

const testPrivValidatorKey = `{
  "address": "A3258DCBF45DCA0DF052981870F2D1441A36D145",
  "pub_key": {
    "type": "tendermint/PubKeyEd25519",
    "value": "AT/+aaL1eB0477Mud9JMm8Sh8BIvOYlPGC9KkIUmFaE="
  },
  "priv_key": {
    "type": "tendermint/PrivKeyEd25519",
    "value": "EVkqJO/jIXp3rkASXfh9YnyToYXRXhBr6g9cQVxPFnQBP/5povV4HTjvsy530kybxKHwEi85iU8YL0qQhSYVoQ=="
  }
}`

const testPrivValidatorState = `{
  "height": "0",
  "round": 0,
  "step": 0
}`
