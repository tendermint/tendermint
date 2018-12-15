## v0.27.1

*TBD*

Special thanks to external contributors on this release:
@danil-lashin, @hleb-albau, @james-ray, @leo-xinwang

### FEATURES:
- [rpc] [\#2964](https://github.com/tendermint/tendermint/issues/2964) Add `UnconfirmedTxs(limit)` and `NumUnconfirmedTxs()` methods to HTTP/Local clients (@danil-lashin)
- [docs] [\#3004](https://github.com/tendermint/tendermint/issues/3004) Enable full-text search on docs pages

### IMPROVEMENTS:
- [consensus] [\#2971](https://github.com/tendermint/tendermint/issues/2971) Return error if ValidatorSet is empty after InitChain
  (@leo-xinwang)
- [ci/cd] [\#3005](https://github.com/tendermint/tendermint/issues/3005) Updated CircleCI job to trigger website build when docs are updated
- [docs] Various updates

### BUG FIXES:
- [cmd] [\#2983](https://github.com/tendermint/tendermint/issues/2983) `testnet` command always sets `addr_book_strict = false`
- [config] [\#2980](https://github.com/tendermint/tendermint/issues/2980) Fix CORS options formatting
- [kv indexer] [\#2912](https://github.com/tendermint/tendermint/issues/2912) Don't ignore key when executing CONTAINS
- [mempool] [\#2961](https://github.com/tendermint/tendermint/issues/2961) Call `notifyTxsAvailable` if there're txs left after committing a block, but recheck=false
- [mempool] [\#2994](https://github.com/tendermint/tendermint/issues/2994) Reject txs with negative GasWanted
- [p2p] [\#2990](https://github.com/tendermint/tendermint/issues/2990) Fix a bug where seeds don't disconnect from a peer after 3h
- [consensus] [\#3006](https://github.com/tendermint/tendermint/issues/3006) Save state after InitChain only when stateHeight is also 0 (@james-ray)
