## v0.33.5

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

  - [evidence] \#4725 Remove `Pubkey` from DuplicateVoteEvidence
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

  - [crypto] [\#4721](https://github.com/tendermint/tendermint/pull/4721) Remove `SimpleHashFromMap()` and `SimpleProofsFromMap()` (@erikgrinaker)
  - [privval] [\#4744](https://github.com/tendermint/tendermint/pull/4744) Remove deprecated `OldFilePV` (@melekes)

- Blockchain Protocol

### FEATURES:

- [evidence] [\#4532](https://github.com/tendermint/tendermint/pull/4532) Handle evidence from light clients (@melekes)
- [lite2] [\#4532](https://github.com/tendermint/tendermint/pull/4532) Submit conflicting headers, if any, to a full node & all witnesses (@melekes)

### IMPROVEMENTS:

- [abci/server] [\#4719](https://github.com/tendermint/tendermint/pull/4719) Print panic & stack trace to STDERR if logger is not set (@melekes)
- [types] [\#4638](https://github.com/tendermint/tendermint/pull/4638) Implement `Header#ValidateBasic` (@alexanderbez)
- [txindex] [\#4466](https://github.com/tendermint/tendermint/pull/4466) Allow to index an event at runtime (@favadi)
- [evidence] [\#4722](https://github.com/tendermint/tendermint/pull/4722) Improved evidence db (@cmwaters)

### BUG FIXES:
