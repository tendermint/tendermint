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

- [types] [\#1571](https://github.com/tendermint/tendermint/issues/1571) Enable subscription to tags emitted from `BeginBlock`/`EndBlock` (@kostko)

### IMPROVEMENTS:

- [config] \#2877 add blocktime_iota to the config.toml (@ackratos)
- [mempool] \#2855 add txs from Update to cache
- [mempool] \#2835 Remove local int64 counter from being stored in every tx
- [node] \#2827 add ability to instantiate IPCVal (@joe-bowman)

### BUG FIXES:

- [blockchain] \#2731 Retry both blocks if either is bad to avoid getting stuck during fast sync (@goolAdapter)
- [log] \#2868 fix module=main setting overriding all others
- [rpc] \#2808 RPC validators calls IncrementAccum if necessary
- [rpc] \#2811 Allow integer IDs in JSON-RPC requests
