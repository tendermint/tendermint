## v0.33.3

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- [types] \#4382 `BlockIdFlag` and `SignedMsgType` has moved to a protobuf enum types
- [types] \#4382 `PartSetHeader` has become a protobuf type, `Total` has been changed from a `int` to a `int32`
- [types] \#4382  `BlockID` has become a protobuf type
- [types] \#4382  enum `CheckTxType` values have been made uppercase: `NEW` & `RECHECK`

- Apps

- Go API

### FEATURES:

### IMPROVEMENTS:

### BUG FIXES:
