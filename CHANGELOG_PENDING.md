## v0.31.5

**

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API

* Blockchain Protocol

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:
- [rpc] [\#3534](https://github.com/tendermint/tendermint/pull/3534) Add support for batched requests/responses in JSON RPC
- [cli] [\#3160](https://github.com/tendermint/tendermint/issues/3160) Add `-config=<path-to-config>` option to `testnet` cmd (@gregdhill)
- [cs/replay] \#3460 check appHash for each block

### BUG FIXES:
- [state] [\#3537](https://github.com/tendermint/tendermint/pull/3537#issuecomment-482711833) LoadValidators: do not return an empty validator set
- [p2p] \#3532 limit the number of attempts to connect to a peer in seed mode
  to 16 (as a result, the node will stop retrying after a 35 hours time window)
