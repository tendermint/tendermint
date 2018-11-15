# Pending

## v0.26.2

*November 15th, 2018*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### FEATURES:

- [rpc] [\#2582](https://github.com/tendermint/tendermint/issues/2582) Enable CORS on RPC API

### IMPROVEMENTS:

### BUG FIXES:

- [abci] \#2748 Unlock mutex in localClient so even when app panics (e.g. during CheckTx), consensus continue working
- [abci] \#2748 Fix DATA RACE in localClient
- [amino] \#2822 Update to v0.14.1 to support compiling on 32-bit platforms
- [rpc] \#2748 Drain channel before calling Unsubscribe(All) in `/broadcast_tx_commit`
