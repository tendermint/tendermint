## v0.29.2

*February 7th, 2019*

Special thanks to external contributors on this release:
@ackratos, ...

... we fixed non-deterministic tests ...

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API
  - [types] [\#3245](https://github.com/tendermint/tendermint/issues/3245) Commit uses `type CommitSig Vote` instead of `Vote` directly.
    In preparation for removing redundant fields from the commit [\#1648](https://github.com/tendermint/tendermint/issues/1648)

* Blockchain Protocol

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:
- [tools] Add go-deadlock tool to help detect deadlocks
- [tools] [\#3106](https://github.com/tendermint/tendermint/issues/3106) Add tm-signer-harness test harness for remote signers
- [crypto] [\#3163](https://github.com/tendermint/tendermint/issues/3163) Use ethereum's libsecp256k1 go-wrapper for signatures when cgo is available
- [crypto] [\#3162](https://github.com/tendermint/tendermint/issues/3162) Wrap btcd instead of forking it to keep up with fixes (used if cgo is not available)
- [tests] [\#3258](https://github.com/tendermint/tendermint/issues/3258) Fixed a bunch of non-deterministic test failures
- [makefile] [\#3233](https://github.com/tendermint/tendermint/issues/3233) Use golangci-lint instead of go-metalinter

### BUG FIXES:
- [node] [\#3186](https://github.com/tendermint/tendermint/issues/3186) EventBus and indexerService should be started before first block (for replay last block on handshake) execution
- [p2p] [\#3232](https://github.com/tendermint/tendermint/issues/3232) Fix infinite loop leading to addrbook deadlock for seed nodes
- [p2p] [\#3247](https://github.com/tendermint/tendermint/issues/3247) Fix panic in SeedMode when calling FlushStop and OnStop
  concurrently
- [privval] [\#3258](https://github.com/tendermint/tendermint/issues/3258) Fix race between sign requests and ping requests in socket
- [p2p] [\#3040](https://github.com/tendermint/tendermint/issues/3040) Fix MITM on secret connection by checking low-order points
