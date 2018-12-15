## v0.27.1

*TBD*

Special thanks to external contributors on this release:

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API

* Blockchain Protocol

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:
- [rpc] Add `UnconfirmedTxs(limit)` and `NumUnconfirmedTxs()` methods to HTTP/Local clients (@danil-lashin)
- [ci/cd] Updated CircleCI job to trigger website build when docs are updated

### BUG FIXES:
- [kv indexer] \#2912 don't ignore key when executing CONTAINS
- [mempool] \#2961 notifyTxsAvailable if there're txs left after committing a block, but recheck=false
- [mempool] \#2994 Don't allow txs with negative gas wanted
- [p2p] \#2715 fix a bug where seeds don't disconnect from a peer after 3h
