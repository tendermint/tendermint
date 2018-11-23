# Pending

## v0.26.4

*TBD*

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

- [rpc] \#2808 RPC validators calls IncrementAccum if necessary
- [kv indexer] \#2775 order results by index if height is the same
