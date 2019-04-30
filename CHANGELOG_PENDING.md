## v0.31.6

**

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API
- [libs/common] Removed `PanicSanity`, `PanicCrisis`, `PanicConsensus` and `PanicQ`
- [node] Moved `GenesisDocProvider` and `DefaultGenesisDocProviderFunc` to state package

* Blockchain Protocol

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:
- [rpc] [\#3534](https://github.com/tendermint/tendermint/pull/3534) Add support for batched requests/responses in JSON RPC
- [cli] [\#3160](https://github.com/tendermint/tendermint/issues/3160) Add `-config=<path-to-config>` option to `testnet` cmd (@gregdhill)
- [cs/replay] \#3460 check appHash for each block

### BUG FIXES:
- [p2p] \#3532 limit the number of attempts to connect to a peer in seed mode
  to 16 (as a result, the node will stop retrying after a 35 hours time window)
- [consensus] \#2723, \#3451 and \#3317 Fix non-deterministic tests
- [pex] \#3603 Dial seeds when addrbook needs more addresses (@defunctzombie)