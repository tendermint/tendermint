## v0.30.0

*TBD*

Special thanks to external contributors on this release:

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API
  - [types] \#3245 Commit uses `type CommitSig Vote` instead of `Vote` directly.

* Blockchain Protocol

* P2P Protocol

### FEATURES:
- [mempool] \#3079 bound mempool memory usage (`mempool.max_txs_total_bytes` is set to 1GB by default; see config.toml)
  mempool's current `txs_total_bytes` is exposed via `total_bytes` field in
  `/num_unconfirmed_txs` and `/unconfirmed_txs` RPC endpoints.

### IMPROVEMENTS:
- [tools] add go-deadlock tool to help detect deadlocks
- [crypto] \#3163 use ethereum's libsecp256k1 go-wrapper for signatures when cgo is available
- [crypto] \#3162 wrap btcd instead of forking it to keep up with fixes (used if cgo is not available)

### BUG FIXES:
- [node] \#3186 EventBus and indexerService should be started before first block (for replay last block on handshake) execution
- [p2p] \#3247 Fix panic in SeedMode when calling FlushStop and OnStop
  concurrently
