Configuration
=============

Tendermint Core can be configured via a TOML file in
``$TMHOME/config/config.toml``. Some of these parameters can be overridden by
command-line flags. For most users, the options in the ``##### main
base configuration options #####`` are intended to be modified while
config options further below are intended for advance power users.

Config options
~~~~~~~~~~~~~~

The default configuration file create by ``tendermint init`` has all
the parameters set with their default values. It will look something
like the file below, however, double check by inspecting the 
``config.toml`` created with your version of ``tendermint`` installed:

::

    # This is a TOML config file.
    # For more information, see https://github.com/toml-lang/toml
    
    ##### main base config options #####
    
    # TCP or UNIX socket address of the ABCI application,
    # or the name of an ABCI application compiled in with the Tendermint binary
    proxy_app = "tcp://127.0.0.1:46658"
    
    # A custom human readable name for this node
    moniker = "anonymous"
    
    # If this node is many blocks behind the tip of the chain, FastSync
    # allows them to catchup quickly by downloading blocks in parallel
    # and verifying their commits
    fast_sync = true
    
    # Database backend: leveldb | memdb
    db_backend = "leveldb"
    
    # Database directory
    db_path = "data"
    
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
    
    # What indexer to use for transactions
    tx_index = "kv"
    
    ##### advanced configuration options #####
    
    ##### rpc server configuration options #####
    [rpc]
    
    # TCP or UNIX socket address for the RPC server to listen on
    laddr = "tcp://0.0.0.0:46657"
    
    # TCP or UNIX socket address for the gRPC server to listen on
    # NOTE: This server only supports /broadcast_tx_commit
    grpc_laddr = ""
    
    # Activate unsafe RPC commands like /dial_seeds and /unsafe_flush_mempool
    unsafe = false
    
    ##### peer to peer configuration options #####
    [p2p]
    
    # Address to listen for incoming connections
    laddr = "tcp://0.0.0.0:46656"
    
    # Comma separated list of seed nodes to connect to
    seeds = ""
    
    # Path to address book
    addr_book_file = "addrbook.json"
    
    # Set true for strict address routability rules
    addr_book_strict = true
    
    # Time to wait before flushing messages out on the connection, in ms
    flush_throttle_timeout = 100
    
    # Maximum number of peers to connect to
    max_num_peers = 50
    
    # Maximum size of a message packet payload, in bytes
    max_msg_packet_payload_size = 1024
    
    # Rate at which packets can be sent, in bytes/second
    send_rate = 512000
    
    # Rate at which packets can be received, in bytes/second
    recv_rate = 512000
    
    ##### mempool configuration options #####
    [mempool]
    
    recheck = true
    recheck_empty = true
    broadcast = true
    wal_dir = "data/mempool.wal"
    
    ##### consensus configuration options #####
    [consensus]
    
    wal_file = "data/cs.wal/wal"
    wal_light = false
    
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
    
    # BlockSize
    max_block_size_txs = 10000
    max_block_size_bytes = 1
    
    # EmptyBlocks mode and possible interval between empty blocks in seconds
    create_empty_blocks = true
    create_empty_blocks_interval = 0
    
    # Reactor sleep duration parameters are in milliseconds
    peer_gossip_sleep_duration = 100
    peer_query_maj23_sleep_duration = 2000
