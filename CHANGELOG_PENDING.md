## v0.31.6

**

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API
- [mempool] \#2659 `Mempool` now an interface
  * old `Mempool` implementation renamed to `CListMempool`
  * `NewMempool` renamed to `NewCListMempool`
  * `Option` renamed to `CListOption`
  * unexpose `MempoolReactor.Mempool`
  * `MempoolReactor` renamed to `Reactor`
  * `NewMempoolReactor` renamed to `NewReactor`
  * unexpose `TxID` method
  * `TxInfo.PeerID` renamed to `SenderID`
- [state] \#2659 `Mempool` interface moved to mempool package
  * `MockMempool` moved to top-level mock package and renamed to `Mempool`
- [libs/common] Removed `PanicSanity`, `PanicCrisis`, `PanicConsensus` and `PanicQ`
- [node] Moved `GenesisDocProvider` and `DefaultGenesisDocProviderFunc` to state package
- [libs/db] \#3611 Removed LevelDBBackend

* Blockchain Protocol

* P2P Protocol

### FEATURES:
- [node] \#2659 Add `node.Mempool()` method, which allows you to access mempool

### IMPROVEMENTS:
- [rpc] [\#3534](https://github.com/tendermint/tendermint/pull/3534) Add support for batched requests/responses in JSON RPC
- [cli] \#3585 Add option to not clear address book with unsafe reset (@climber73)
- [cli] [\#3160](https://github.com/tendermint/tendermint/issues/3160) Add `-config=<path-to-config>` option to `testnet` cmd (@gregdhill)
- [cs/replay] \#3460 check appHash for each block
- [rpc] \#3362 `/dial_seeds` & `/dial_peers` return errors if addresses are incorrect (except when IP lookup fails)
- [node] \#3362 returns an error if `persistent_peers` list is invalid (except when IP lookup fails)
- [p2p] \#3531 Terminate session on nonce wrapping (@climber73)
- [libs/db] \#3611 Conditional compilation
  * Use `cleveldb` tag instead of `gcc` to compile Tendermint with CLevelDB or
    use `make build_c` / `make install_c`

### BUG FIXES:
- [p2p] \#3532 limit the number of attempts to connect to a peer in seed mode
  to 16 (as a result, the node will stop retrying after a 35 hours time window)
- [consensus] \#2723, \#3451 and \#3317 Fix non-deterministic tests
- [p2p] \#3362 make persistent prop independent of conn direction
  * `Switch#DialPeersAsync` now only takes a list of peers
  * `Switch#DialPeerWithAddress` now only takes an address
- [consensus] \#3067 getBeginBlockValidatorInfo loads validators from stateDB instead of state (@james-ray)
- [pex] \#3603 Dial seeds when addrbook needs more addresses (@defunctzombie)
