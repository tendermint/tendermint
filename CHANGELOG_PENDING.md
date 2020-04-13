## v0.33.4

- Nodes are no longer guaranteed to contain all blocks up to the latest height. The ABCI app can now control which blocks to retain through the ABCI field `ResponseCommit.retain_height`, all blocks and associated data below this height will be removed.

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- [blockchain] \#4637 Transition blockchain reactor to protobuf encoding
- [blockchain] \#4656 Remove blockchain reactor v1
- [consensus] \#4582 RoundState: `Round`, `LockedRound` & `CommitRound` are now int32
- [consensus] \#4582 HeightVoteSet: `round` is now int32
- [consensus] \#4733 Msgs & WALMsgs are now moved to proto
- [crypto] \#4733 `SimpleProof` Total & Index fields have become int64's
- [crypto] \#4633 Remove suffixes from all keys.
    - ed25519: type `PrivKeyEd25519` is now `PrivKey`
    - ed25519: type `PubKeyEd25519` is now `PubKey`
    - secp256k1: type`PrivKeySecp256k1` is now `PrivKey`
    - secp256k1: type`PubKeySecp256k1` is now `PubKey`
    - sr25519: type `PrivKeySr25519` is now `PrivKey`
    - sr25519: type `PubKeySr25519` is now `PubKey`
    - multisig: type `PubKeyMultisigThreshold` is now `PubKey`
- [evidence] \#4617 Remove `Pubkey` from DuplicateVoteEvidence
- [lite] \#4760 Remove lite package
- [privval] \#4582 `round` in private_validator_state.json is no longer a string in json.
- [script] \#4733 Transition Wal2json & json2Wal scripts to work with protobuf
- [state] \#4679 `TxResult` is a protobuf type defined in `abci` types directory
- [types] \#4382 `BlockIdFlag` and `SignedMsgType` has moved to a protobuf enum types
- [types] \#4382 `PartSetHeader` has become a protobuf type, `Total` has been changed from a `int` to a `int32`
- [types] \#4382 `BlockID` has become a protobuf type
- [types] \#4382 enum `CheckTxType` values have been made uppercase: `NEW` & `RECHECK`
- [types] \#4582 Vote: `ValidatorIndex` & `Round` are now int32
- [types] \#4582 Proposal: `POLRound` & `Round` are now int32
- [types] \#4582 Block: `Round` is now int32
- [types] \#4680 `ConsensusParams`, `BlockParams`, `EvidenceParams`, `ValidatorParams` & `HashedParams` are now protobuf types

- Apps

- P2P Protocol

- Go API

  - [rpc/client] [\#4628](https://github.com/tendermint/tendermint/pull/4628) Split out HTTP and local clients into `http` and `local` packages (@erikgrinaker).
  - [lite2] [\#4616](https://github.com/tendermint/tendermint/pull/4616) Make `maxClockDrift` an option (@melekes).
    `Verify/VerifyAdjacent/VerifyNonAdjacent` now accept `maxClockDrift time.Duration`.

### FEATURES:

- [abci] Add `ResponseCommit.retain_height` field, which will automatically remove blocks below this height.
- [rpc] Add `/status` response fields for the earliest block available on the node
- [rpc] [\#4611](https://github.com/tendermint/tendermint/pull/4611) Add `codespace` to `ResultBroadcastTx` (@whylee259)
- [cmd] [\#4665](https://github.com/tendermint/tendermint/pull/4665) New `tedermint completion` command to generate Bash/Zsh completion scripts (@alessio).

### IMPROVEMENTS:

- [blockchain] Add `Base` to blockchain reactor P2P messages `StatusRequest` and `StatusResponse`
- [example/kvstore] Add `RetainBlocks` option to control block retention
- [p2p] [\#4548](https://github.com/tendermint/tendermint/pull/4548) Add ban list to address book (@cmwaters)
- [privval] \#4534 Add `error` as a return value on`GetPubKey()`
- [Docker] \#4569 Default configuration added to docker image (you can still mount your own config the same way) (@greg-szabo)
- [lite2] [\#4562](https://github.com/tendermint/tendermint/pull/4562) Cache headers when using bisection (@cmwaters)
- [all] [\#4608](https://github.com/tendermint/tendermint/pull/4608) Give reactors descriptive names when they're initialized
- [lite2] [\#4575](https://github.com/tendermint/tendermint/pull/4575) Use bisection for within-range verification (@cmwaters)
- [tools] \#4615 Allow developers to use Docker to generate proto stubs, via `make proto-gen-docker`.
- [p2p] \#4621(https://github.com/tendermint/tendermint/pull/4621) ban peers when messages are unsolicited or too frequent (@cmwaters)
- [evidence] [\#4632](https://github.com/tendermint/tendermint/pull/4632) Inbound evidence checked if already existing (@cmwaters)

### BUG FIXES:

- [rpc] \#4568 Fix panic when `Subscribe` is called, but HTTP client is not running (@melekes)
  `Subscribe`, `Unsubscribe(All)` methods return an error now.
