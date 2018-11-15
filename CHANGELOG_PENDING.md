# Pending

## v0.26.2

*TBA*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API

* Blockchain Protocol

* P2P Protocol

### FEATURES:

- [rpc] [\#2582](https://github.com/tendermint/tendermint/issues/2582) Enable CORS on RPC API

### IMPROVEMENTS:

- [log] [\#2765](https://github.com/tendermint/tendermint/issues/2765) Make "Update to validators" msg value pretty

### BUG FIXES:

- [abci] unlock mutex in localClient so even when app panics (e.g. during CheckTx), consensus continue working
- [abci] fix DATA RACE in localClient
- [rpc] drain channel before calling Unsubscribe(All) in /broadcast_tx_commit
