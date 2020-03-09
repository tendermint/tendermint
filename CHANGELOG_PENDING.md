## v0.33.2

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- [types] \#4382 `BlockIdFlag` and `SignedMsgType` has moved to a protobuf enum types
- [types] \#4382 `PartSetHeader` has become a protobuf type, total has been changed from a `int` to a `int32`
- [types] \#4382  `BlockID` has become a protobuf type

- Apps

- Go API

### FEATURES:

### IMPROVEMENTS:

### BUG FIXES:

- [rpc] [\#4437](https://github.com/tendermint/tendermint/pull/4437) Fix tx_search pagination with ordered results (@erikgrinaker)

- [rpc] [\#4406](https://github.com/tendermint/tendermint/pull/4406) Fix issue with multiple subscriptions on the websocket (@antho1404)
