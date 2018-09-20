# Configuration

Tendermint Core can be configured via a TOML file in
`$TMHOME/config/config.toml`. Some of these parameters can be overridden by
command-line flags. For most users, the options in the `##### main base configuration options #####` are intended to be modified while
config options further below are intended for advance power users.

## Options

The default configuration file create by `tendermint init` has all
the parameters set with their default values. It will look something
like the file below, however, double check by inspecting the
`config.toml` created with your version of `tendermint` installed:

```
# This is a TOML config file.
# For more information, see https://github.com/toml-lang/toml

##### main base config options #####

# TCP or UNIX socket address of the ABCI application,
# or the name of an ABCI application compiled in with the Tendermint binary
proxy_app = "tcp://127.0.0.1:26658"

# A custom human readable name for this node
moniker = "anonymous"

# If this node is many blocks behind the tip of the chain, FastSync
# allows them to catchup quickly by downloading blocks in parallel
# and verifying their commits
fast_sync = true

# Database backend: leveldb | memdb | cleveldb
db_backend = "leveldb"

# Database directory
db_dir = "data"

# Output level for logging
log_level = "state:info,*:error"

##### additional base config options #####

# The ID of the chain to join (should be signed with every transaction and vote)
chain_id = ""

# Path to the JSON file containing the initial validator set and other meta data
genesis_file = "genesis.json"

# Path to the JSON file containing the private key to use as a validator in the consensus protocol
priv_validator_file = "priv_validator.json"

# Mechanism to connect to the ABCI application: socket | grpc
abci = "socket"

# TCP or UNIX socket address for the profiling server to listen on
prof_laddr = ""

# If true, query the ABCI app on connecting to a new peer
# so the app can decide if we should keep the connection or not
filter_peers = false

##### advanced configuration options #####

##### rpc server configuration options #####
[rpc]

# TCP or UNIX socket address for the RPC server to listen on
laddr = "tcp://0.0.0.0:26657"

# TCP or UNIX socket address for the gRPC server to listen on
# NOTE: This server only supports /broadcast_tx_commit
grpc_laddr = ""

# Maximum number of simultaneous connections.
# Does not include RPC (HTTP&WebSocket) connections. See max_open_connections
# If you want to accept more significant number than the default, make sure
# you increase your OS limits.
# 0 - unlimited.
# Should be < {ulimit -Sn} - {MaxNumInboundPeers} - {MaxNumOutboundPeers} - {N of wal, db and other open files}
# 1024 - 40 - 10 - 50 = 924 = ~900
grpc_max_open_connections = 900

# Activate unsafe RPC commands like /dial_seeds and /unsafe_flush_mempool
unsafe = false

# Maximum number of simultaneous connections (including WebSocket).
# Does not include gRPC connections. See grpc_max_open_connections
# If you want to accept more significant number than the default, make sure
# you increase your OS limits.
# 0 - unlimited.
# Should be < {ulimit -Sn} - {MaxNumInboundPeers} - {MaxNumOutboundPeers} - {N of wal, db and other open files}
# 1024 - 40 - 10 - 50 = 924 = ~900
max_open_connections = 900

##### peer to peer configuration options #####
[p2p]

# Address to listen for incoming connections
laddr = "tcp://0.0.0.0:26656"

# Comma separated list of seed nodes to connect to
seeds = ""

# Comma separated list of nodes to keep persistent connections to
persistent_peers = ""

# UPNP port forwarding
upnp = false

# Path to address book
addr_book_file = "addrbook.json"

# Set true for strict address routability rules
# Set false for private or local networks
addr_book_strict = true

# Time to wait before flushing messages out on the connection, in ms
flush_throttle_timeout = 100

# Maximum number of inbound peers
max_num_inbound_peers = 40

# Maximum number of outbound peers to connect to, excluding persistent peers
max_num_outbound_peers = 10

# Maximum size of a message packet payload, in bytes
max_packet_msg_payload_size = 1024

# Rate at which packets can be sent, in bytes/second
send_rate = 5120000

# Rate at which packets can be received, in bytes/second
recv_rate = 5120000

# Set true to enable the peer-exchange reactor
pex = true

# Seed mode, in which node constantly crawls the network and looks for
# peers. If another node asks it for addresses, it responds and disconnects.
#
# Does not work if the peer-exchange reactor is disabled.
seed_mode = false

# Comma separated list of peer IDs to keep private (will not be gossiped to other peers)
private_peer_ids = ""

##### mempool configuration options #####
[mempool]

recheck = true
recheck_empty = true
broadcast = true
wal_dir = "data/mempool.wal"

# size of the mempool
size = 100000

# size of the cache (used to filter transactions we saw earlier)
cache_size = 100000

##### consensus configuration options #####
[consensus]

wal_file = "data/cs.wal/wal"

# All timeouts are in milliseconds
timeout_propose = 3000
timeout_propose_delta = 500
timeout_prevote = 1000
timeout_prevote_delta = 500
timeout_precommit = 1000
timeout_precommit_delta = 500
timeout_commit = 1000

# Make progress as soon as we have all the precommits (as if TimeoutCommit = 0)
skip_timeout_commit = false

# EmptyBlocks mode and possible interval between empty blocks in seconds
create_empty_blocks = true
create_empty_blocks_interval = 0

# Reactor sleep duration parameters are in milliseconds
peer_gossip_sleep_duration = 100
peer_query_maj23_sleep_duration = 2000

##### transactions indexer configuration options #####
[tx_index]

# What indexer to use for transactions
#
# Options:
#   1) "null"
#   2) "kv" (default) - the simplest possible indexer, backed by key-value storage (defaults to levelDB; see DBBackend).
indexer = "kv"

# Comma-separated list of tags to index (by default the only tag is "tx.hash")
#
# You can also index transactions by height by adding "tx.height" tag here.
# 
# It's recommended to index only a subset of tags due to possible memory
# bloat. This is, of course, depends on the indexer's DB and the volume of
# transactions.
index_tags = ""

# When set to true, tells indexer to index all tags (predefined tags:
# "tx.hash", "tx.height" and all tags from DeliverTx responses). 
#	
# Note this may be not desirable (see the comment above). IndexTags has a
# precedence over IndexAllTags (i.e. when given both, IndexTags will be
# indexed).
index_all_tags = false

##### instrumentation configuration options #####
[instrumentation]

# When true, Prometheus metrics are served under /metrics on
# PrometheusListenAddr.
# Check out the documentation for the list of available metrics.
prometheus = false

# Address to listen for Prometheus collector(s) connections
prometheus_listen_addr = ":26660"

# Maximum number of simultaneous connections.
# If you want to accept a more significant number than the default, make sure
# you increase your OS limits.
# 0 - unlimited.
max_open_connections = 3
```
