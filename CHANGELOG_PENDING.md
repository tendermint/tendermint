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

### IMPROVEMENTS:

### BUG FIXES:

- [p2p/conn] \#3347 Reject all-zero shared secrets in the Diffie-Hellman step of secret-connection
- [libs/pubsub] \#951, \#1880 use non-blocking send when dispatching messages [ADR-33](https://github.com/tendermint/tendermint/blob/develop/docs/architecture/adr-033-pubsub.md)
