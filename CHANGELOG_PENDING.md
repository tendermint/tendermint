## v0.31.0

**

Special thanks to external contributors on this release:

### BREAKING CHANGES:

* CLI/RPC/Config
- [httpclient] Update Subscribe interface to reflect new pubsub/eventBus API [ADR-33](https://github.com/tendermint/tendermint/blob/develop/docs/architecture/adr-033-pubsub.md)

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

- [p2p/conn] \#3347 Reject all-zero shared secrets in the Diffie-Hellman step of secret-connection
- [libs/pubsub] \#951, \#1880 use non-blocking send when dispatching messages [ADR-33](https://github.com/tendermint/tendermint/blob/develop/docs/architecture/adr-033-pubsub.md)
