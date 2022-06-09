---
order: 3
---

# Configuration

Tendermint Core can be configured via a TOML file in
`$TMHOME/config/config.toml`. Some of these parameters can be overridden by
command-line flags. For most users, the options in the `##### main base configuration options #####` are intended to be modified while config options
further below are intended for advance power users.

## Options

The default configuration file create by `tendermint init` has all
the parameters set with their default values. It will look something
like the file below, however, double check by inspecting the
`config.toml` created with your version of `tendermint` installed:

```toml
# This is a TOML config file.
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
proxy-app = "tcp://127.0.0.1:26658"

# A custom human readable name for this node
moniker = "sidewinder"

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
mode = "validator"

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
db-backend = "goleveldb"

# Database directory
db-dir = "data"

# Output level for logging, including package level options
log-level = "info"

# Output format: 'plain' (colored text) or 'json'
log-format = "plain"

##### additional base config options #####

# Path to the JSON file containing the initial validator set and other meta data
genesis-file = "config/genesis.json"

# Path to the JSON file containing the private key to use for node authentication in the p2p protocol
node-key-file = "config/node_key.json"

# Mechanism to connect to the ABCI application: socket | grpc
abci = "socket"

# If true, query the ABCI app on connecting to a new peer
# so the app can decide if we should keep the connection or not
filter-peers = false


#######################################################
###       Priv Validator Configuration              ###
#######################################################
[priv-validator]

# Path to the JSON file containing the private key to use as a validator in the consensus protocol
key-file = "config/priv_validator_key.json"

# Path to the JSON file containing the last sign state of a validator
state-file = "data/priv_validator_state.json"

# TCP or UNIX socket address for Tendermint to listen on for
# connections from an external PrivValidator process
# when the listenAddr is prefixed with grpc instead of tcp it will use the gRPC Client
laddr = ""

# Path to the client certificate generated while creating needed files for secure connection.
# If a remote validator address is provided but no certificate, the connection will be insecure
client-certificate-file = ""

# Client key generated while creating certificates for secure connection
client-key-file = ""

# Path to the Root Certificate Authority used to sign both client and server certificates
root-ca-file = ""


#######################################################################
###                 Advanced Configuration Options                  ###
#######################################################################

#######################################################
###       RPC Server Configuration Options          ###
#######################################################
[rpc]

# TCP or UNIX socket address for the RPC server to listen on
laddr = "tcp://127.0.0.1:26657"

# A list of origins a cross-domain request can be executed from
# Default value '[]' disables cors support
# Use '["*"]' to allow any origin
cors-allowed-origins = []

# A list of methods the client is allowed to use with cross-domain requests
cors-allowed-methods = ["HEAD", "GET", "POST", ]

# A list of non simple headers the client is allowed to use with cross-domain requests
cors-allowed-headers = ["Origin", "Accept", "Content-Type", "X-Requested-With", "X-Server-Time", ]

# Activate unsafe RPC commands like /dial-seeds and /unsafe-flush-mempool
unsafe = false

# Maximum number of simultaneous connections (including WebSocket).
# If you want to accept a larger number than the default, make sure
# you increase your OS limits.
# 0 - unlimited.
# Should be < {ulimit -Sn} - {MaxNumInboundPeers} - {MaxNumOutboundPeers} - {N of wal, db and other open files}
# 1024 - 40 - 10 - 50 = 924 = ~900
max-open-connections = 900

# Maximum number of unique clientIDs that can /subscribe
# If you're using /broadcast_tx_commit, set to the estimated maximum number
# of broadcast_tx_commit calls per block.
max-subscription-clients = 100

# Maximum number of unique queries a given client can /subscribe to
# If you're using a Local RPC client and /broadcast_tx_commit, set this
# to the estimated maximum number of broadcast_tx_commit calls per block.
max-subscriptions-per-client = 5

# If true, disable the websocket interface to the RPC service.  This has
# the effect of disabling the /subscribe, /unsubscribe, and /unsubscribe_all
# methods for event subscription.
#
# EXPERIMENTAL: This setting will be removed in Tendermint v0.37.
experimental-disable-websocket = false

# The time window size for the event log. All events up to this long before
# the latest (up to EventLogMaxItems) will be available for subscribers to
# fetch via the /events method.  If 0 (the default) the event log and the
# /events RPC method are disabled.
event-log-window-size = "0s"

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
event-log-max-items = 0

# How long to wait for a tx to be committed during /broadcast_tx_commit.
# WARNING: Using a value larger than 10s will result in increasing the
# global HTTP write timeout, which applies to all connections and endpoints.
# See https://github.com/tendermint/tendermint/issues/3435
timeout-broadcast-tx-commit = "10s"

# Maximum size of request body, in bytes
max-body-bytes = 1000000

# Maximum size of request header, in bytes
max-header-bytes = 1048576

# The path to a file containing certificate that is used to create the HTTPS server.
# Might be either absolute path or path related to Tendermint's config directory.
# If the certificate is signed by a certificate authority,
# the certFile should be the concatenation of the server's certificate, any intermediates,
# and the CA's certificate.
# NOTE: both tls-cert-file and tls-key-file must be present for Tendermint to create HTTPS server.
# Otherwise, HTTP server is run.
tls-cert-file = ""

# The path to a file containing matching private key that is used to create the HTTPS server.
# Might be either absolute path or path related to Tendermint's config directory.
# NOTE: both tls-cert-file and tls-key-file must be present for Tendermint to create HTTPS server.
# Otherwise, HTTP server is run.
tls-key-file = ""

# pprof listen address (https://golang.org/pkg/net/http/pprof)
pprof-laddr = ""

#######################################################
###           P2P Configuration Options             ###
#######################################################
[p2p]

# Select the p2p internal queue
queue-type = "priority"

# Address to listen for incoming connections
laddr = "tcp://0.0.0.0:26656"

# Address to advertise to peers for them to dial
# If empty, will use the same port as the laddr,
# and will introspect on the listener or use UPnP
# to figure out the address. ip and port are required
# example: 159.89.10.97:26656
external-address = ""

# Comma separated list of seed nodes to connect to
# We only use these if we can’t connect to peers in the addrbook
# NOTE: not used by the new PEX reactor. Please use BootstrapPeers instead.
# TODO: Remove once p2p refactor is complete
# ref: https:#github.com/tendermint/tendermint/issues/5670
seeds = ""

# Comma separated list of peers to be added to the peer store
# on startup. Either BootstrapPeers or PersistentPeers are
# needed for peer discovery
bootstrap-peers = ""

# Comma separated list of nodes to keep persistent connections to
persistent-peers = ""

# UPNP port forwarding
upnp = false

# Maximum number of connections (inbound and outbound).
max-connections = 64

# Rate limits the number of incoming connection attempts per IP address.
max-incoming-connection-attempts = 100

# Set true to enable the peer-exchange reactor
pex = true

# Comma separated list of peer IDs to keep private (will not be gossiped to other peers)
# Warning: IPs will be exposed at /net_info, for more information https://github.com/tendermint/tendermint/issues/3055
private-peer-ids = ""

# Toggle to disable guard against peers connecting from the same ip.
allow-duplicate-ip = false

# Peer connection configuration.
handshake-timeout = "20s"
dial-timeout = "3s"

# Time to wait before flushing messages out on the connection
# TODO: Remove once MConnConnection is removed.
flush-throttle-timeout = "100ms"

# Maximum size of a message packet payload, in bytes
# TODO: Remove once MConnConnection is removed.
max-packet-msg-payload-size = 1400

# Rate at which packets can be sent, in bytes/second
# TODO: Remove once MConnConnection is removed.
send-rate = 5120000

# Rate at which packets can be received, in bytes/second
# TODO: Remove once MConnConnection is removed.
recv-rate = 5120000


#######################################################
###          Mempool Configuration Option          ###
#######################################################
[mempool]

recheck = true
broadcast = true

# Maximum number of transactions in the mempool
size = 5000

# Limit the total size of all txs in the mempool.
# This only accounts for raw transactions (e.g. given 1MB transactions and
# max-txs-bytes=5MB, mempool will only accept 5 transactions).
max-txs-bytes = 1073741824

# Size of the cache (used to filter transactions we saw earlier) in transactions
cache-size = 10000

# Do not remove invalid transactions from the cache (default: false)
# Set to true if it's not possible for any invalid transaction to become valid
# again in the future.
keep-invalid-txs-in-cache = false

# Maximum size of a single transaction.
# NOTE: the max size of a tx transmitted over the network is {max-tx-bytes}.
max-tx-bytes = 1048576

# Maximum size of a batch of transactions to send to a peer
# Including space needed by encoding (one varint per transaction).
# XXX: Unused due to https://github.com/tendermint/tendermint/issues/5796
max-batch-bytes = 0

# ttl-duration, if non-zero, defines the maximum amount of time a transaction
# can exist for in the mempool.
#
# Note, if ttl-num-blocks is also defined, a transaction will be removed if it
# has existed in the mempool at least ttl-num-blocks number of blocks or if it's
# insertion time into the mempool is beyond ttl-duration.
ttl-duration = "0s"

# ttl-num-blocks, if non-zero, defines the maximum number of blocks a transaction
# can exist for in the mempool.
#
# Note, if ttl-duration is also defined, a transaction will be removed if it
# has existed in the mempool at least ttl-num-blocks number of blocks or if
# it's insertion time into the mempool is beyond ttl-duration.
ttl-num-blocks = 0

#######################################################
###         State Sync Configuration Options        ###
#######################################################
[statesync]
# State sync rapidly bootstraps a new node by discovering, fetching, and restoring a state machine
# snapshot from peers instead of fetching and replaying historical blocks. Requires some peers in
# the network to take and serve state machine snapshots. State sync is not attempted if the node
# has any local state (LastBlockHeight > 0). The node will have a truncated block history,
# starting from the height of the snapshot.
enable = false

# State sync uses light client verification to verify state. This can be done either through the
# P2P layer or RPC layer. Set this to true to use the P2P layer. If false (default), RPC layer
# will be used.
use-p2p = false

# If using RPC, at least two addresses need to be provided. They should be compatible with net.Dial,
# for example: "host.example.com:2125"
rpc-servers = ""

# The hash and height of a trusted block. Must be within the trust-period.
trust-height = 0
trust-hash = ""

# The trust period should be set so that Tendermint can detect and gossip misbehavior before
# it is considered expired. For chains based on the Cosmos SDK, one day less than the unbonding
# period should suffice.
trust-period = "168h0m0s"

# Time to spend discovering snapshots before initiating a restore.
discovery-time = "15s"

# Temporary directory for state sync snapshot chunks, defaults to os.TempDir().
# The synchronizer will create a new, randomly named directory within this directory
# and remove it when the sync is complete.
temp-dir = ""

# The timeout duration before re-requesting a chunk, possibly from a different
# peer (default: 15 seconds).
chunk-request-timeout = "15s"

# The number of concurrent chunk and block fetchers to run (default: 4).
fetchers = "4"

#######################################################
###         Consensus Configuration Options         ###
#######################################################
[consensus]

wal-file = "data/cs.wal/wal"

# How many blocks to look back to check existence of the node's consensus votes before joining consensus
# When non-zero, the node will panic upon restart
# if the same consensus key was used to sign {double-sign-check-height} last blocks.
# So, validators should stop the state machine, wait for some blocks, and then restart the state machine to avoid panic.
double-sign-check-height = 0

# EmptyBlocks mode and possible interval between empty blocks
create-empty-blocks = true
create-empty-blocks-interval = "0s"

# Reactor sleep duration parameters
peer-gossip-sleep-duration = "100ms"
peer-query-maj23-sleep-duration = "2s"

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
# unsafe-propose-timeout-override = 0s

# This field provides an unsafe override of the ProposeDelta timeout consensus parameter.
# This field configures how much the propose timeout increases with each round.
# If this field is set to a value greater than 0, it will take effect.
# unsafe-propose-timeout-delta-override = 0s

# This field provides an unsafe override of the Vote timeout consensus parameter.
# This field configures how long the consensus engine will wait after
# receiving +2/3 votes in a around.
# If this field is set to a value greater than 0, it will take effect.
# unsafe-vote-timeout-override = 0s

# This field provides an unsafe override of the VoteDelta timeout consensus parameter.
# This field configures how much the vote timeout increases with each round.
# If this field is set to a value greater than 0, it will take effect.
# unsafe-vote-timeout-delta-override = 0s

# This field provides an unsafe override of the Commit timeout consensus parameter.
# This field configures how long the consensus engine will wait after receiving
# +2/3 precommits before beginning the next height.
# If this field is set to a value greater than 0, it will take effect.
# unsafe-commit-timeout-override = 0s

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
#   1) "null"
#   2) "kv" (default) - the simplest possible indexer, backed by key-value storage (defaults to levelDB; see DBBackend).
#   3) "psql" - the indexer services backed by PostgreSQL.
# When "kv" or "psql" is chosen "tx.height" and "tx.hash" will always be indexed.
indexer = ["kv"]

# The PostgreSQL connection configuration, the connection format:
#   postgresql://<user>:<password>@<host>:<port>/<db>?<opts>
psql-conn = ""

#######################################################
###       Instrumentation Configuration Options     ###
#######################################################
[instrumentation]

# When true, Prometheus metrics are served under /metrics on
# PrometheusListenAddr.
# Check out the documentation for the list of available metrics.
prometheus = false

# Address to listen for Prometheus collector(s) connections
prometheus-listen-addr = ":26660"

# Maximum number of simultaneous connections.
# If you want to accept a larger number than the default, make sure
# you increase your OS limits.
# 0 - unlimited.
max-open-connections = 3

# Instrumentation namespace
namespace = "tendermint"
```

## Empty blocks VS no empty blocks

### create-empty-blocks = true

If `create-empty-blocks` is set to `true` in your config, blocks will be
created ~ every second (with default consensus parameters). You can regulate
the delay between blocks by changing the `timeout-commit`. E.g. `timeout-commit = "10s"` should result in ~ 10 second blocks.

### create-empty-blocks = false

In this setting, blocks are created when transactions received.

Note after the block H, Tendermint creates something we call a "proof block"
(only if the application hash changed) H+1. The reason for this is to support
proofs. If you have a transaction in block H that changes the state to X, the
new application hash will only be included in block H+1. If after your
transaction is committed, you want to get a light-client proof for the new state
(X), you need the new block to be committed in order to do that because the new
block has the new application hash for the state X. That's why we make a new
(empty) block if the application hash changes. Otherwise, you won't be able to
make a proof for the new state.

Plus, if you set `create-empty-blocks-interval` to something other than the
default (`0`), Tendermint will be creating empty blocks even in the absence of
transactions every `create-empty-blocks-interval`. For instance, with
`create-empty-blocks = false` and `create-empty-blocks-interval = "30s"`,
Tendermint will only create blocks if there are transactions, or after waiting
30 seconds without receiving any transactions.


## P2P settings

This section will cover settings within the p2p section of the `config.toml`.

- `external-address` = is the address that will be advertised for other nodes to use. We recommend setting this field with your public IP and p2p port.
  - > We recommend setting an external address. When used in a private network, Tendermint Core currently doesn't advertise the node's public address. There is active and ongoing work to improve the P2P system, but this is a helpful workaround for now.
- `persistent-peers` = is a list of comma separated peers that you will always want to be connected to. If you're already connected to the maximum number of peers, persistent peers will not be added.
- `pex` = turns the peer exchange reactor on or off. Validator node will want the `pex` turned off so it would not begin gossiping to unknown peers on the network. PeX can also be turned off for statically configured networks with fixed network connectivity. For full nodes on open, dynamic networks, it should be turned on.
- `private-peer-ids` = is a comma-separated list of node ids that will _not_ be exposed to other peers (i.e., you will not tell other peers about the ids in this list). This can be filled with a validator's node id.

Recently the Tendermint Team conducted a refactor of the p2p layer. This lead to multiple config parameters being deprecated and/or replaced. 

We will cover the new and deprecated parameters below.
### New Parameters

There are three new parameters, which are enabled if use-legacy is set to false.

- `queue-type` = sets a type of queue to use in the p2p layer. There are three options available `fifo`, `priority` and `wdrr`. The default is priority
- `bootstrap-peers` = is a list of comma seperated peers which will be used to bootstrap the address book. 
- `max-connections` = is the max amount of allowed inbound and outbound connections.
### Deprecated Parameters

> Note: For Tendermint 0.35, there are two p2p implementations. The old version is used by default with the deprecated fields. The new implementation uses different config parameters, explained above.

- `max-num-inbound-peers` = is the maximum number of peers you will accept inbound connections from at one time (where they dial your address and initiate the connection). *This was replaced by `max-connections`*
- `max-num-outbound-peers` = is the maximum number of peers you will initiate outbound connects to at one time (where you dial their address and initiate the connection).*This was replaced by `max-connections`*
- `unconditional-peer-ids` = is similar to `persistent-peers` except that these peers will be connected to even if you are already connected to the maximum number of peers. This can be a validator node ID on your sentry node. *Deprecated*
- `seeds` = is a list of comma separated seed nodes that you will connect upon a start and ask for peers. A seed node is a node that does not participate in consensus but only helps propagate peers to nodes in the networks *Deprecated, replaced by bootstrap peers*

## Indexing Settings

Operators can configure indexing via the `[tx_index]` section. The `indexer`
field takes a series of supported indexers. If `null` is included, indexing will
be turned off regardless of other values provided.

### Supported Indexers

#### KV

The `kv` indexer type is an embedded key-value store supported by the main
underlying Tendermint database. Using the `kv` indexer type allows you to query
for block and transaction events directly against Tendermint's RPC. However, the
query syntax is limited and so this indexer type might be deprecated or removed
entirely in the future.

#### PostgreSQL

The `psql` indexer type allows an operator to enable block and transaction event
indexing by proxying it to an external PostgreSQL instance allowing for the events
to be stored in relational models. Since the events are stored in a RDBMS, operators
can leverage SQL to perform a series of rich and complex queries that are not
supported by the `kv` indexer type. Since operators can leverage SQL directly,
searching is not enabled for the `psql` indexer type via Tendermint's RPC -- any
such query will fail.

Note, the SQL schema is stored in `state/indexer/sink/psql/schema.sql` and operators
must explicitly create the relations prior to starting Tendermint and enabling
the `psql` indexer type.

Example:

```shell
$ psql ... -f state/indexer/sink/psql/schema.sql
```

## Unsafe Consensus Timeout Overrides

Tendermint version v0.36 provides a set of unsafe overrides for the consensus
timing parameters. These parameters are provided as a safety measure in case of
unusual timing issues during the upgrade to v0.36 so that an operator may
override the timings for a single node. These overrides will completely be
removed in Tendermint v0.37.

- `unsafe-propose-override`: How long the Tendermint consensus engine will wait
  for a proposal block before prevoting nil.
- `unsafe-propose-delta-override`: How much the propose timeout increase with
  each round.
- `unsafe-vote-override`: How long the consensus engine will wait after
  receiving +2/3 votes in a round.
- `unsafe-vote-delta-override`: How much the vote timeout increases with each
  round.
- `unsafe-commit-override`: How long the consensus engine will wait after
  receiving +2/3 precommits before beginning the next height.
- `unsafe-bypass-commit-timeout-override`: Configures if the consensus engine
  will wait for the full commit timeout before proceeding to the next height. If
  this field is set to true, the consensus engine will proceed to the next
  height as soon as the node has gathered votes from all of the validators on
  the network.
