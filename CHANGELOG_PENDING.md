## v0.33.3

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- [types] \#4382 `BlockIdFlag` and `SignedMsgType` has moved to a protobuf enum types
- [types] \#4382 `PartSetHeader` has become a protobuf type, `Total` has been changed from a `int` to a `int32`
- [types] \#4382 `BlockID` has become a protobuf type
- [types] \#4382 enum `CheckTxType` values have been made uppercase: `NEW` & `RECHECK`
- [types] \#4582 Vote: `ValidatorIndex` & `Round` are now int32
- [types] \#4582 Proposal: `POLRound` & `Round` are now int32
- [types] \#4582 Block: `Round` is now int32
- [consensus] \#4582 RoundState: `Round`, `LockedRound` & `CommitRound` are now int32
- [consensus] \#4582 HeightVoteSet: `round` is now int32
- [crypto] \#4633 Remove suffixes from all keys.
    - ed25519: type `PrivKeyEd25519` is now `PrivKey`
    - ed25519: type `PubKeyEd25519` is now `PubKey`
    - secp256k1: type`PrivKeySecp256k1` is now `PrivKey`
    - secp256k1: type`PubKeySecp256k1` is now `PubKey`
    - sr25519: type `PrivKeySr25519` is now `PrivKey`
    - sr25519: type `PubKeySr25519` is now `PubKey`
- [evidence] \#4617 Remove `Pubkey` from DuplicateVoteEvidence
- [privval] \#4582 `round` in private_validator_state.json is no longer a string in json.

- Apps

- Go API

  - [rpc/client] [\#4628](https://github.com/tendermint/tendermint/pull/4628) Split out HTTP and local clients into `http` and `local` packages (@erikgrinaker).
  - [lite2] [\#4616](https://github.com/tendermint/tendermint/pull/4616) Make `maxClockDrift` an option (@melekes).
    `Verify/VerifyAdjacent/VerifyNonAdjacent` now accept `maxClockDrift time.Duration`.

### FEATURES:

- [rpc] [\#4611](https://github.com/tendermint/tendermint/pull/4611) Add `codespace` to `ResultBroadcastTx` (@whylee259)

### IMPROVEMENTS:

- [p2p] [\#4548](https://github.com/tendermint/tendermint/pull/4548) Add ban list to address book (@cmwaters)
- [privval] \#4534 Add `error` as a return value on`GetPubKey()`
- [Docker] \#4569 Default configuration added to docker image (you can still mount your own config the same way) (@greg-szabo)
- [lite2] [\#4562](https://github.com/tendermint/tendermint/pull/4562) Cache headers when using bisection (@cmwaters)
- [all] [\#4608](https://github.com/tendermint/tendermint/pull/4608) Give reactors descriptive names when they're initialized
- [lite2] [\#4575](https://github.com/tendermint/tendermint/pull/4575) Use bisection for within-range verification (@cmwaters)
- [tools] \#4615 Allow developers to use Docker to generate proto stubs, via `make proto-gen-docker`.

### BUG FIXES:

- [rpc] \#4568 Fix panic when `Subscribe` is called, but HTTP client is not running (@melekes)
  `Subscribe`, `Unsubscribe(All)` methods return an error now.
