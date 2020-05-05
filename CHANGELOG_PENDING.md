## v0.33.5

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

  - [evidence] \#4725 Remove `Pubkey` from DuplicateVoteEvidence

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
- [state] [\#4781](https://github.com/tendermint/tendermint/pull/4781) Export `InitStateVersion` for the initial state version (@erikgrinaker)
- [p2p/conn] \#4795 Return err on `signChallenge()` instead of panic

### BUG FIXES:

- [blockchain/v2] [\#4761](https://github.com/tendermint/tendermint/pull/4761) Fix excessive CPU usage caused by spinning on closed channels (@erikgrinaker)
- [blockchain/v2] Respect `fast_sync` option (@erikgrinaker)
- [light] [\#4741](https://github.com/tendermint/tendermint/pull/4741) Correctly return  `ErrSignedHeaderNotFound` and `ErrValidatorSetNotFound` on corresponding RPC errors (@erikgrinaker)
- [types] [\#4764](https://github.com/tendermint/tendermint/pull/4764) Return an error if voting power overflows in `VerifyCommitTrusting` (@melekes)
