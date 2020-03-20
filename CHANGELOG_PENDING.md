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
- [types] \#4582 Vote: `ValidatorIndex` & `Round` are now int32
- [types] \#4582 Proposal: `POLRound` & `Round` are now int32
- [types] \#4582 Block:`Round` are now int32
- [Consensus] \#4582 RoundState:`Round`, `LockedRound` & `CommitRound` are now int32
- [Consensus] \#4582 HeightVoteSet:`round` are now int32
- [privval] \#4582 `round` in private_validator_state.json is no longer a string in json.
- Apps

- Go API

### FEATURES:

### IMPROVEMENTS:

- [p2p] [\#4548](https://github.com/tendermint/tendermint/pull/4548) Add ban list to address book (@cmwaters)
- [privval] \#4534 Add `error` as a return value on`GetPubKey()`

### BUG FIXES:
