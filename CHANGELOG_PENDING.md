## v0.31.0

**

Special thanks to external contributors on this release:

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API

* Blockchain Protocol

* P2P Protocol

### FEATURES:
- [mempool] \#3079 bound mempool memory usage (`mempool.max_txs_bytes` is set to 1GB by default; see config.toml)
  mempool's current `txs_total_bytes` is exposed via `total_bytes` field in
  `/num_unconfirmed_txs` and `/unconfirmed_txs` RPC endpoints.

### IMPROVEMENTS:

### BUG FIXES:
