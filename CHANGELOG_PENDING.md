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

* Blockchain Protocol

* P2P Protocol

### FEATURES:
- [node] \#2659 Add `node.Mempool()` method, which allows you to access mempool

### IMPROVEMENTS:
- [p2p] [\#3463](https://github.com/tendermint/tendermint/pull/3463) Do not log "Can't add peer's address to addrbook" error for a private peer
- [p2p] [\#3552](https://github.com/tendermint/tendermint/pull/3552) Add PeerBehaviour Interface (@brapse)
- [rpc] [\#3534](https://github.com/tendermint/tendermint/pull/3534) Add support for batched requests/responses in JSON RPC
- [cli] [\#3661](https://github.com/tendermint/tendermint/pull/3661) Add
  `--hostname-suffix`, `--hostname` and `--random-monikers` options to `testnet`
  cmd for greater peer address/identity generation flexibility.
- [cli] \#3585 Add option to not clear address book with unsafe reset (@climber73)
- [cli] [\#3160](https://github.com/tendermint/tendermint/issues/3160) Add `--config=<path-to-config>` option to `testnet` cmd (@gregdhill)
- [cs/replay] \#3460 check appHash for each block
- [rpc] \#3362 `/dial_seeds` & `/dial_peers` return errors if addresses are incorrect (except when IP lookup fails)
- [node] \#3362 returns an error if `persistent_peers` list is invalid (except when IP lookup fails)
- [p2p] \#3531 Terminate session on nonce wrapping (@climber73)
- [libs/db] \#3611 Conditional compilation
  * Use `cleveldb` tag instead of `gcc` to compile Tendermint with CLevelDB or
    use `make build_c` / `make install_c` (full instructions can be found at
    https://tendermint.com/docs/introduction/install.html#compile-with-cleveldb-support)
  * Use `boltdb` tag to compile Tendermint with bolt db

### BUG FIXES:
- [p2p] \#3532 limit the number of attempts to connect to a peer in seed mode
  to 16 (as a result, the node will stop retrying after a 35 hours time window)
- [consensus] \#2723, \#3451 and \#3317 Fix non-deterministic tests
- [p2p] \#3362 make persistent prop independent of conn direction
  * `Switch#DialPeersAsync` now only takes a list of peers
  * `Switch#DialPeerWithAddress` now only takes an address
- [consensus] \#3067 getBeginBlockValidatorInfo loads validators from stateDB instead of state (@james-ray)
- [pex] \#3603 Dial seeds when addrbook needs more addresses (@defunctzombie)
- [mempool] \#3322 Remove only valid (Code==0) txs on Update
  * `Mempool#Update` and `BlockExecutor#Commit` now accept
    `[]*abci.ResponseDeliverTx` - list of `DeliverTx` responses, which should
    match `block.Txs`
