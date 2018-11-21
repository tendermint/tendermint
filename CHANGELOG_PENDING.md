# Pending

## v0.26.4

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API

* Blockchain Protocol

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:

- [config] \#2877 add blocktime_iota to the config.toml (@ackratos)
- [mempool] \#2855 add txs from Update to cache
- [mempool] \#2835 Remove local int64 counter from being stored in every tx
- [node] \#2827 add ability to instantiate IPCVal (@joe-bowman)

### BUG FIXES:

- [blockchain] \#2731 Retry both blocks if either is bad to avoid getting stuck during fast sync (@goolAdapter)

- [rpc] \#2808 RPC validators calls IncrementAccum if necessary
