## v0.33.3

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- [abci] \#4624 The proto type Header's chainid value has changed to `ChainId` 
- [types] \#4382 `BlockIdFlag` and `SignedMsgType` has moved to a protobuf enum types
- [types] \#4382 `PartSetHeader` has become a protobuf type, `Total` has been changed from a `int` to a `int32`
- [types] \#4382 `BlockID` has become a protobuf type
- [types] \#4382 enum `CheckTxType` values have been made uppercase: `NEW` & `RECHECK`
- [types] \#4582 Vote: `ValidatorIndex` & `Round` are now int32
- [types] \#4582 Proposal: `POLRound` & `Round` are now int32
- [types] \#4582 Block: `Round` is now int32
- [consensus] \#4582 RoundState: `Round`, `LockedRound` & `CommitRound` are now int32
- [consensus] \#4582 HeightVoteSet: `round` is now int32
- [privval] \#4582 `round` in private_validator_state.json is no longer a string in json.

- Apps

- Go API

### FEATURES:

### IMPROVEMENTS:

- [p2p] [\#4548](https://github.com/tendermint/tendermint/pull/4548) Add ban list to address book (@cmwaters)
- [privval] \#4534 Add `error` as a return value on`GetPubKey()`
- [Docker] \#4569 Default configuration added to docker image (you can still mount your own config the same way) (@greg-szabo)
- [lite2] [\#4562](https://github.com/tendermint/tendermint/pull/4562) Cache headers when using bisection (@cmwaters)
- [all] [\4608](https://github.com/tendermint/tendermint/pull/4608) Give reactors descriptive names when they're initialized

### BUG FIXES:

- [rpc] \#4568 Fix panic when `Subscribe` is called, but HTTP client is not running (@melekes)
  `Subscribe`, `Unsubscribe(All)` methods return an error now.
