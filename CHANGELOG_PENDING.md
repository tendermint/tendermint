## v0.30.0

*TBD*

Special thanks to external contributors on this release:

### BREAKING CHANGES:

* CLI/RPC/Config
- [httpclient] Update Subscribe interface to reflect new pubsub/eventBus API [ADR-33](https://github.com/tendermint/tendermint/blob/develop/docs/architecture/adr-033-pubsub.md)

* Apps

* Go API
  - [types] \#3245 Commit uses `type CommitSig Vote` instead of `Vote` directly.

* Blockchain Protocol

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:
- [tools] Add go-deadlock tool to help detect deadlocks
- [tools] \#3106 Add tm-signer-harness test harness for remote signers
- [crypto] \#3163 Use ethereum's libsecp256k1 go-wrapper for signatures when cgo is available
- [crypto] \#3162 Wrap btcd instead of forking it to keep up with fixes (used if cgo is not available)

### BUG FIXES:
- [node] \#3186 EventBus and indexerService should be started before first block (for replay last block on handshake) execution
- [p2p] \#3232 Fix infinite loop leading to addrbook deadlock for seed nodes
- [p2p] \#3247 Fix panic in SeedMode when calling FlushStop and OnStop
  concurrently
- [libs/pubsub] \#951, \#1880 use non-blocking send when dispatching messages [ADR-33](https://github.com/tendermint/tendermint/blob/develop/docs/architecture/adr-033-pubsub.md)
