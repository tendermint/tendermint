# Pending

## v0.26.4

*November 27th, 2018*

Special thanks to external contributors on this release:
ackratos, goolAdapter, james-ray, joe-bowman, kostko,
nagarajmanjunath, tomtau


Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### FEATURES:

- [rpc] [\#2747](https://github.com/tendermint/tendermint/issues/2747) Enable subscription to tags emitted from `BeginBlock`/`EndBlock` (@kostko)
- [types] [\#2747](https://github.com/tendermint/tendermint/issues/2747) Add `ResultBeginBlock` and `ResultEndBlock` fields to `EventDataNewBlock`
    and `EventDataNewBlockHeader` to support subscriptions (@kostko)
- [types] [\#2918](https://github.com/tendermint/tendermint/issues/2918) Add Marshal, MarshalTo, Unmarshal methods to various structs
  to support Protobuf compatibility (@nagarajmanjunath)

### IMPROVEMENTS:

- [config] [\#2877](https://github.com/tendermint/tendermint/issues/2877) Add `blocktime_iota` to the config.toml (@ackratos)
    - NOTE: this should be a ConsensusParam, not part of the config, and will be
      removed from the config at a later date.
- [mempool] [\#2882](https://github.com/tendermint/tendermint/issues/2882) Add txs from Update to cache
- [mempool] [\#2891](https://github.com/tendermint/tendermint/issues/2891) Remove local int64 counter from being stored in every tx
- [node] [\#2866](https://github.com/tendermint/tendermint/issues/2866) Add ability to instantiate IPCVal (@joe-bowman)

### BUG FIXES:

- [blockchain] [\#2731](https://github.com/tendermint/tendermint/issues/2731) Retry both blocks if either is bad to avoid getting stuck during fast sync (@goolAdapter)
- [consensus] [\#2893](https://github.com/tendermint/tendermint/issues/2893) Use genDoc.Validators instead of state.NextValidators on replay when appHeight==0 (@james-ray)
- [log] [\#2868](https://github.com/tendermint/tendermint/issues/2868) Fix `module=main` setting overriding all others
- [rpc] [\#2808](https://github.com/tendermint/tendermint/issues/2808) RPC validators calls IncrementAccum if necessary
- [rpc] [\#2811](https://github.com/tendermint/tendermint/issues/2811) Allow integer IDs in JSON-RPC requests (@tomtau)
- [txindex/kv] [\#2759](https://github.com/tendermint/tendermint/issues/2759) Fix tx.height range queries
- [txindex/kv] [\#2775](https://github.com/tendermint/tendermint/issues/2775) Order tx results by index if height is the same
- [txindex/kv] [\#2908](https://github.com/tendermint/tendermint/issues/2908) Don't return false positives when searching for a prefix of a tag value
