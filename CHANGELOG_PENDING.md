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
- [log] new `log_format` config option, which can be set to 'plain' for colored
  text or 'json' for JSON output

### IMPROVEMENTS:

### BUG FIXES:

- [abci] unlock mutex in localClient so even when app panics (e.g. during CheckTx), consensus continue working
- [abci] fix DATA RACE in localClient
- [rpc] drain channel before calling Unsubscribe(All) in /broadcast_tx_commit
