## v0.27.1

*TBD*

Special thanks to external contributors on this release:

### BREAKING CHANGES:

* CLI/RPC/Config
- [config] `allow_duplicate_ip` is now set to false

* Apps

* Go API

* Blockchain Protocol

* P2P Protocol
- multiple connections from the same IP are now disabled by default (see `allow_duplicate_ip` config option)

### FEATURES:

### IMPROVEMENTS:
- [rpc] Add `UnconfirmedTxs(limit)` and `NumUnconfirmedTxs()` methods to HTTP/Local clients (@danil-lashin)

### BUG FIXES:
- [kv indexer] \#2912 don't ignore key when executing CONTAINS
- [config] \#2980 fix cors options formatting
- [p2p] \#2715 fix a bug where seeds don't disconnect from a peer after 3h
