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

  - [abci] [\#4704](https://github.com/tendermint/tendermint/pull/4704) Add ABCI methods `ListSnapshots`, `LoadSnapshotChunk`, `OfferSnapshot`, and `ApplySnapshotChunk` for state sync snapshots. `ABCIVersion` bumped to 0.17.0.

- P2P Protocol

- Go API

  - [crypto] [\#4721](https://github.com/tendermint/tendermint/pull/4721) Remove `SimpleHashFromMap()` and `SimpleProofsFromMap()` (@erikgrinaker)
  - [privval] [\#4744](https://github.com/tendermint/tendermint/pull/4744) Remove deprecated `OldFilePV` (@melekes)

- Blockchain Protocol

### FEATURES:

- [statesync] Add state sync support, where a new node can be rapidly bootstrapped by fetching state snapshots from peers instead of replaying blocks. See the `[statesync]` config section.
- [evidence] [\#4532](https://github.com/tendermint/tendermint/pull/4532) Handle evidence from light clients (@melekes)
- [lite2] [\#4532](https://github.com/tendermint/tendermint/pull/4532) Submit conflicting headers, if any, to a full node & all witnesses (@melekes)

### IMPROVEMENTS:

- [abci/server] [\#4719](https://github.com/tendermint/tendermint/pull/4719) Print panic & stack trace to STDERR if logger is not set (@melekes)
- [types] [\#4638](https://github.com/tendermint/tendermint/pull/4638) Implement `Header#ValidateBasic` (@alexanderbez)
- [txindex] [\#4466](https://github.com/tendermint/tendermint/pull/4466) Allow to index an event at runtime (@favadi)
- [evidence] [\#4722](https://github.com/tendermint/tendermint/pull/4722) Improved evidence db (@cmwaters)
- [buildsystem] [\#4378](https://github.com/tendermint/tendermint/pull/4738) Replace build_c and install_c with TENDERMINT_BUILD_OPTIONS parsing. The following options are available:
  - nostrip: don't strip debugging symbols nor DWARF tables.
  - cleveldb: use cleveldb as db backend instead of goleveldb.
  - race: pass -race to go build and enable data race detection.

### BUG FIXES:

- [blockchain/v2] [\#4761](https://github.com/tendermint/tendermint/pull/4761) Fix excessive CPU usage caused by spinning on closed channels (@erikgrinaker)
- [blockchain/v2] Respect `fast_sync` option (@erikgrinaker)
- [light] [\#4741](https://github.com/tendermint/tendermint/pull/4741) Correctly return  `ErrSignedHeaderNotFound` and `ErrValidatorSetNotFound` on corresponding RPC errors (@erikgrinaker)
