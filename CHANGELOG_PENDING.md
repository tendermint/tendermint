## v0.31.0

**

Special thanks to external contributors on this release:
@srmo

### BREAKING CHANGES:

* CLI/RPC/Config
- [rpc/client] Update Subscribe interface to reflect new pubsub/eventBus API [ADR-33](https://github.com/tendermint/tendermint/blob/develop/docs/architecture/adr-033-pubsub.md)

* Apps

* Go API
- [libs/common] TrapSignal accepts logger as a first parameter and does not block anymore
  * previously it was dumping "captured ..." msg to os.Stdout
  * TrapSignal should not be responsible for blocking thread of execution

* Blockchain Protocol

* P2P Protocol

### FEATURES:
- [mempool] \#3079 bound mempool memory usage (`mempool.max_txs_bytes` is set to 1GB by default; see config.toml)
  mempool's current `txs_total_bytes` is exposed via `total_bytes` field in
  `/num_unconfirmed_txs` and `/unconfirmed_txs` RPC endpoints.
- [config] \#2920 Remove `consensus.blocktime_iota` parameter
- [genesis] \#2920 Add `time_iota_ms` to block's consensus parameters (not exposed to the application)
- [genesis] \#2920 Rename `consensus_params.block_size` to `consensus_params.block`
- [lite] add `/unsubscribe_all` endpoint, which allows you to unsubscribe from all events

### IMPROVEMENTS:
- [libs/common] \#3238 exit with zero (0) code upon receiving SIGTERM/SIGINT
- [libs/db] \#3378 CLevelDB#Stats now returns the following properties:
  - leveldb.num-files-at-level{n}
  - leveldb.stats
  - leveldb.sstables
  - leveldb.blockpool
  - leveldb.cachedblock
  - leveldb.openedtables
  - leveldb.alivesnaps
  - leveldb.aliveiters

### BUG FIXES:
- [p2p/conn] \#3347 Reject all-zero shared secrets in the Diffie-Hellman step of secret-connection
- [libs/pubsub] \#951, \#1880 use non-blocking send when dispatching messages [ADR-33](https://github.com/tendermint/tendermint/blob/develop/docs/architecture/adr-033-pubsub.md)
- [p2p] \#3369 do not panic when filter times out
- [cmd] \#3408 Fix `testnet` command's panic when creating non-validator configs (using `--n` flag) (@srmo)
